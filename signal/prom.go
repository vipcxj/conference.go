package signal

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/vipcxj/conference.go/config"
)

type Metrics struct {
	reg                    *prometheus.Registry
	signalConnectsTotal    *prometheus.CounterVec
	signalDisconnectsTotal *prometheus.CounterVec
	signalConnectingTotal  *prometheus.GaugeVec
	webrtcConnectsTotal    *prometheus.CounterVec
	webrtcDisconnectsTotal *prometheus.CounterVec
	webrtcConnectingTotal  *prometheus.GaugeVec
	webrtcReadBytesTotal   *prometheus.CounterVec
	webrtcWriteBytesTotal  *prometheus.CounterVec
}

func NewMetrics(reg *prometheus.Registry, conf *config.ConferenceConfigure) *Metrics {
	cfg := &conf.Signal.Prometheus
	fatory := promauto.With(reg)
	nodeName := conf.ClusterNodeName()
	constLabels := make(prometheus.Labels)
	constLabels["node_name"] = nodeName
	commonLabels := []string{}
	return &Metrics{
		reg: reg,
		signalConnectsTotal: fatory.NewCounterVec(prometheus.CounterOpts{
			Namespace:   cfg.Namespace,
			Subsystem:   cfg.Subsystem,
			ConstLabels: constLabels,
			Name:        "signal_connects_total",
			Help:        "Total number of signal connections opened",
		}, commonLabels),
		signalDisconnectsTotal: fatory.NewCounterVec(prometheus.CounterOpts{
			Namespace:   cfg.Namespace,
			Subsystem:   cfg.Subsystem,
			ConstLabels: constLabels,
			Name:        "signal_disconnects_total",
			Help:        "Total number of signal connections closed",
		}, commonLabels),
		signalConnectingTotal: fatory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   cfg.Namespace,
			Subsystem:   cfg.Subsystem,
			ConstLabels: constLabels,
			Name:        "signal_connecting_total",
			Help:        "Total number of active signal connections",
		}, commonLabels),
		webrtcConnectsTotal: fatory.NewCounterVec(prometheus.CounterOpts{
			Namespace:   cfg.Namespace,
			Subsystem:   cfg.Subsystem,
			ConstLabels: constLabels,
			Name:        "webrtc_connects_total",
			Help:        "Total number of webrtc connections opened",
		}, commonLabels),
		webrtcDisconnectsTotal: fatory.NewCounterVec(prometheus.CounterOpts{
			Namespace:   cfg.Namespace,
			Subsystem:   cfg.Subsystem,
			ConstLabels: constLabels,
			Name:        "webrtc_disconnects_total",
			Help:        "Total number of webrtc connections closed",
		}, commonLabels),
		webrtcConnectingTotal: fatory.NewGaugeVec(prometheus.GaugeOpts{
			Namespace:   cfg.Namespace,
			Subsystem:   cfg.Subsystem,
			ConstLabels: constLabels,
			Name:        "webrtc_connecting_total",
			Help:        "Total number of active webrtc connections",
		}, commonLabels),
		webrtcReadBytesTotal: fatory.NewCounterVec(prometheus.CounterOpts{
			Namespace:   cfg.Namespace,
			Subsystem:   cfg.Subsystem,
			ConstLabels: constLabels,
			Name:        "webrtc_read_bytes_total",
			Help:        "Total number of bytes webrtc read",
		}, commonLabels),
		webrtcWriteBytesTotal: fatory.NewCounterVec(prometheus.CounterOpts{
			Namespace:   cfg.Namespace,
			Subsystem:   cfg.Subsystem,
			ConstLabels: constLabels,
			Name:        "webrtc_write_bytes_total",
			Help:        "Total number of bytes webrtc write",
		}, commonLabels),
	}
}

func (me *Metrics) OnSignalConnectStart(ctx *SignalContext) {
	if me == nil {
		return
	}
	me.signalConnectsTotal.WithLabelValues().Inc()
	me.signalConnectingTotal.WithLabelValues().Inc()
}

func (me *Metrics) OnSignalConnectClose(ctx *SignalContext) {
	if me == nil {
		return
	}
	me.signalDisconnectsTotal.WithLabelValues().Inc()
	me.signalConnectingTotal.WithLabelValues().Dec()
}

func (me *Metrics) OnWebrtcConnectStart(ctx *SignalContext) {
	if me == nil {
		return
	}
	me.webrtcConnectsTotal.WithLabelValues().Inc()
	me.webrtcConnectingTotal.WithLabelValues().Inc()
}

func (me *Metrics) OnWebrtcConnectClose(ctx *SignalContext) {
	if me == nil {
		return
	}
	me.webrtcDisconnectsTotal.WithLabelValues().Inc()
	me.webrtcConnectingTotal.WithLabelValues().Dec()
}

func (me *Metrics) OnWebrtcRtpRead(ctx *SignalContext, nBytes int) {
	if me == nil {
		return
	}
	me.webrtcReadBytesTotal.WithLabelValues().Add(float64(nBytes))
}

func (me *Metrics) OnWebrtcRtpWrite(ctx *SignalContext, nBytes int) {
	if me == nil {
		return
	}
	me.webrtcWriteBytesTotal.WithLabelValues().Add(float64(nBytes))
}


