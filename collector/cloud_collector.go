package collector

import (
	"fmt"
	"strings"

	"github.com/KarolBedkowski/routeros-go-client/proto"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

func init() {
	registerCollector("cloud", newCloudCollector)
}

type cloudCollector struct {
	propslist       string
	ifaceStatusDesc *prometheus.Desc
}

func newCloudCollector() routerOSCollector {
	labelNames := []string{"name", "address"}

	c := &cloudCollector{
		propslist:       "status,rate,full-duplex",
		ifaceStatusDesc: descriptionForPropertyName("cloud", "status", append(labelNames, "status")),
	}

	return c
}

func (c *cloudCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- c.ifaceStatusDesc
}

func (c *cloudCollector) collect(ctx *collectorContext) error {
	reply, err := ctx.client.Run("/ip/cloud/print")
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching cloud")

		return fmt.Errorf("get cloud error: %w", err)
	}

	if len(reply.Re) != 1 {
		return nil
	}

	c.collectStatus(reply.Re[0], ctx)

	return nil
}

func (c *cloudCollector) collectStatus(se *proto.Sentence, ctx *collectorContext) {
	var (
		statusUnknown  = 0.0
		statusUpdated  = 0.0
		statusUpdating = 0.0
		statusError    = 0.0
	)

	v, ok := se.Map["status"]
	if !ok {
		return
	}

	v = strings.ToLower(v)

	switch {
	case v == "updated":
		statusUpdated = 1.0
	case strings.HasPrefix(v, "updating"):
		statusUpdating = 1.0
	case strings.HasPrefix(v, "error"):
		statusError = 1.0
	default:
		statusUnknown = 1.0
	}

	ctx.ch <- prometheus.MustNewConstMetric(c.ifaceStatusDesc, prometheus.GaugeValue,
		statusUnknown, ctx.device.Name, ctx.device.Address, "unknown")
	ctx.ch <- prometheus.MustNewConstMetric(c.ifaceStatusDesc, prometheus.GaugeValue,
		statusUpdated, ctx.device.Name, ctx.device.Address, "updated")
	ctx.ch <- prometheus.MustNewConstMetric(c.ifaceStatusDesc, prometheus.GaugeValue,
		statusUpdating, ctx.device.Name, ctx.device.Address, "updating")
	ctx.ch <- prometheus.MustNewConstMetric(c.ifaceStatusDesc, prometheus.GaugeValue,
		statusError, ctx.device.Name, ctx.device.Address, "error")
}
