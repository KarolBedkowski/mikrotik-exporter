package collector

import (
	"fmt"

	"github.com/KarolBedkowski/routeros-go-client/proto"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
)

func init() {
	registerCollector("wlansta", newWlanSTACollector)
}

type wlanSTACollector struct {
	signalToNoseDesc   *prometheus.Desc
	signalStrengthDesc *prometheus.Desc
	packetsDesc        *TXRXDecription
	bytesDesc          *TXRXDecription
	framesDesc         *TXRXDecription
}

func newWlanSTACollector() routerOSCollector {
	labelNames := []string{"name", "address", "interface", "mac_address"}

	collector := &wlanSTACollector{
		signalToNoseDesc:   descriptionForPropertyName("wlan_station", "signal-to-noise", labelNames),
		signalStrengthDesc: descriptionForPropertyName("wlan_station", "signal-strength", labelNames),
		packetsDesc:        NewTXRXDescription("wlan_station", "packets_total", labelNames),
		bytesDesc:          NewTXRXDescription("wlan_station", "bytes_total", labelNames),
		framesDesc:         NewTXRXDescription("wlan_station", "frames_total", labelNames),
	}

	return collector
}

func (c *wlanSTACollector) describe(ch chan<- *prometheus.Desc) {
	ch <- c.signalToNoseDesc
	ch <- c.signalStrengthDesc

	c.packetsDesc.describe(ch)
	c.bytesDesc.describe(ch)
	c.framesDesc.describe(ch)
}

func (c *wlanSTACollector) collect(ctx *collectorContext) error {
	stats, err := c.fetch(ctx)
	if err != nil {
		return err
	}

	for _, re := range stats {
		c.collectForStat(re, ctx)
	}

	return nil
}

func (c *wlanSTACollector) fetch(ctx *collectorContext) ([]*proto.Sentence, error) {
	reply, err := ctx.client.Run("/interface/wireless/registration-table/print",
		"=.proplist=interface,mac-address,signal-to-noise,signal-strength,packets,bytes,frames")
	if err != nil {
		log.WithFields(log.Fields{
			"device": ctx.device.Name,
			"error":  err,
		}).Error("error fetching wlan station metrics")

		return nil, fmt.Errorf("read wireless reg error: %w", err)
	}

	return reply.Re, nil
}

func (c *wlanSTACollector) collectForStat(re *proto.Sentence, ctx *collectorContext) {
	pcl := newPropertyCollector(re, ctx, re.Map["interface"], re.Map["mac-address"])
	_ = pcl.collectGaugeValue(c.signalToNoseDesc, "signal-to-noise", nil)
	_ = pcl.collectGaugeValue(c.signalStrengthDesc, "signal-strength", nil)
	_ = pcl.collectRXTXCounterValue(c.packetsDesc, "packets", nil)
	_ = pcl.collectRXTXCounterValue(c.bytesDesc, "bytes", nil)
	_ = pcl.collectRXTXCounterValue(c.framesDesc, "frames", nil)
}
