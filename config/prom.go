package config

import "github.com/prometheus/client_golang/prometheus"

type PrometheusConfig struct {
	reg *prometheus.Registry
}

func NewPrometheusConfig() *PrometheusConfig {
	return &PrometheusConfig{
		reg: prometheus.NewRegistry(),
	}
}

func (me *PrometheusConfig) Registry() *prometheus.Registry {
	return me.reg
}

var PROM_CONF = NewPrometheusConfig()

func Prom() *PrometheusConfig {
	return PROM_CONF
}