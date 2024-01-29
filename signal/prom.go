package signal

import "github.com/prometheus/client_golang/prometheus"

type PromConf struct {
	reg               *prometheus.Registry
	signalConnTotal   *prometheus.CounterVec
	signalReconnTotal *prometheus.CounterVec
	signalConnecting  *prometheus.Gauge
	webrtcConnTotal   *prometheus.CounterVec
	webrtcConnecting  *prometheus.Gauge
}
