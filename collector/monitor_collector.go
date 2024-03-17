package collector

import (
	"fmt"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

func init() {
	registerCollector("monitor", newMonitorCollector)
}

type monitorCollector struct {
	ifaceStatusDesc *prometheus.Desc
	ifaceRateDesc   *prometheus.Desc
	ifaceDuplexDesc *prometheus.Desc
}

func newMonitorCollector() routerOSCollector {
	labelNames := []string{"name", "address", "interface"}

	c := &monitorCollector{
		ifaceStatusDesc: descriptionForPropertyName("monitor", "status", labelNames),
		ifaceRateDesc:   descriptionForPropertyName("monitor", "rate", labelNames),
		ifaceDuplexDesc: descriptionForPropertyName("monitor", "full-duplex", labelNames),
	}

	return c
}

func (c *monitorCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- c.ifaceStatusDesc
	ch <- c.ifaceRateDesc
	ch <- c.ifaceDuplexDesc
}

func (c *monitorCollector) collect(ctx *collectorContext) error {
	reply, err := ctx.client.Run("/interface/ethernet/print", "=.proplist=name")
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching ethernet interfaces")

		return fmt.Errorf("get ethernet error: %w", err)
	}

	eths := make([]string, len(reply.Re))
	for idx, eth := range reply.Re {
		eths[idx] = eth.Map["name"]
	}

	return c.collectForMonitor(eths, ctx)
}

func (c *monitorCollector) collectForMonitor(eths []string, ctx *collectorContext) error {
	reply, err := ctx.client.Run("/interface/ethernet/monitor",
		"=numbers="+strings.Join(eths, ","),
		"=once=",
		"=.proplist=name,status,rate,full-duplex")
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching ethernet monitor info")

		return fmt.Errorf("get ethernet monitor error: %w", err)
	}

	for _, e := range reply.Re {
		name := e.Map["name"]
		pcl := newPropertyCollector(e, ctx, name)
		_ = pcl.collectGaugeValue(c.ifaceStatusDesc, "status", convertFromStatus)
		_ = pcl.collectGaugeValue(c.ifaceRateDesc, "rate", convertFromRate)
		_ = pcl.collectGaugeValue(c.ifaceDuplexDesc, "full-duplex", convertFromBool)
	}

	return nil
}

func convertFromStatus(value string) (float64, error) {
	if value == "link-ok" {
		return 1.0, nil
	}

	return 0.0, nil
}

func convertFromRate(v string) (float64, error) {
	value := 0

	switch v {
	case "10Mbps":
		value = 10
	case "100Mbps":
		value = 100
	case "1Gbps":
		value = 1000
	case "2.5Gbps":
		value = 2500
	case "5Gbps":
		value = 5000
	case "10Gbps":
		value = 10000
	case "25Gbps":
		value = 25000
	case "40Gbps":
		value = 40000
	case "50Gbps":
		value = 50000
	case "100Gbps":
		value = 100000
	}

	return float64(value), nil
}
