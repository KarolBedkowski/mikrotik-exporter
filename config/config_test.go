package config

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestShouldParse(t *testing.T) {
	b := loadTestFile(t)

	c, err := Load(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("could not parse: %v", err)
	}

	if len(c.Devices) != 6 {
		t.Fatalf("expected 6 devices, got %v", len(c.Devices))
	}

	assertDevice("test1", "192.168.1.1", "foo", "bar", &c.Devices[0], t)
	assertDevice("test2", "192.168.2.1", "test", "123", &c.Devices[1], t)
	assertFeature("BGP", c.Features, t)
	assertFeature("Conntrack", c.Features, t)
	assertFeature("DHCP", c.Features, t)
	assertFeature("DHCPv6", c.Features, t)
	assertFeature("Pools", c.Features, t)
	assertFeature("Routes", c.Features, t)
	assertFeature("Optics", c.Features, t)
	assertFeature("WlanSTA", c.Features, t)
	assertFeature("WlanIF", c.Features, t)
	assertFeature("Ipsec", c.Features, t)
	assertFeature("Lte", c.Features, t)
	assertFeature("Netwatch", c.Features, t)
	assertFeature("Queue", c.Features, t)

	f, _ := c.DeviceFeatures("testProfileMinimal")
	assertFeature("Firmware", f, t)
	assertFeature("Health", f, t)
	assertFeature("Monitor", f, t)
	assertNotFeature("BGP", f, t)

	if dev, err := c.FindDevice("testDns"); err != nil {
		t.Fatalf("could not find device: %v", err)
	} else {
		if dev.Srv.Record != "record2" {
			t.Fatalf("expected `record2` service, got %#v", dev.Srv.Record)
		}
		if dev.Srv.DNS.Address != "dnsaddress" {
			t.Fatalf("expected `dnsaddress` dns address, got %#v", dev.Srv.DNS)
		}
		if dev.Srv.DNS.Port != 1053 {
			t.Fatalf("expected `1053` dns port, got %#v", dev.Srv.DNS)
		}
	}
}

func loadTestFile(t *testing.T) []byte {
	b, err := os.ReadFile("config.test.yml")
	if err != nil {
		t.Fatalf("could not load config: %v", err)
	}

	return b
}

func assertDevice(name, address, user, password string, c *Device, t *testing.T) {
	if c.Name != name {
		t.Fatalf("expected name %s, got %s", name, c.Name)
	}

	if c.Address != address {
		t.Fatalf("expected address %s, got %s", address, c.Address)
	}

	if c.User != user {
		t.Fatalf("expected user %s, got %s", user, c.User)
	}

	if c.Password != password {
		t.Fatalf("expected password %s, got %s", password, c.Password)
	}
}

func assertFeature(name string, f Features, t *testing.T) {
	name = strings.ToLower(name)
	if v, ok := f[name]; !ok || !v {
		t.Fatalf("expected feature %s to be enabled", name)
	}
}

func assertNotFeature(name string, f Features, t *testing.T) {
	name = strings.ToLower(name)
	if v, ok := f[name]; ok && v {
		t.Fatalf("expected feature %s to be disabled", name)
	}
}
