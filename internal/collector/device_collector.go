package collector

//
// device_collector.go
//
// Distributed under terms of the GPLv3 license.
//

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	"mikrotik-exporter/internal/collectors"
	"mikrotik-exporter/internal/config"
	"mikrotik-exporter/internal/metrics"
	"mikrotik-exporter/routeros"

	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

var scrapeCollectorErrorsDesc = prometheus.NewDesc(
	prometheus.BuildFQName(config.Namespace, "scrape", "device_errors_total"),
	"mikrotik_exporter: number of failed collection per device",
	[]string{"dev_name", "dev_address"},
	nil,
)

type (
	deviceCollectorRC struct {
		collector   collectors.RouterOSCollector
		name        string
		featureConf config.FeatureConf
	}

	deviceCollector struct {
		logger     *slog.Logger
		cl         *routeros.Client
		device     config.Device
		collectors []deviceCollectorRC
		isSrv      bool
		errors     int64
	}
)

func newDeviceCollector(device config.Device, collectors []deviceCollectorRC) *deviceCollector {
	if device.TLS {
		if (device.Port) == "" {
			device.Port = config.APIPortTLS
		}
	} else {
		if (device.Port) == "" {
			device.Port = config.APIPort
		}
	}

	if device.Timeout == 0 {
		device.Timeout = config.DefaultTimeout
	}

	return &deviceCollector{
		device:     device,
		collectors: collectors,
		isSrv:      device.Srv != nil,
		logger:     slog.Default().With("device", device.Name),
	}
}

func (dc *deviceCollector) disconnect() {
	// close connection for srv-defined targets
	if dc.isSrv {
		if dc.cl != nil {
			dc.cl.Close()
			dc.cl = nil
		}
	}
}

func (dc *deviceCollector) connect() (*routeros.Client, error) {
	// try do get connection from cache
	if dc.cl != nil {
		// check is connection alive
		if reply, err := dc.cl.Run("/system/identity/print"); err == nil && len(reply.Re) > 0 {
			return dc.cl, nil
		}

		dc.logger.Info("reconnecting")

		// check failed, reconnect
		dc.cl.Close()
		dc.cl = nil
	}

	dc.logger.Debug("trying to Dial")

	conn, err := dc.dial()
	if err != nil {
		return nil, err
	}

	dc.logger.Debug("done dialing")

	client, err := routeros.NewClient(conn)
	if err != nil {
		return nil, fmt.Errorf("create client error: %w", err)
	}

	dc.logger.Debug("got client, trying to login")

	if err := client.Login(dc.device.User, dc.device.Password); err != nil {
		client.Close()

		return nil, fmt.Errorf("login error: %w", err)
	}

	dc.logger.Debug("done with login")
	dc.cl = client

	return client, nil
}

func (dc *deviceCollector) dial() (net.Conn, error) {
	var (
		con     net.Conn
		err     error
		timeout = time.Duration(dc.device.Timeout) * time.Second
	)

	if !dc.device.TLS {
		con, err = net.DialTimeout("tcp", dc.device.Address+":"+dc.device.Port, timeout)
	} else {
		con, err = tls.DialWithDialer(
			&net.Dialer{
				Timeout: timeout,
			},
			"tcp",
			dc.device.Address+":"+dc.device.Port,
			&tls.Config{
				InsecureSkipVerify: dc.device.Insecure, // #nosec
			},
		)
	}

	if err != nil {
		return nil, fmt.Errorf("dial error: %w", err)
	}

	return con, nil
}

// collect data for device and return number of failed collectors and
// error if any.
func (dc *deviceCollector) collect(ch chan<- prometheus.Metric) error {
	client, err := dc.connect()
	if err != nil {
		// clear FirmwareVersion and reload on next successful connection.
		dc.device.FirmwareVersion.Major = 0

		return fmt.Errorf("connect error: %w", err)
	}

	defer dc.disconnect()

	if dc.device.Srv != nil {
		// get identity for service-defined devices
		if err := dc.updateIdentity(client); err != nil {
			return fmt.Errorf("get identity error: %w", err)
		}
	}

	address, name := dc.device.Address, dc.device.Name

	var result *multierror.Error
	// get once version
	if dc.device.FirmwareVersion.Major == 0 {
		if err := dc.getVersion(client); err != nil {
			dc.logger.Warn("get version error", "err", err)
		}
	}

	for _, drc := range dc.collectors {
		logger := dc.logger.With("collector", drc.name)
		ctx := metrics.NewCollectorContext(ch, &dc.device, client, drc.name, logger, drc.featureConf)

		logger.Debug("start collect", "feature_conf", drc.featureConf)

		if err := drc.collector.Collect(&ctx); err != nil {
			result = multierror.Append(result, fmt.Errorf("collect %s error: %w", drc.name, err))

			dc.errors++
		}
	}

	if err := result.ErrorOrNil(); err != nil {
		return fmt.Errorf("collect error: %w", err)
	}

	ch <- prometheus.MustNewConstMetric(scrapeCollectorErrorsDesc, prometheus.CounterValue,
		float64(dc.errors), name, address)

	return nil
}

func (dc *deviceCollector) updateIdentity(client *routeros.Client) error {
	reply, err := client.Run("/system/identity/print")
	if err != nil {
		return fmt.Errorf("get identity error: %w", err)
	}

	if len(reply.Re) == 0 {
		return ErrInvalidResponse
	}

	dc.device.Name = reply.Re[0].Map["name"]
	if dc.device.Name == "" {
		return ErrInvalidResponse
	}

	return nil
}

func (dc *deviceCollector) getVersion(client *routeros.Client) error {
	reply, err := client.Run("/system/resource/print")
	if err != nil {
		return fmt.Errorf("get version error: %w", err)
	}

	version := reply.Re[0].Map["version"]

	dc.device.FirmwareVersion, err = parseFirmwareVersion(version)
	if err != nil {
		return fmt.Errorf("parse version %q error: %w", version, err)
	}

	return nil
}

var ErrInvalidVersion = errors.New("invalid version")

func parseFirmwareVersion(version string) (config.FirmwareVersion, error) {
	version, _, _ = strings.Cut(version, " ")

	parts := strings.Split(version, ".")
	if len(parts) != 3 { //nolint:mnd
		return config.FirmwareVersion{}, ErrInvalidVersion
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return config.FirmwareVersion{}, fmt.Errorf("parse error: %w", err)
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return config.FirmwareVersion{}, fmt.Errorf("parse error: %w", err)
	}

	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return config.FirmwareVersion{}, fmt.Errorf("parse error: %w", err)
	}

	return config.FirmwareVersion{Major: major, Minor: minor, Patch: patch}, nil
}
