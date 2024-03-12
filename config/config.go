package config

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/hashicorp/go-multierror"
	yaml "gopkg.in/yaml.v2"
)

func isValidFeature(name string) bool {
	validNames := []string{
		"bgp",
		"conntrack",
		"dhcp",
		"dhcpl",
		"dhcpv6",
		"firmware",
		"health",
		"routes",
		"poe",
		"pools",
		"optics",
		"w60g",
		"wlansta",
		"capsman",
		"wlanif",
		"monitor",
		"ipsec",
		"lte",
		"netwatch",
		"queue",
		"resource",
		"interface",
	}

	return slices.Contains(validNames, strings.ToLower(name))
}

type Features map[string]bool

func (f Features) validate() error {
	var result *multierror.Error

	for key := range f {
		if !isValidFeature(key) {
			result = multierror.Append(result, fmt.Errorf("invalid feature '%s'", key))
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
	Name     string    `yaml:"name"`
	Address  string    `yaml:"address,omitempty"`
	Srv      SrvRecord `yaml:"srv,omitempty"`
	User     string    `yaml:"user"`
	Password string    `yaml:"password"`
	Port     string    `yaml:"port"`
	Profile  string    `yaml:"profile,omitempty"`
}

type SrvRecord struct {
	Record string    `yaml:"record"`
	Dns    DnsServer `yaml:"dns,omitempty"`
}
type DnsServer struct {
	Address string `yaml:"address"`
	Port    int    `yaml:"port"`
}

// Load reads YAML from reader and unmashals in Config.
func Load(r io.Reader) (*Config, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	c := &Config{}

	if err := yaml.Unmarshal(b, c); err != nil {
		return nil, err
	}

	if err := c.Features.validate(); err != nil {
		return nil, err
	}

	for name, features := range c.Profiles {
		if err := features.validate(); err != nil {
			return nil, fmt.Errorf("invalid profile %s: %s", name, err)
		}

		// always enabled
		features["interface"] = true
		features["resource"] = true
	}

	return c, nil
}

func (c *Config) DeviceFeatures(deviceName string) (Features, error) {
	for _, d := range c.Devices {
		if d.Name == deviceName {
			if len(d.Profile) == 0 {
				return c.Features, nil
			}

			if f, ok := c.Profiles[d.Profile]; ok {
				return f, nil
			}

			return c.Features, fmt.Errorf("unknown profile %s for device %s",
				d.Profile, d.Name)
		}
	}

	return c.Features, fmt.Errorf("unknown device %s", deviceName)
}
