package collector

import (
	"errors"
	"fmt"

	"github.com/KarolBedkowski/routeros-go-client/proto"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

func init() {
	registerCollector("netwatch", newNetwatchCollector)
}

type netwatchCollector struct {
	propslist  string
	statusDesc *prometheus.Desc
}

func newNetwatchCollector() routerOSCollector {
	labelNames := []string{"name", "address", "host", "comment", "status"}
	c := &netwatchCollector{
		propslist:  "host,comment,status",
		statusDesc: descriptionForPropertyName("netwatch", "status", labelNames),
	}

	return c
}

func (c *netwatchCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- c.statusDesc
}

func (c *netwatchCollector) collect(ctx *collectorContext) error {
	stats, err := c.fetch(ctx)
	if err != nil {
		return err
	}

	for _, re := range stats {
		c.collectStatus(re.Map["host"], re.Map["comment"], re, ctx)
	}

	return nil
}

func (c *netwatchCollector) fetch(ctx *collectorContext) ([]*proto.Sentence, error) {
	reply, err := ctx.client.Run("/tool/netwatch/print", "?disabled=false", "=.proplist="+c.propslist)
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching netwatch metrics")

		return nil, fmt.Errorf("get netwatch error: %w", err)
	}

	return reply.Re, nil
}

var ErrUnexpectedStatus = errors.New("unexpected netwatch status value")

func (c *netwatchCollector) collectStatus(
	host, comment string, re *proto.Sentence, ctx *collectorContext,
) {
	if value := re.Map["status"]; value != "" {
		var upVal, downVal, unknownVal float64

		switch value {
		case "up":
			upVal = 1
		case "unknown":
			unknownVal = 1
		case "down":
			downVal = 1
		default:
			log.WithFields(log.Fields{
				"device":   ctx.device.Name,
				"host":     host,
				"property": "status",
				"value":    value,
				"error":    ErrUnexpectedStatus,
			}).Error("error parsing netwatch metric value")
		}

		desc := c.statusDesc
		ctx.ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue,
			upVal, ctx.device.Name, ctx.device.Address, host, comment, "up")
		ctx.ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue,
			downVal, ctx.device.Name, ctx.device.Address, host, comment, "down")
		ctx.ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue,
			unknownVal, ctx.device.Name, ctx.device.Address, host, comment, "unknown")
	}
}
