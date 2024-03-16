package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"mikrotik-exporter/collector"
	"mikrotik-exporter/config"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/collectors/version"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
)

// single device can be defined via CLI flags, multiple via config file.
var (
	address     = flag.String("address", "", "address of the device to monitor")
	configFile  = flag.String("config-file", "", "config file to load")
	device      = flag.String("device", "", "single device to monitor")
	insecure    = flag.Bool("insecure", false, "skips verification of server certificate when using TLS (not recommended)")
	logFormat   = flag.String("log-format", "json", "logformat text or json (default json)")
	logLevel    = flag.String("log-level", "info", "log level")
	metricsPath = flag.String("path", "/metrics", "path to answer requests on")
	password    = flag.String("password", "", "password for authentication for single device")
	deviceport  = flag.String("deviceport", "8728", "port for single device")
	port        = flag.String("port", ":9436", "port number to listen on")
	timeout     = flag.Uint("timeout", collector.DefaultTimeout, "timeout when connecting to devices")
	tls         = flag.Bool("tls", false, "use tls to connect to routers")
	user        = flag.String("user", "", "user for authentication with single device")
	ver         = flag.Bool("version", false, "find the version of binary")

	appVersion = "DEVELOPMENT"
	shortSha   = "0xDEADBEEF"
)

func init() {
	prometheus.MustRegister(version.NewCollector("mikrotik_exporter"))
}

func main() {
	flag.Bool("with-bgp", false, "retrieves BGP routing infrormation")
	flag.Bool("with-capsman", false, "retrieves capsman station metrics")
	flag.Bool("with-cloud", false, "retrieves cloud routing infrormation")
	flag.Bool("with-conntrack", false, "retrieves connection tracking metrics")
	flag.Bool("with-dhcp", false, "retrieves DHCP server metrics")
	flag.Bool("with-dhcpl", false, "retrieves DHCP server lease metrics")
	flag.Bool("with-dhcpv6", false, "retrieves DHCPv6 server metrics")
	flag.Bool("with-firmware", false, "retrieves firmware versions")
	flag.Bool("with-health", false, "retrieves board Health metrics")
	flag.Bool("with-ipsec", false, "retrieves ipsec metrics")
	flag.Bool("with-lte", false, "retrieves lte metrics")
	flag.Bool("with-monitor", false, "retrieves ethernet interface monitor info")
	flag.Bool("with-netwatch", false, "retrieves netwatch metrics")
	flag.Bool("with-optics", false, "retrieves optical diagnostic metrics")
	flag.Bool("with-poe", false, "retrieves PoE metrics")
	flag.Bool("with-pools", false, "retrieves IP(v6) pool metrics")
	flag.Bool("with-queue", false, "retrieves queue metrics")
	flag.Bool("with-routes", false, "retrieves routing table information")
	flag.Bool("with-w60g", false, "retrieves w60g interface metrics")
	flag.Bool("with-wlanif", false, "retrieves wlan interface metrics")
	flag.Bool("with-wlansta", false, "retrieves connected wlan station metrics")

	flag.Parse()

	if *ver {
		fmt.Printf("\nVersion:   %s\nShort SHA: %s\n\n", appVersion, shortSha)
		os.Exit(0)
	}

	configureLog()

	cfg := loadConfig()

	startServer(cfg)
}

func configureLog() {
	ll, err := log.ParseLevel(*logLevel)
	if err != nil {
		panic(err)
	}

	log.SetLevel(ll)

	if *logFormat == "text" {
		log.SetFormatter(&log.TextFormatter{})
	} else {
		log.SetFormatter(&log.JSONFormatter{})
	}
}

func loadConfig() *config.Config {
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
		log.Errorf("Could not load config: %v", err)
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

	cfg, err := config.Load(bytes.NewReader(b), collector.AvailableCollectors())
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
				TLS:      *tls,
				Insecure: true,
				Timeout:  *timeout,
			},
		},
		Features: make(config.Features),
	}, nil
}

func startServer(cfg *config.Config) {
	h, err := createMetricsHandler(cfg)
	if err != nil {
		log.Fatal(err)
	}

	http.Handle(*metricsPath, h)

	http.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("ok"))
	})

	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<html>
			<head><title>Mikrotik Exporter</title></head>
			<body>
			<h1>Mikrotik Exporter</h1>
			<p><a href="` + *metricsPath + `">Metrics</a></p>
			</body>
			</html>`))
	})

	log.Info("Listening on ", *port)

	serverTimeout := time.Duration(2 * *timeout)
	srv := &http.Server{
		Addr:         *port,
		ReadTimeout:  serverTimeout,
		WriteTimeout: serverTimeout,
	}
	log.Fatal(srv.ListenAndServe())
}

func createMetricsHandler(cfg *config.Config) (http.Handler, error) {
	collector, err := collector.NewCollector(cfg)
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
			ErrorLog:      log.New(),
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
