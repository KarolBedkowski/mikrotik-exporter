package collector

import (
	"fmt"

	"github.com/KarolBedkowski/routeros-go-client/proto"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

func init() {
	registerCollector("interface", newInterfaceCollector)
}

type interfaceCollector struct {
	actualMtuDesc *prometheus.Desc
	runningDesc   *prometheus.Desc
	rxByteDesc    *prometheus.Desc
	txByteDesc    *prometheus.Desc
	rxPacketDesc  *prometheus.Desc
	txPacketDesc  *prometheus.Desc
	rxErrorDesc   *prometheus.Desc
	txErrorDesc   *prometheus.Desc
	rxDropDesc    *prometheus.Desc
	txDropDesc    *prometheus.Desc
	linkDownsDesc *prometheus.Desc
}

func newInterfaceCollector() routerOSCollector {
	labelNames := []string{"name", "address", "interface", "type", "disabled", "comment", "running", "slave"}

	collector := &interfaceCollector{
		actualMtuDesc: descriptionForPropertyName("interface", "actual_mtu_total", labelNames),
		runningDesc:   descriptionForPropertyName("interface", "running_total", labelNames),
		rxByteDesc:    descriptionForPropertyName("interface", "rx-byte_total", labelNames),
		txByteDesc:    descriptionForPropertyName("interface", "tx-byte_total", labelNames),
		rxPacketDesc:  descriptionForPropertyName("interface", "rx-packet_total", labelNames),
		txPacketDesc:  descriptionForPropertyName("interface", "tx-packet_total", labelNames),
		rxErrorDesc:   descriptionForPropertyName("interface", "rx-error_total", labelNames),
		txErrorDesc:   descriptionForPropertyName("interface", "tx-error_total", labelNames),
		rxDropDesc:    descriptionForPropertyName("interface", "rx-drop_total", labelNames),
		txDropDesc:    descriptionForPropertyName("interface", "tx-drop_total", labelNames),
		linkDownsDesc: descriptionForPropertyName("interface", "link-downs_total", labelNames),
	}

	return collector
}

func (c *interfaceCollector) describe(ch chan<- *prometheus.Desc) {
	ch <- c.actualMtuDesc
	ch <- c.runningDesc
	ch <- c.rxByteDesc
	ch <- c.txByteDesc
	ch <- c.rxPacketDesc
	ch <- c.txPacketDesc
	ch <- c.rxErrorDesc
	ch <- c.txErrorDesc
	ch <- c.rxDropDesc
	ch <- c.txDropDesc
	ch <- c.linkDownsDesc
}

func (c *interfaceCollector) collect(ctx *collectorContext) error {
	stats, err := c.fetch(ctx)
	if err != nil {
		return err
	}

	for _, re := range stats {
		c.collectForStat(re, ctx)
	}

	return nil
}

func (c *interfaceCollector) fetch(ctx *collectorContext) ([]*proto.Sentence, error) {
	reply, err := ctx.client.Run("/interface/print",
		"=.proplist=name,type,disabled,comment,slave,actual-mtu,running,rx-byte,tx-byte,"+
			"rx-packet,tx-packet,rx-error,tx-error,rx-drop,tx-drop,link-downs")
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching interface metrics")

		return nil, fmt.Errorf("get interfaces detail error: %w", err)
	}

	return reply.Re, nil
}

func (c *interfaceCollector) collectForStat(reply *proto.Sentence, ctx *collectorContext) {
	pcl := newPropertyCollector(reply, ctx,
		reply.Map["name"], reply.Map["type"], reply.Map["disabled"], reply.Map["comment"],
		reply.Map["running"], reply.Map["slave"])

	_ = pcl.collectGaugeValue(c.actualMtuDesc, "actual_mtu", nil)
	_ = pcl.collectGaugeValue(c.runningDesc, "running", convertFromBool)
	_ = pcl.collectCounterValue(c.rxByteDesc, "rx-byte", nil)
	_ = pcl.collectCounterValue(c.txByteDesc, "tx-byte", nil)
	_ = pcl.collectCounterValue(c.rxPacketDesc, "rx-packet", nil)
	_ = pcl.collectCounterValue(c.txPacketDesc, "tx-packet", nil)
	_ = pcl.collectCounterValue(c.rxErrorDesc, "rx-error", nil)
	_ = pcl.collectCounterValue(c.txErrorDesc, "tx-error", nil)
	_ = pcl.collectCounterValue(c.rxDropDesc, "rx-drop", nil)
	_ = pcl.collectCounterValue(c.txDropDesc, "tx-drop", nil)
	_ = pcl.collectCounterValue(c.linkDownsDesc, "link-downs", nil)
}
