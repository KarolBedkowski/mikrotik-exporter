package collector

import (
	"errors"
	"fmt"

	"github.com/KarolBedkowski/routeros-go-client/proto"
	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

func init() {
	registerCollector("netwatch", newNetwatchCollector, "retrieves Netwatch status")
}

type netwatchCollector struct {
	statusDesc *prometheus.Desc
}

func newNetwatchCollector() routerOSCollector {
	labelNames := []string{"name", "address", "host", "comment", "status"}

	return &netwatchCollector{
		statusDesc: descriptionForPropertyName("netwatch", "status", labelNames),
	}
}

func (c *netwatchCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- c.statusDesc
}

func (c *netwatchCollector) collect(ctx *collectorContext) error {
	reply, err := ctx.client.Run("/tool/netwatch/print", "?disabled=false",
		"=.proplist=host,comment,status")
	if err != nil {
		return fmt.Errorf("fetch netwatch error: %w", err)
	}

	var errs *multierror.Error

	for _, re := range reply.Re {
		if err := c.collectStatus(re.Map["host"], re.Map["comment"], re, ctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect error %w", err))
		}
	}

	return errs.ErrorOrNil()
}

var ErrUnexpectedStatus = errors.New("unexpected netwatch status value")

func (c *netwatchCollector) collectStatus(
	host, comment string, re *proto.Sentence, ctx *collectorContext,
) error {
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
			return fmt.Errorf("parse value %v for host %s (%v) error: %w", value, host, comment, ErrUnexpectedStatus)
		}

		desc := c.statusDesc
		ctx.ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue,
			upVal, ctx.device.Name, ctx.device.Address, host, comment, "up")
		ctx.ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue,
			downVal, ctx.device.Name, ctx.device.Address, host, comment, "down")
		ctx.ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue,
			unknownVal, ctx.device.Name, ctx.device.Address, host, comment, "unknown")
	}

	return nil
}
