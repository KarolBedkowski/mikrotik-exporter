package collector

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

func init() {
	registerCollector("pools", newPoolCollector)
}

type poolCollector struct {
	usedCountDesc *prometheus.Desc
}

func newPoolCollector() routerOSCollector {
	const prefix = "ip_pool"

	labelNames := []string{"name", "address", "ip_version", "pool"}
	c := &poolCollector{
		usedCountDesc: description(prefix, "pool_used", "number of used IP/prefixes in a pool", labelNames),
	}

	return c
}

func (c *poolCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- c.usedCountDesc
}

func (c *poolCollector) collect(ctx *collectorContext) error {
	return c.collectForIPVersion("4", "ip", ctx)
}

func (c *poolCollector) collectForIPVersion(ipVersion, topic string, ctx *collectorContext) error {
	names, err := c.fetchPoolNames(ipVersion, topic, ctx)
	if err != nil {
		return err
	}

	for _, n := range names {
		if err := c.collectForPool(ipVersion, topic, n, ctx); err != nil {
			return err
		}
	}

	return nil
}

func (c *poolCollector) fetchPoolNames(ipVersion, topic string, ctx *collectorContext) ([]string, error) {
	reply, err := ctx.client.Run("/"+topic+"/pool/print", "=.proplist=name")
	if err != nil {
		log.WithFields(log.Fields{
			"device":     ctx.device.Name,
			"ip_version": ipVersion,
			"topic":      topic,
			"error":      err,
		}).Error("error fetching pool names")

		return nil, fmt.Errorf("get pool %s error: %w", topic, err)
	}

	names := make([]string, 0, len(reply.Re))
	for _, re := range reply.Re {
		names = append(names, re.Map["name"])
	}

	return names, nil
}

func (c *poolCollector) collectForPool(ipVersion, topic, pool string, ctx *collectorContext) error {
	reply, err := ctx.client.Run("/"+topic+"/pool/used/print", "?pool="+pool, "=count-only=")
	if err != nil {
		log.WithFields(log.Fields{
			"pool":       pool,
			"topic":      topic,
			"ip_version": ipVersion,
			"device":     ctx.device.Name,
			"error":      err,
		}).Error("error fetching pool counts")

		return fmt.Errorf("get used pool %s/%s error: %w", topic, pool, err)
	}

	rcl := newRetCollector(reply, ctx, ipVersion, pool)

	return rcl.collectGaugeValue(c.usedCountDesc, nil)
}
