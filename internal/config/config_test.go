package config

import (
	"bytes"
	"errors"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"
)

func TestShouldParse(t *testing.T) {
	b := loadTestFile(t)

	c, err := Load(bytes.NewReader(b), nil)
	if err != nil {
		t.Fatalf("could not parse: %v", err)
	}

	if len(c.Devices) != 6 {
		t.Fatalf("expected 6 devices, got %v", len(c.Devices))
	}

	assertDevice("test1", "192.168.1.1", "foo", "bar", &c.Devices[0], t)
	assertDevice("test2", "192.168.2.1", "test", "123", &c.Devices[1], t)

	featuresEnabled := []string{
		"Conntrack", "DHCP", "DHCPv6", "Pools", "Routes", "Optics", "WlanSTA",
		"WlanIF", "Ipsec", "Lte", "Netwatch", "Queue",
	}

	for _, feat := range featuresEnabled {
		assertFeature(feat, c.Features, t)
	}

	f := c.DeviceFeatures("testProfileMinimal")
	assertFeature("Firmware", f, t)
	assertFeature("Health", f, t)
	assertFeature("Monitor", f, t)

	dev := c.FindDevice("testDns")
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
	v, ok := f[name]
	if !ok {
		t.Fatalf("expected feature %s to be enabled - not in map ", name)
	}

	c, ok := v.(FeatureConf)
	if !ok {
		t.Fatalf("expected feature %s is not FeatureConf: %v ", name, v)
	}

	if !c.Enabled() {
		t.Fatalf("expected feature %s to be enabled", name)
	}
}

func assertNotFeature(name string, f Features, t *testing.T) {
	name = strings.ToLower(name)
	v, ok := f[name]
	if !ok {
		t.Fatalf("expected feature %s to be disabled - not in map", name)
	}
	c, ok := v.(FeatureConf)
	if !ok {
		t.Fatalf("expected feature %s is not FeatureConf: %v ", name, v)
	}

	if c.Enabled() {
		t.Fatalf("expected feature %s to be disabled", name)
	}
}

func TestValidatorsDevice(t *testing.T) {
	t.Run("errors", func(t *testing.T) {
		config := []byte(`
devices:
  - name: test1
    address: 192.168.1.1
    profile: test
    `)

		_, err := Load(bytes.NewReader(config), nil)
		if err == nil {
			t.Fatalf("expected error")
		}

		t.Logf("errors: %s", err)

		if !errors.Is(err, MissingFieldError("user")) {
			t.Fatalf("no error: missing field user")
		}
		if !errors.Is(err, MissingFieldError("password")) {
			t.Fatalf("no error: missing field password")
		}
		if !errors.Is(err, UnknownProfileError("test")) {
			t.Fatalf("no error: unknown profile test")
		}
	})

	t.Run("error: missing fields", func(t *testing.T) {
		config := []byte(`
devices:
  - profile: test
    user: test
    password: 1234
    `)

		_, err := Load(bytes.NewReader(config), nil)
		if err == nil {
			t.Fatalf("expected error")
		}

		t.Logf("errors: %s", err)

		if !errors.Is(err, MissingFieldError("name")) {
			t.Fatalf("no error: missing field name")
		}
		if !errors.Is(err, MissingFieldError("address")) {
			t.Fatalf("no error: missing field address")
		}
	})
}

func TestValidatorsDeviceSrv(t *testing.T) {
	config := []byte(`
devices:
  - srv:
      record:
    user: test
    `)

	_, err := Load(bytes.NewReader(config), nil)
	if err == nil {
		t.Fatalf("expected error")
	}

	t.Logf("errors: %s", err)

	if !errors.Is(err, MissingFieldError("password")) {
		t.Fatalf("no error: missing field password")
	}
	if !errors.Is(err, MissingFieldError("srv.record")) {
		t.Fatalf("no error: missing field password")
	}
}

