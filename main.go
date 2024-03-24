package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"mikrotik-exporter/collector"
	"mikrotik-exporter/config"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	common_version "github.com/prometheus/common/version"
	"github.com/prometheus/exporter-toolkit/web"
)

// single device can be defined via CLI flags, multiple via config file.
var (
	address     = flag.String("address", "", "address of the device to monitor")
	configFile  = flag.String("config-file", "", "config file to load")
	device      = flag.String("device", "", "single device to monitor")
	insecure    = flag.Bool("insecure", false, "skips verification of server certificate when using TLS (not recommended)")
	logFormat   = flag.String("log-format", "text", "log format text or json (default text)")
	logLevel    = flag.String("log-level", "info", "log level")
	metricsPath = flag.String("path", "/metrics", "path to answer requests on")
	password    = flag.String("password", "", "password for authentication for single device")
	deviceport  = flag.String("deviceport", "8728", "port for single device")
	port        = flag.String("port", ":9436", "port number to listen on")
	timeout     = flag.Uint("timeout", collector.DefaultTimeout, "timeout when connecting to devices")
	tlsEnabled  = flag.Bool("tls", false, "use tls to connect to routers")
	user        = flag.String("user", "", "user for authentication with single device")
	ver         = flag.Bool("version", false, "find the version of binary")
	webConfig   = flag.String("web-config", "", "web config file to load")

	listCollectors = flag.Bool("list-collectors", false, "list available collectors")
)

func init() {
	prometheus.MustRegister(version.NewCollector("mikrotik_exporter"))
}

func main() {
	for _, c := range collector.AvailableCollectors() {
		flag.Bool("with-"+c.Name, false, c.Description)
	}

	flag.Parse()

	if *ver {
		fmt.Printf("\nVersion:   %s\n\n", common_version.Print("mikrotik_exporter"))
		os.Exit(0)
	}

	if *listCollectors {
		fmt.Printf("\nAvailable collectors:\n")

		var collectors []string
		for _, c := range collector.AvailableCollectors() {
			collectors = append(collectors,
				fmt.Sprintf(" - %-12s %s", c.Name, c.Description))
		}

		sort.Strings(collectors)

		for _, c := range collectors {
			fmt.Println(c)
		}

		fmt.Println()
		os.Exit(0)
	}

	logger := config.ConfigureLog(*logLevel, *logFormat)
	cfg := loadConfig(logger)

	startServer(cfg, logger)
}

func loadConfig(logger log.Logger) *config.Config {
	var (
		cfg *config.Config
		err error
	)

	if *configFile != "" {
		cfg, err = loadConfigFromFile()
	} else {
		cfg, err = loadConfigFromFlags()
	}

	if err != nil {
		_ = level.Error(logger).Log("msg", "could not load config", "error", err)

		os.Exit(3)
	}

	updateConfigFromFlags(cfg)

	return cfg
}

func loadConfigFromFile() (*config.Config, error) {
	b, err := os.ReadFile(*configFile)
	if err != nil {
		return nil, fmt.Errorf("read file error: %w", err)
	}

	cfg, err := config.Load(bytes.NewReader(b), collector.AvailableCollectorsNames())
	if err != nil {
		return nil, fmt.Errorf("load error: %w", err)
	}

	return cfg, nil
}

var ErrMissingParam = errors.New("missing required param for single device configuration")

func loadConfigFromFlags() (*config.Config, error) {
	// Attempt to read credentials from env if not already defined
	if *user == "" {
		*user = os.Getenv("MIKROTIK_USER")
	}

	if *password == "" {
		*password = os.Getenv("MIKROTIK_PASSWORD")
	}

	if *device == "" || *address == "" || *user == "" || *password == "" {
		return nil, ErrMissingParam
	}

	return &config.Config{
		Devices: []config.Device{
			{
				Name:     *device,
				Address:  *address,
				User:     *user,
				Password: *password,
				Port:     *deviceport,
				TLS:      *tlsEnabled,
				Insecure: *insecure,
				Timeout:  *timeout,
			},
		},
		Features: make(config.Features),
	}, nil
}

func startServer(cfg *config.Config, logger log.Logger) {
	h, err := createMetricsHandler(cfg, logger)
	if err != nil {
		panic(err)
	}

	http.Handle(*metricsPath, h)

	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	if *metricsPath != "/" {
		landingConfig := web.LandingConfig{
			Name:        "Mikrotik Exporter",
			Description: "Prometheus Mikrotik Exporter",
			Version:     common_version.Info(),
			Links: []web.LandingLinks{
				{
					Address: *metricsPath,
					Text:    "Metrics",
				},
			},
		}

		landingPage, err := web.NewLandingPage(landingConfig)
		if err != nil {
			level.Error(logger).Log("err", err)
			os.Exit(1)
		}

		http.Handle("/", landingPage)
	}

	serverTimeout := time.Duration(2**timeout) * time.Second
	srv := &http.Server{
		ReadTimeout:  serverTimeout,
		WriteTimeout: serverTimeout,
	}
	level.Error(logger).Log(web.ListenAndServe(srv, &web.FlagConfig{
		WebListenAddresses: &[]string{*port},
		WebConfigFile:      webConfig,
	}, logger))
}

func createMetricsHandler(cfg *config.Config, logger log.Logger) (http.Handler, error) {
	collector, err := collector.NewCollector(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("create collector error: %w", err)
	}

	promhttp.Handler()

	registry := prometheus.NewRegistry()

	err = registry.Register(collectors.NewGoCollector())
	if err != nil {
		return nil, fmt.Errorf("register gocollector error: %w", err)
	}

	err = registry.Register(collector)
	if err != nil {
		return nil, fmt.Errorf("register collector error: %w", err)
	}

	return promhttp.HandlerFor(registry,
		promhttp.HandlerOpts{
			ErrorLog:      loggerBridge{logger},
			ErrorHandling: promhttp.ContinueOnError,
		}), nil
}

func updateConfigFromFlags(cfg *config.Config) {
	flag.Visit(func(f *flag.Flag) {
		if strings.HasPrefix(f.Name, "with-") {
			feat := strings.TrimPrefix(f.Name, "with-")
			cfg.Features[feat] = true
		}
	})
}

type loggerBridge struct {
	logger log.Logger
}

func (l loggerBridge) Println(v ...interface{}) {
	_ = level.Info(l.logger).Log(v...)
}
