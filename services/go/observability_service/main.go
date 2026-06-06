package main

import (
	"fmt"
	"sync"
	"time"
)

// Observability Service - Prometheus metrics, Grafana, OpenTelemetry, Jaeger

type Metric struct {
	Name      string
	Value    float64
	Labels   map[string]string
	Type     string // counter, gauge, histogram
	Time     time.Time
}

type Metrics struct {
	mu      sync.RWMutex
	metrics map[string]map[string]Metric  // name -> labels -> metric
}

func NewMetrics() *Metrics {
	return &Metrics{
		metrics: make(map[string]map[string]Metric),
	}
}

func (m *Metrics) IncCounter(name string, value float64, labels map[string]string) {
	m.set(name, value, "counter", labels)
}

func (m *Metrics) SetGauge(name string, value float64, labels map[string]string) {
	m.set(name, value, "gauge", labels)
}

func (m *Metrics) ObserveHistogram(name string, value float64, labels map[string]string) {
	m.set(name, value, "histogram", labels)
}

func (m *Metrics) set(name string, value float64, metricType string, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	labelKey := ""
	for k, v := range labels {
		labelKey += k + "=" + v + ","
	}
	
	if _, ok := m.metrics[name]; !ok {
		m.metrics[name] = make(map[string]Metric)
	}
	
	m.metrics[name][labelKey] = Metric{
		Name:   name,
		Value:  value,
		Labels: labels,
		Type:   metricType,
		Time:  time.Now(),
	}
}

func (m *Metrics) GetPrometheusFormat() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	var output string
	for name, metrics := range m.metrics {
		for _, metric := range metrics {
			labels := ""
			for k, v := range metric.Labels {
				labels += k + `="` + v + `",`
			}
			output += fmt.Sprintf("%s{%s} %f\n", name, labels, metric.Value)
		}
	}
	return output
}

func (m *Metrics) ExportToOpenTelemetry() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	spans := make([]map[string]interface{}, 0)
	for _, metrics := range m.metrics {
		for _, metric := range metrics {
			spans = append(spans, map[string]interface{}{
				"name":      metric.Name,
				"value":    metric.Value,
				"type":     metric.Type,
				"labels":   metric.Labels,
				"timestamp": metric.Time.Unix(),
			})
		}
	}
	return map[string]interface{}{
		"spans": spans,
	}
}

func main() {
	metrics := NewMetrics()
	
	// Record metrics
	metrics.IncCounter("requests_total", 1, map[string]string{"method": "GET", "status": "200"})
	metrics.IncCounter("requests_total", 1, map[string]string{"method": "POST", "status": "201"})
	metrics.SetGauge("active_connections", 100, map[string]string{})
	metrics.ObserveHistogram("request_duration_seconds", 0.025, map[string]string{"method": "GET"})
	
	// Export
	prometheus := metrics.GetPrometheusFormat()
	fmt.Println(prometheus)
	
	otlp := metrics.ExportToOpenTelemetry()
	fmt.Printf("OTLP: %+v\n", otlp)
}