func TestFeatures(t *testing.T) {
	t.Run("validate", func(t *testing.T) {
		t.Parallel()

		features := make(Features)
		features["abc"] = true
		features["cde"] = true
		features["fgh"] = FeatureConf{"enabled": true}
		features.normalize()

		collectors := []string{"abc", "cde", "fgh"}
		if err := features.validate(collectors); err != nil {
			t.Errorf("error not expected for %v: %s", collectors, err)
		}

		collectors = []string{"abc", "cde", "fgh", "ijk"}
		if err := features.validate(collectors); err != nil {
			t.Errorf("error not expected for %v: %s", collectors, err)
		}

		collectors = []string{}
		if err := features.validate(collectors); err != nil {
			t.Errorf("error not expected for %v: %s", collectors, err)
		}

		collectors = []string{"abc", "fgh"}
		if err := features.validate(collectors); !errors.Is(err, UnknownFeatureError("cde")) {
			t.Errorf("error expected for %v: %s", collectors, err)
		}

		collectors = []string{"fgh"}
		if err := features.validate(collectors); !errors.Is(err, UnknownFeatureError("cde")) && !errors.Is(err, UnknownFeatureError("abc")) {
			t.Errorf("error expected for %v: %s", collectors, err)
		}
	})

	t.Run("names", func(t *testing.T) {
		t.Parallel()

		features := make(Features)
		features["abc"] = true
		features["cde"] = true
		features["fgh"] = true
		features.normalize()

		names := features.FeatureNames()
		sort.Strings(names)
		if slices.Compare(names, []string{"abc", "cde", "fgh"}) != 0 {
			t.Errorf("wrong names: %v", names)
		}

		features["abc"] = false
		features["cde"] = false
		features["fgh"] = true
		features.normalize()

		names = features.FeatureNames()
		if slices.Compare(names, []string{"fgh"}) != 0 {
			t.Errorf("wrong names: %v", names)
		}
	})
}

func TestDeviceFeatures(t *testing.T) {
	tests := []struct {
		device   string
		features []string
	}{
		{
			"testProfileMinimal",
			[]string{"firmware", "health", "monitor", "resource"},
		},
		{
			"testProfileBasic",
			[]string{"dhcp", "dhcpl", "firmware", "health", "monitor", "resource", "routes", "wlanif"},
		},
		{
			// default profile
			"test1",
			[]string{"conntrack", "dhcp", "dhcpl", "dhcpv6", "ipsec", "lte", "netwatch", "optics", "pools", "queue", "resource", "routes", "wlanif", "wlansta"},
		},
	}

	b := loadTestFile(t)

	c, err := Load(bytes.NewReader(b), nil)
	if err != nil {
		t.Fatalf("could not parse: %v", err)
	}

	for _, dt := range tests {
		feats := c.DeviceFeatures(dt.device)
		names := feats.FeatureNames()
		sort.Strings(names)
		if slices.Compare(names, dt.features) != 0 {
			t.Errorf("invalid features for device %s: %v", dt.device, names)
		}

	}
}

func TestFirmwareVersionCompare(t *testing.T) {
	tests := [][]int{
		{5, 10, 15, 0},
		{1, 10, 15, 1},
		{1, 5, 15, 1},
		{1, 5, 10, 1},
		{1, 5, 20, 1},
		{1, 15, 20, 1},
		{5, 9, 20, 1},
		{5, 9, 10, 1},
		{5, 9, 0, 1},
		{5, 10, 1, 1},
		{5, 10, 10, 1},
		{6, 10, 15, -1},
		{5, 15, 15, -1},
		{5, 10, 20, -1},
		{6, 10, 20, -1},
		{6, 1, 20, -1},
		{6, 10, 16, -1},
		{6, 10, 1, -1},
	}

	ver := FirmwareVersion{5, 10, 15}

	for _, tc := range tests {
		if r := ver.Compare(tc[0], tc[1], tc[2]); r != tc[3] {
			t.Fatalf("compare failed for %v; expected %d got %d", tc, tc[3], r)
		}
	}
}
