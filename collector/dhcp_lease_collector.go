package collector

import (
	"fmt"
	"strconv"

	"github.com/KarolBedkowski/routeros-go-client/proto"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

func init() {
	registerCollector("dhcpl", newDHCPLCollector)
}

type dhcpLeaseCollector struct {
	descriptions *prometheus.Desc
}

func newDHCPLCollector() routerOSCollector {
	labelNames := []string{"name", "address", "activemacaddress", "server", "status", "activeaddress", "hostname"}
	c := &dhcpLeaseCollector{
		descriptions: description("dhcp", "leases_metrics", "number of metrics", labelNames),
	}

	return c
}

func (c *dhcpLeaseCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- c.descriptions
}

func (c *dhcpLeaseCollector) collect(ctx *collectorContext) error {
	stats, err := c.fetch(ctx)
	if err != nil {
		return err
	}

	for _, re := range stats {
		c.collectMetric(ctx, re)
	}

	return nil
}

func (c *dhcpLeaseCollector) fetch(ctx *collectorContext) ([]*proto.Sentence, error) {
	reply, err := ctx.client.Run("/ip/dhcp-server/lease/print", "?status=bound",
		"=.proplist=active-mac-address,server,status,active-address,host-name")
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
			"reply":  reply,
		}).Error("error fetching DHCP leases metrics")

		return nil, fmt.Errorf("get lease error: %w", err)
	}

	return reply.Re, nil
}

func (c *dhcpLeaseCollector) collectMetric(ctx *collectorContext, re *proto.Sentence) {
	// QuoteToASCII because of broken DHCP clients
	hostname := strconv.QuoteToASCII(re.Map["host-name"])

	metric, err := prometheus.NewConstMetric(c.descriptions, prometheus.GaugeValue, 1.0,
		ctx.device.Name, ctx.device.Address,
		re.Map["active-mac-address"], re.Map["server"], re.Map["status"], re.Map["active-address"],
		hostname)
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
			"reply":  re,
		}).Error("error parsing dhcp lease")

		return
	}

	ctx.ch <- metric
}
