package collector

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/KarolBedkowski/routeros-go-client/proto"
	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
)

// TODO: probably totally broken

func init() {
	registerCollector("lte", newLteCollector)
}

type lteCollector struct {
	props        []string
	propslist    string
	descriptions map[string]*prometheus.Desc
}

func newLteCollector() routerOSCollector {
	c := &lteCollector{
		descriptions: make(map[string]*prometheus.Desc),
	}

	c.props = []string{"current-cellid", "primary-band", "ca-band", "rssi", "rsrp", "rsrq", "sinr"}
	c.propslist = strings.Join(c.props, ",")
	labelNames := []string{"name", "address", "interface", "cellid", "primaryband", "caband"}

	for _, p := range c.props {
		c.descriptions[p] = descriptionForPropertyName("lte_interface", p, labelNames)
	}

	return c
}

func (c *lteCollector) describe(ch chan<- *prometheus.Desc) {
	for _, d := range c.descriptions {
		ch <- d
	}
}

func (c *lteCollector) collect(ctx *collectorContext) error {
	names, err := c.fetchInterfaceNames(ctx)
	if err != nil {
		return err
	}

	for _, n := range names {
		err := c.collectForInterface(n, ctx)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *lteCollector) fetchInterfaceNames(ctx *collectorContext) ([]string, error) {
	reply, err := ctx.client.Run("/interface/lte/print", "?disabled=false", "=.proplist=name")
	if err != nil {
		return nil, fmt.Errorf("fetch lte interface names error: %w", err)
	}

	return extractPropertyFromReplay(reply, "name"), nil
}

func (c *lteCollector) collectForInterface(iface string, ctx *collectorContext) error {
	reply, err := ctx.client.Run("/interface/lte/info", "=number="+iface, "=once=", "=.proplist="+c.propslist)
	if err != nil {
		return fmt.Errorf("fetch %s lte interface statistics error: %w", iface, err)
	}

	if len(reply.Re) == 0 {
		return nil
	}

	var errs *multierror.Error

	for _, p := range c.props[3:] {
		// there's always going to be only one sentence in reply, as we
		// have to explicitly specify the interface
		if err := c.collectMetricForProperty(p, iface, reply.Re[0], ctx); err != nil {
			errs = multierror.Append(errs, fmt.Errorf("collect %s for %s error: %w", p, iface, err))
		}
	}

	return errs.ErrorOrNil()
}

func (c *lteCollector) collectMetricForProperty(property, iface string,
	reply *proto.Sentence, ctx *collectorContext,
) error {
	desc := c.descriptions[property]
	currentCellID := reply.Map["current-cellid"]
	// get only band and its width, drop earfcn and phy-cellid info
	primaryband := reply.Map["primary-band"]
	if primaryband != "" {
		primaryband = strings.Fields(primaryband)[0]
	}

	caband := reply.Map["ca-band"]
	if caband != "" {
		caband = strings.Fields(caband)[0]
	}

	value := reply.Map[property]
	if value == "" {
		return nil
	}

	v, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fmt.Errorf("parse %v error: %w", value, err)
	}

	ctx.ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v,
		ctx.device.Name, ctx.device.Address,
		iface, currentCellID, primaryband, caband)

	return nil
}
