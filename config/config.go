package config

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"

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

	WaitForFinishCollectingTime = 5
)

var ErrUnknownDevice = errors.New("unknown device")

type MissingFieldError string

func (m MissingFieldError) Error() string {
	return "missing `" + string(m) + "`"
}

type UnknownFeatureError string

func (e UnknownFeatureError) Error() string {
	return "unknown feature: " + string(e)
}

type UnknownProfileError string

func (e UnknownProfileError) Error() string {
	return "unknown profile: " + string(e)
}

type InvalidFieldValueError struct {
	field string
	value string
}

func (i InvalidFieldValueError) Error() string {
	return "invalid value of `" + i.field + "`: `" + i.value + "`"
}

// --------------------------------------

type Features map[string]bool

func (f Features) FeatureNames() []string {
	res := make([]string, 0, len(f))

	for name, enabled := range f {
		if enabled {
			res = append(res, strings.ToLower(name))
		}
	}

	return res
}

func (f Features) validate(collectors []string) error {
	// skip validation when there is no collectors (test, not real life)
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

// --------------------------------------

// Config represents the configuration for the exporter.
type Config struct {
	Features Features            `yaml:"features,omitempty"`
	Profiles map[string]Features `yaml:"profiles,omitempty"`
	Devices  []Device            `yaml:"devices"`
}

func (c *Config) DeviceFeatures(deviceName string) Features {
	for _, d := range c.Devices {
		if d.Name == deviceName {
			if d.Profile == "" {
				return c.Features
			}

			if f, ok := c.Profiles[d.Profile]; ok {
				return f
			}

			panic("unknown profile " + d.Profile + " in device " + deviceName)
		}
	}

	panic("unknown device " + deviceName)
}

func (c *Config) FindDevice(deviceName string) *Device {
	for _, d := range c.Devices {
		if d.Name == deviceName {
			return &d
		}
	}

	panic("unknown device " + deviceName)
}

func (c *Config) AllEnabledFeatures() []string {
	uniqueNames := make(map[string]struct{})

	for _, dev := range c.Devices {
		features := c.Features
		if dev.Profile != "" {
			features = c.DeviceFeatures(dev.Name)
		}

		for _, name := range features.FeatureNames() {
			uniqueNames[name] = struct{}{}
		}
	}

	names := make([]string, 0, len(uniqueNames))
	for k := range uniqueNames {
		names = append(names, k)
	}

	return names
}

func (c *Config) validate(collectors []string) error {
	for name, features := range c.Profiles {
		if err := features.validate(collectors); err != nil {
			return fmt.Errorf("invalid profile '%s': %w", name, err)
		}

		// always enabled
		features["resource"] = true
	}

	var errs *multierror.Error

	for idx, d := range c.Devices {
		if err := d.validate(c.Profiles); err != nil {
			errs = multierror.Append(errs,
				fmt.Errorf("invalid device %d (%s) configuration: %w",
					idx, d.Name, err))
		}
	}

	if err := errs.ErrorOrNil(); err != nil {
		return err
	}

	return nil
}

// --------------------------------------

type SrvRecord struct {
	DNS    *DNSServer `yaml:"dns,omitempty"`
	Record string     `yaml:"record"`
}

type DNSServer struct {
	Address string `yaml:"address"`
	Port    int    `yaml:"port"`
}

// Device represents a target device.
type Device struct {
	Srv                 *SrvRecord          `yaml:"srv,omitempty"`
	FWCollectorSettings map[string][]string `yaml:"fw_collector_settings"`
	Profile             string              `yaml:"profile,omitempty"`
	User                string              `yaml:"user"`
	Password            string              `yaml:"password"`
	Port                string              `yaml:"port"`
	Name                string              `yaml:"name"`
	Address             string              `yaml:"address,omitempty"`
	Scripts             []string            `yaml:"scripts"`
	Timeout             int                 `yaml:"timeout,omitempty"`
	IPv6Disabled        bool                `yaml:"ipv6_disabled"`
	TLS                 bool                `yaml:"tls,omitempty"`
	Insecure            bool                `yaml:"insecure,omitempty"`
	Disabled            bool                `yaml:"disabled,omitempty"`
}

func (d *Device) validate(profiles map[string]Features) error {
	return multierror.Append(nil,
		d.validateConnConf(),
		d.validateFwSettings(),
		d.validateProfile(profiles)).
		ErrorOrNil()
}

func (d *Device) validateConnConf() error {
	var errs *multierror.Error

	if d.Srv == nil {
		if d.Name == "" {
			errs = multierror.Append(errs, MissingFieldError("name"))
		}

		if d.Address == "" {
			errs = multierror.Append(errs, MissingFieldError("address"))
		}
	} else if d.Srv.Record == "" {
		errs = multierror.Append(errs, MissingFieldError("srv.record"))
	}

	if d.User == "" {
		errs = multierror.Append(errs, MissingFieldError("user"))
	}

	if d.Password == "" {
		errs = multierror.Append(errs, MissingFieldError("password"))
	}

	return errs.ErrorOrNil()
}

func (d *Device) validateFwSettings() error {
	var errs *multierror.Error

	validChains := []string{"filter", "mangle", "raw", "nat"}

	for f := range d.FWCollectorSettings {
		if !slices.Contains(validChains, f) {
			errs = multierror.Append(errs, InvalidFieldValueError{"firewall name", f})
		}
	}

	return errs.ErrorOrNil()
}

func (d *Device) validateProfile(profiles map[string]Features) error {
	if d.Profile != "" {
		if _, ok := profiles[d.Profile]; !ok {
			return UnknownProfileError(d.Profile)
		}
	}

	return nil
}

// --------------------------------------

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

	if cfg.Features == nil {
		cfg.Features = make(Features)
	}

	// always enabled
	cfg.Features["resource"] = true

	// remove disabled devices
	cfg.Devices = filterDevices(cfg.Devices)

	if err := cfg.validate(collectors); err != nil {
		return nil, err
	}

	return cfg, nil
}

// --------------------------------------

func filterDevices(devices []Device) []Device {
	// remove disabled devices
	enabled := make([]Device, 0, len(devices))

	for _, d := range devices {
		if !d.Disabled {
			enabled = append(enabled, d)
		}
	}

	return enabled
}
