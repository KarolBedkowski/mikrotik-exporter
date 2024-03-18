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
	leases propertyMetricCollector
}

func newDHCPLCollector() routerOSCollector {
	labelNames := []string{"name", "address", "activemacaddress", "server", "status", "activeaddress", "hostname"}
	c := &dhcpLeaseCollector{
		leases: newPropertyGaugeMetric("dhcp", "status", labelNames).
			withName("leases_metrics").withHelp("number of metrics").
			withConverter(convertToOne).build(),
	}

	return c
}

func (c *dhcpLeaseCollector) describe(ch chan<- *prometheus.Desc) {
	c.leases.describe(ch)
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
	labels := []string{
		re.Map["active-mac-address"], re.Map["server"], re.Map["status"],
		re.Map["active-address"],
		// QuoteToASCII because of broken DHCP clients
		strconv.QuoteToASCII(re.Map["host-name"]),
	}

	_ = c.leases.collect(re, ctx, labels)
}
