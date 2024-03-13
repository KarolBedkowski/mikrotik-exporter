package collector

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/KarolBedkowski/routeros-go-client/proto"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

func init() {
	registerCollector("w60g", neww60gInterfaceCollector)
}

type w60gInterfaceCollector struct {
	frequencyDesc         *prometheus.Desc
	txMCSDesc             *prometheus.Desc
	txPHYRateDesc         *prometheus.Desc
	signalDesc            *prometheus.Desc
	rssiDesc              *prometheus.Desc
	txSectorDesc          *prometheus.Desc
	txDistanceDesc        *prometheus.Desc
	txPacketErrorRateDesc *prometheus.Desc
	props                 []string
	propslist             string
}

func (c *w60gInterfaceCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- c.frequencyDesc
	ch <- c.txMCSDesc
	ch <- c.txPHYRateDesc
	ch <- c.signalDesc
	ch <- c.rssiDesc
	ch <- c.txSectorDesc
	ch <- c.txDistanceDesc
	ch <- c.txPacketErrorRateDesc
}

func (c *w60gInterfaceCollector) collect(ctx *collectorContext) error {
	reply, err := ctx.client.Run("/interface/w60g/print", "=.proplist=name")
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching w60g interface metrics")

		return fmt.Errorf("read w60g error: %w", err)
	}

	ifaces := make([]string, 0)

	for _, iface := range reply.Re {
		n := iface.Map["name"]
		ifaces = append(ifaces, n)
	}

	if len(ifaces) == 0 {
		return nil
	}

	return c.collectw60gMetricsForInterfaces(ifaces, ctx)
}

func (c *w60gInterfaceCollector) collectw60gMetricsForInterfaces(ifaces []string, ctx *collectorContext) error {
	reply, err := ctx.client.Run("/interface/w60g/monitor",
		"=numbers="+strings.Join(ifaces, ","),
		"=once=",
		"=.proplist=name,"+c.propslist)
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching w60g interface monitor metrics")

		return fmt.Errorf("read w60g monitor error: %w", err)
	}

	for _, se := range reply.Re {
		if name, ok := se.Map["name"]; ok {
			c.collectMetricsForw60gInterface(name, se, ctx)
		}
	}

	return nil
}

func (c *w60gInterfaceCollector) collectMetricsForw60gInterface(
	name string, se *proto.Sentence, ctx *collectorContext,
) {
	for _, prop := range c.props {
		v, ok := se.Map[prop]
		if !ok || v == "" {
			continue
		}

		value, err := strconv.ParseFloat(v, 64)
		if err != nil {
			log.WithFields(log.Fields{
				"device":    ctx.device.Name,
				"interface": name,
				"property":  prop,
				"error":     err,
			}).Error("error parsing w60g interface monitor metric")

			return
		}

		ctx.ch <- prometheus.MustNewConstMetric(c.descForKey(prop), prometheus.GaugeValue,
			value, ctx.device.Name, ctx.device.Address, name)
	}
}

func neww60gInterfaceCollector() routerOSCollector {
	const prefix = "w60ginterface"

	props := []string{
		"signal", "rssi", "tx-mcs", "frequency", "tx-phy-rate", "tx-sector",
		"distance", "tx-packet-error-rate",
	}
	labelNames := []string{"name", "address", "interface"}

	return &w60gInterfaceCollector{
		frequencyDesc:         description(prefix, "frequency", "frequency of tx in MHz", labelNames),
		txMCSDesc:             description(prefix, "txMCS", "TX MCS", labelNames),
		txPHYRateDesc:         description(prefix, "txPHYRate", "PHY Rate in bps", labelNames),
		signalDesc:            description(prefix, "signal", "Signal quality in %", labelNames),
		rssiDesc:              description(prefix, "rssi", "Signal RSSI in dB", labelNames),
		txSectorDesc:          description(prefix, "txSector", "TX Sector", labelNames),
		txDistanceDesc:        description(prefix, "txDistance", "Distance to remote", labelNames),
		txPacketErrorRateDesc: description(prefix, "txPacketErrorRate", "TX Packet Error Rate", labelNames),
		props:                 props,
		propslist:             strings.Join(props, ","),
	}
}

func (c *w60gInterfaceCollector) descForKey(name string) *prometheus.Desc {
	switch name {
	case "signal":
		return c.signalDesc
	case "rssi":
		return c.rssiDesc
	case "tx-mcs":
		return c.txMCSDesc
	case "tx-phy-rate":
		return c.txPHYRateDesc
	case "frequency":
		return c.frequencyDesc
	case "tx-sector":
		return c.txSectorDesc
	case "distance":
		return c.txDistanceDesc
	case "tx-packet-error-rate":
		return c.txPacketErrorRateDesc
	}

	return nil
}
