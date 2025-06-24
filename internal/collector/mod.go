package collector

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"slices"
	"strconv"
	"strings"

	"mikrotik-exporter/internal/collectors"
	"mikrotik-exporter/internal/config"

	"github.com/miekg/dns"
)

// --------------------------------------------

var ErrNoServersDefined = errors.New("no servers defined")

func resolveServices(srvDNS *config.DNSServer, record string) ([]string, error) {
	var dnsServer string

	if srvDNS == nil {
		conf, err := dns.ClientConfigFromFile("/etc/resolv.conf")
		if err != nil {
			return nil, fmt.Errorf("load resolv.conf file error: %w", err)
		}

		if conf == nil || len(conf.Servers) == 0 {
			return nil, ErrNoServersDefined
		}

		dnsServer = net.JoinHostPort(conf.Servers[0], strconv.Itoa(config.DNSPort))
	} else {
		dnsServer = net.JoinHostPort(srvDNS.Address, strconv.Itoa(srvDNS.Port))
	}

	slog.Debug("resolve services", "dns_server", dnsServer, "record", record)

	dnsMsg := new(dns.Msg)
	dnsMsg.RecursionDesired = true
	dnsMsg.SetQuestion(dns.Fqdn(record), dns.TypeSRV)

	dnsCli := new(dns.Client)

	r, _, err := dnsCli.Exchange(dnsMsg, dnsServer)
	if err != nil {
		return nil, fmt.Errorf("dns query for %s error: %w", record, err)
	}

	result := make([]string, 0, len(r.Answer))

	for _, k := range r.Answer {
		if s, ok := k.(*dns.SRV); ok {
			slog.Debug("resolved services", "dns_server", dnsServer, "record", record, "result", s.Target)

			result = append(result, strings.TrimRight(s.Target, "."))
		}
	}

	return result, nil
}

// --------------------------------------------

type collectorInstances map[string]collectors.RouterOSCollector

// createCollectors create instances of collectors according to configuration.
func createCollectors(cfg *config.Config) collectorInstances {
	colls := make(map[string]collectors.RouterOSCollector)

	for _, k := range cfg.AllEnabledFeatures() {
		col := collectors.InstanateCollector(k)
		if col != nil {
			colls[k] = col

			slog.Default().Debug("new collector", "collector", k)
		} else {
			slog.Default().Error("unknown collector " + k)
		}
	}

	return colls
}

func (ci collectorInstances) get(names []string, features config.Features) []deviceCollectorRC {
	dcols := make([]deviceCollectorRC, 0, len(names))

	for _, n := range names {
		dcols = append(dcols, deviceCollectorRC{ci[n], n, features.ConfigFor(n)})
	}

	return dcols
}

func (ci collectorInstances) instances() []collectors.RouterOSCollector {
	return slices.Collect(maps.Values(ci))
}
