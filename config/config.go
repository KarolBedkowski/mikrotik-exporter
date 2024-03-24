package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/go-kit/log/term"

	"github.com/hashicorp/go-multierror"
	yaml "gopkg.in/yaml.v2"
)

const (
	Namespace  = "mikrotik"
	APIPort    = "8728"
	APIPortTLS = "8729"
	DNSPort    = 53

	// DefaultTimeout defines the default timeout when connecting to a router.
	DefaultTimeout = 5
)

var (
	ErrUnknownDevice  = errors.New("unknown device")
	ErrUnknownProfile = errors.New("unknown profile")
)

var GlobalLogger log.Logger

type UnknownFeatureError string

func (e UnknownFeatureError) Error() string {
	return "unknown feature: " + string(e)
}

type Features map[string]bool

func (f Features) validate(collectors []string) error {
	// for tests
	if len(collectors) == 0 {
		return nil
	}

	var result *multierror.Error

	for key := range f {
		if !slices.Contains(collectors, strings.ToLower(key)) {
			result = multierror.Append(result, UnknownFeatureError(key))
		}
	}

	return result.ErrorOrNil()
}

func (f Features) FeatureNames() []string {
	res := make([]string, 0, len(f))

	for name, enabled := range f {
		if enabled {
			res = append(res, strings.ToLower(name))
		}
	}

	return res
}

// Config represents the configuration for the exporter.
type Config struct {
	Devices  []Device            `yaml:"devices"`
	Features Features            `yaml:"features,omitempty"`
	Profiles map[string]Features `yaml:"profiles,omitempty"`
}

// Device represents a target device.
type Device struct {
	Name     string     `yaml:"name"`
	Address  string     `yaml:"address,omitempty"`
	Srv      *SrvRecord `yaml:"srv,omitempty"`
	User     string     `yaml:"user"`
	Password string     `yaml:"password"`
	Port     string     `yaml:"port"`
	Profile  string     `yaml:"profile,omitempty"`

	IPv6Disabled bool `yaml:"ipv6_disabled"`

	Timeout  uint `yaml:"timeout,omitempty"`
	TLS      bool `yaml:"tls,omitempty"`
	Insecure bool `yaml:"insecure,omitempty"`
}

type SrvRecord struct {
	Record string `yaml:"record"`
	/// DNS is additional dns server used to resolved `Record`
	DNS *DNSServer `yaml:"dns,omitempty"`
}

type DNSServer struct {
	Address string `yaml:"address"`
	Port    int    `yaml:"port"`
}

// Load reads YAML from reader and unmashals in Config.
func Load(r io.Reader, collectors []string) (*Config, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read error: %w", err)
	}

	cfg := &Config{}

	if err := yaml.Unmarshal(b, cfg); err != nil {
		return nil, fmt.Errorf("unmarshal error: %w", err)
	}

	if err := cfg.Features.validate(collectors); err != nil {
		return nil, fmt.Errorf("validate features error: %w", err)
	}

	// always enabled
	cfg.Features["resource"] = true

	for name, features := range cfg.Profiles {
		if err := features.validate(collectors); err != nil {
			return nil, fmt.Errorf("invalid profile '%s': %w", name, err)
		}

		// always enabled
		features["resource"] = true
	}

	return cfg, nil
}

func (c *Config) DeviceFeatures(deviceName string) (*Features, error) {
	for _, d := range c.Devices {
		if d.Name == deviceName {
			if d.Profile == "" {
				return &c.Features, nil
			}

			if f, ok := c.Profiles[d.Profile]; ok {
				return &f, nil
			}

			return nil, ErrUnknownProfile
		}
	}

	return nil, ErrUnknownDevice
}

func (c *Config) FindDevice(deviceName string) (*Device, error) {
	for _, d := range c.Devices {
		if d.Name == deviceName {
			return &d, nil
		}
	}

	return nil, ErrUnknownDevice
}

func ConfigureLog(logLevel, logFormat string) log.Logger {
	var logger log.Logger

	w := log.NewSyncWriter(os.Stdout)

	if logFormat == "json" {
		logger = term.NewLogger(w, log.NewJSONLogger, logColorFunc)
	} else {
		logger = term.NewLogger(w, log.NewLogfmtLogger, logColorFunc)
	}

	logger = level.NewFilter(logger, level.Allow(level.ParseDefault(logLevel, level.DebugValue())))
	logger = log.WithSuffix(logger, "caller", log.DefaultCaller)

	nlogger := log.LoggerFunc(func(keyvals ...interface{}) error {
		if err := logger.Log(keyvals...); err != nil {
			panic(fmt.Errorf("%v: %w", keyvals, err))
		}

		return nil
	})

	GlobalLogger = nlogger

	return nlogger
}

func logColorFunc(keyvals ...interface{}) term.FgBgColor {
	for i := 0; i < len(keyvals)-1; i += 2 {
		if keyvals[i] != "level" {
			continue
		}

		level, ok := keyvals[i+1].(level.Value)
		if !ok {
			continue
		}

		switch level.String() {
		case "debug":
			return term.FgBgColor{Fg: term.DarkGray}
		case "info":
			return term.FgBgColor{Fg: term.Gray}
		case "warn":
			return term.FgBgColor{Fg: term.Yellow}
		case "error":
			return term.FgBgColor{Fg: term.Red}
		default:
			return term.FgBgColor{}
		}
	}

	return term.FgBgColor{}
}
