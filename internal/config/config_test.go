package config

import (
	"bytes"
	"errors"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldParse(t *testing.T) {
	b := loadTestFile(t)

	c, err := Load(bytes.NewReader(b), nil)
	require.NoError(t, err, "load error")

	require.Equalf(t, 6, len(c.Devices), "expected 6 devices, got %+v", c.Devices)

	assertDevice("test1", "192.168.1.1", "foo", "bar", &c.Devices[0], t)
	assertDevice("test2", "192.168.2.1", "test", "123", &c.Devices[1], t)

	featuresEnabled := []string{
		"Conntrack", "DHCP", "DHCPv6", "Pools", "Routes", "Optics", "Ipsec", "Lte", "Netwatch", "Queue",
	}

	for _, feat := range featuresEnabled {
		assertFeature(feat, c.Features, t)
	}

	f := c.DeviceFeatures("testProfileMinimal")
	assertFeature("Firmware", f, t)
	assertFeature("Health", f, t)
	assertFeature("Monitor", f, t)

	dev := c.FindDevice("testDns")
	assert.Equal(t, "record2", dev.Srv.Record, "invalid service")
	assert.Equal(t, "dnsaddress", dev.Srv.DNS.Address, "invalid dns address")
	assert.Equal(t, 1053, dev.Srv.DNS.Port, "invalid dns port")
}

func loadTestFile(t *testing.T) []byte {
	b, err := os.ReadFile("config.test.yml")
	require.NoErrorf(t, err, "could not load config: %v", err)

	return b
}

func assertDevice(name, address, user, password string, c *Device, t *testing.T) {
	assert.Equal(t, name, c.Name)
	assert.Equal(t, address, c.Address)
	assert.Equal(t, user, c.User)
	assert.Equal(t, password, c.Password)
}

func assertFeature(name string, f Features, t *testing.T) {
	name = strings.ToLower(name)
	v, ok := f[name]

	assert.True(t, ok, "expected feature %s to be enabled - not in map ", name)
	if ok {
		assert.True(t, v.Enabled(), "expected feature %s to be enabled", name)
	}
}

// func assertNotFeature(name string, f Features, t *testing.T) {
// 	name = strings.ToLower(name)
// 	v, ok := f[name]
// 	if !ok {
// 		t.Fatalf("expected feature %s to be disabled - not in map", name)
// 	}

// 	if v.Enabled() {
// 		t.Fatalf("expected feature %s to be disabled", name)
// 	}
// }

func TestValidatorsDevice(t *testing.T) {
	t.Run("errors", func(t *testing.T) {
		config := []byte(`
devices:
  - name: test1
    address: 192.168.1.1
    profile: test
    `)

		_, err := Load(bytes.NewReader(config), nil)
		require.Error(t, err, "expected load error")

		t.Logf("expected errors: %s", err)

		assert.ErrorIs(t, err, MissingFieldError("user"))
		assert.ErrorIs(t, err, MissingFieldError("password"))
		assert.ErrorIs(t, err, UnknownProfileError("test"))
	})

	t.Run("error: missing fields", func(t *testing.T) {
		config := []byte(`
devices:
  - profile: test
    user: test
    password: 1234
    `)

		_, err := Load(bytes.NewReader(config), nil)
		require.Error(t, err, "expected load error")

		t.Logf("errors: %s", err)

		assert.ErrorIs(t, err, MissingFieldError("name"))
		assert.ErrorIs(t, err, MissingFieldError("address"))
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
		features["abc"] = NewFeatureConf()
		features["cde"] = NewFeatureConf()
		features["fgh"] = FeatureConf{"enabled": true}

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
		features["abc"] = NewFeatureConf()
		features["cde"] = NewFeatureConf()
		features["fgh"] = NewFeatureConf()

		names := features.FeatureNames()
		sort.Strings(names)
		if slices.Compare(names, []string{"abc", "cde", "fgh"}) != 0 {
			t.Errorf("wrong names: %v", names)
		}

		features["abc"] = FeatureConf{"enabled": false}
		features["cde"] = FeatureConf{"enabled": false}
		features["fgh"] = FeatureConf{"enabled": true}

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
			[]string{"conntrack", "dhcp", "dhcpl", "dhcpv6", "ipsec", "lte", "netwatch", "optics", "pools", "queue", "resource", "routes"},
		},
	}

	b := loadTestFile(t)

	c, err := Load(bytes.NewReader(b), nil)
	assert.NoError(t, err, "could not parse file")

	for _, dt := range tests {
		feats := c.DeviceFeatures(dt.device)
		names := feats.FeatureNames()
		sort.Strings(names)
		assert.Equalf(t, dt.features, names, "invalid features for device %s", dt.device)
	}
}

func TestDeviceFeaturesConf(t *testing.T) {
	b := loadTestFile(t)

	c, err := Load(bytes.NewReader(b), nil)
	assert.NoError(t, err, "could not parse file")

	// test custom config
	feats := c.DeviceFeatures("test1") // default profile
	lte := feats["lte"]
	assert.Equal(t, 123, lte["settings1"], "invalid setting1 for lte for test1 dev")
	assert.Equal(t, "abc", lte["settings2"], "invalid setting2 for lte for test1 dev")
	assert.True(t, lte["enabled"].(bool), "invalid enabled for lte for test1 dev")
	assert.True(t, lte.BoolValue("enabled", false), "invalid enabled for lte for test1 dev")
	assert.True(t, lte.Enabled(), "invalid enabled for lte for test1 dev")
	assert.True(t, lte.BoolValue("enabled", false), "invalid enabled for lte for test1 dev")

	feat := feats["firmware"]
	assert.Nilf(t, feat, "found firmware for test1: %+v", feat)

	// empty dict
	feat = feats["queue"]
	assert.Equalf(t, 1, len(feat), "queue features should have only on item for test1: %+v", feat)
	assert.Truef(t, feat.BoolValue("enabled", false), "queue features should be enabled for test1: %+v", feat)
	assert.Truef(t, feat.Enabled(), "invalid enabled for queue for test1: %+v", feat)

	// via enable property
	feat = feats["routes"]
	assert.Truef(t, feat.BoolValue("enabled", false), "invalid enabled for routes for test1: %+v", feat)
	assert.Truef(t, feat.Enabled(), "invalid enabled for routes for test1 dev: %+v", feat)

	// disable via enable property
	feat = feats["wlanif"]
	assert.Falsef(t, feat.BoolValue("enabled", false), "invalid enabled for wlanif for test1: %+v", feat)
	assert.Falsef(t, feat.Enabled(), "invalid enabled for wlanif for test1 dev: %+v", feat)

	// disable via false
	feat = feats["wlansta"]
	assert.Falsef(t, feat.BoolValue("enabled", false), "invalid enabled for wlansta for test1: %+v", feat)
	assert.Falsef(t, feat.Enabled(), "invalid enabled for wlansta for test1 dev: %+v", feat)

	feats = c.DeviceFeatures("testProfileBasic")
	feat = feats["scripts"]
	assert.Falsef(t, feat.BoolValue("enabled", false), "invalid enabled for scripts  for testProfileBasic: %+v", feat)
	assert.Falsef(t, feat.Enabled(), "invalid enabled for routes for wlanif dev: %+v", feat)

	scripts, err := feat.Strs("scripts")
	assert.NoError(t, err)
	assert.Equal(t, 2, len(scripts))
	assert.Equal(t, []string{"script1", "script2"}, scripts)
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
		assert.Equalf(t, tc[3], ver.Compare(tc[0], tc[1], tc[2]), "compare failed for %v", tc)
	}
}
