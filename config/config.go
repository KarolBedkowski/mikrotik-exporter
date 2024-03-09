package config

import (
	"fmt"
	"io"
	"io/ioutil"

	yaml "gopkg.in/yaml.v2"
)

type Features struct {
	BGP       bool `yaml:"bgp,omitempty"`
	Conntrack bool `yaml:"conntrack,omitempty"`
	DHCP      bool `yaml:"dhcp,omitempty"`
	DHCPL     bool `yaml:"dhcpl,omitempty"`
	DHCPv6    bool `yaml:"dhcpv6,omitempty"`
	Firmware  bool `yaml:"firmware,omitempty"`
	Health    bool `yaml:"health,omitempty"`
	Routes    bool `yaml:"routes,omitempty"`
	POE       bool `yaml:"poe,omitempty"`
	Pools     bool `yaml:"pools,omitempty"`
	Optics    bool `yaml:"optics,omitempty"`
	W60G      bool `yaml:"w60g,omitempty"`
	WlanSTA   bool `yaml:"wlansta,omitempty"`
	Capsman   bool `yaml:"capsman,omitempty"`
	WlanIF    bool `yaml:"wlanif,omitempty"`
	Monitor   bool `yaml:"monitor,omitempty"`
	Ipsec     bool `yaml:"ipsec,omitempty"`
	Lte       bool `yaml:"lte,omitempty"`
	Netwatch  bool `yaml:"netwatch,omitempty"`
	Queue     bool `yaml:"queue,omitempty"`
}

func (f Features) FeatureNames() []string {
	var res []string
	if f.BGP {
		res = append(res, "BGP")
	}

	if f.Routes {
		res = append(res, "Routes")
	}

	if f.DHCP {
		res = append(res, "DHCPL")
	}

	if f.DHCPL {
		res = append(res, "DHCP")
	}

	if f.DHCPv6 {
		res = append(res, "DHCPv6")
	}

	if f.Firmware {
		res = append(res, "Firmware")
	}

	if f.Health {
		res = append(res, "Health")
	}

	if f.POE {
		res = append(res, "POE")
	}

	if f.Pools {
		res = append(res, "Pool")
	}

	if f.Optics {
		res = append(res, "Optics")
	}

	if f.W60G {
		res = append(res, "W60gInterface")
	}

	if f.WlanSTA {
		res = append(res, "WlanSTA")
	}

	if f.Capsman {
		res = append(res, "Capsman")
	}

	if f.WlanIF {
		res = append(res, "WlanIF")
	}

	if f.Monitor {
		res = append(res, "Monitor")
	}

	if f.Ipsec {
		res = append(res, "Ipsec")
	}

	if f.Conntrack {
		res = append(res, "Conntrack")
	}

	if f.Lte {
		res = append(res, "Lte")
	}

	if f.Netwatch {
		res = append(res, "Netwatch")
	}

	if f.Queue {
		res = append(res, "Queue")
	}

	return res
}

// Config represents the configuration for the exporter
type Config struct {
	Devices  []Device            `yaml:"devices"`
	Features Features            `yaml:"features,omitempty"`
	Profiles map[string]Features `yaml:"profiles,omitempty"`
}

// Device represents a target device
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

// Load reads YAML from reader and unmashals in Config
func Load(r io.Reader) (*Config, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	c := &Config{}
	err = yaml.Unmarshal(b, c)
	if err != nil {
		return nil, err
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
