package observability

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type Metrics struct {
	requestCount    atomic.Int64
	errorCount      atomic.Int64
	messageCount    atomic.Int64
	broadcastCount  atomic.Int64
	activeConns     atomic.Int64
	latencySum      atomic.Int64
	latencyCount    atomic.Int64

	histogram   map[string]*atomic.Int64
	histogramMu sync.RWMutex

	startTime time.Time
}

func NewMetrics() *Metrics {
	return &Metrics{
		histogram: make(map[string]*atomic.Int64),
		startTime: time.Now(),
	}
}

func (m *Metrics) IncRequests()    { m.requestCount.Add(1) }
func (m *Metrics) IncErrors()      { m.errorCount.Add(1) }
func (m *Metrics) IncMessages()    { m.messageCount.Add(1) }
func (m *Metrics) IncBroadcasts()  { m.broadcastCount.Add(1) }
func (m *Metrics) IncConns()       { m.activeConns.Add(1) }
func (m *Metrics) DecConns()       { m.activeConns.Add(-1) }

func (m *Metrics) RecordLatency(d time.Duration) {
	m.latencySum.Add(d.Milliseconds())
	m.latencyCount.Add(1)
}

func (m *Metrics) IncCounter(name string) {
	m.histogramMu.RLock()
	c, ok := m.histogram[name]
	m.histogramMu.RUnlock()

	if !ok {
		m.histogramMu.Lock()
		c, ok = m.histogram[name]
		if !ok {
			c = &atomic.Int64{}
			m.histogram[name] = c
		}
		m.histogramMu.Unlock()
	}
	c.Add(1)
}

func (m *Metrics) Snapshot() map[string]interface{} {
	avgLatency := float64(0)
	if count := m.latencyCount.Load(); count > 0 {
		avgLatency = float64(m.latencySum.Load()) / float64(count)
	}

	counters := make(map[string]int64)
	m.histogramMu.RLock()
	for k, v := range m.histogram {
		counters[k] = v.Load()
	}
	m.histogramMu.RUnlock()

	return map[string]interface{}{
		"uptime_seconds":     time.Since(m.startTime).Seconds(),
		"total_requests":     m.requestCount.Load(),
		"total_errors":       m.errorCount.Load(),
		"total_messages":     m.messageCount.Load(),
		"total_broadcasts":   m.broadcastCount.Load(),
		"active_connections": m.activeConns.Load(),
		"avg_latency_ms":     avgLatency,
		"counters":           counters,
	}
}

// PrometheusHandler serves metrics in Prometheus text exposition format.
func (m *Metrics) PrometheusHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")

		avgLatency := float64(0)
		if count := m.latencyCount.Load(); count > 0 {
			avgLatency = float64(m.latencySum.Load()) / float64(count)
		}

		fmt.Fprintf(w, "# HELP gateway_uptime_seconds Time since gateway started\n")
		fmt.Fprintf(w, "# TYPE gateway_uptime_seconds gauge\n")
		fmt.Fprintf(w, "gateway_uptime_seconds %f\n", time.Since(m.startTime).Seconds())

		fmt.Fprintf(w, "# HELP gateway_requests_total Total HTTP requests\n")
		fmt.Fprintf(w, "# TYPE gateway_requests_total counter\n")
		fmt.Fprintf(w, "gateway_requests_total %d\n", m.requestCount.Load())

		fmt.Fprintf(w, "# HELP gateway_errors_total Total errors\n")
		fmt.Fprintf(w, "# TYPE gateway_errors_total counter\n")
		fmt.Fprintf(w, "gateway_errors_total %d\n", m.errorCount.Load())

		fmt.Fprintf(w, "# HELP gateway_messages_total Total messages routed\n")
		fmt.Fprintf(w, "# TYPE gateway_messages_total counter\n")
		fmt.Fprintf(w, "gateway_messages_total %d\n", m.messageCount.Load())

		fmt.Fprintf(w, "# HELP gateway_broadcasts_total Total broadcasts\n")
		fmt.Fprintf(w, "# TYPE gateway_broadcasts_total counter\n")
		fmt.Fprintf(w, "gateway_broadcasts_total %d\n", m.broadcastCount.Load())

		fmt.Fprintf(w, "# HELP gateway_active_connections Current active connections\n")
		fmt.Fprintf(w, "# TYPE gateway_active_connections gauge\n")
		fmt.Fprintf(w, "gateway_active_connections %d\n", m.activeConns.Load())

		fmt.Fprintf(w, "# HELP gateway_avg_latency_ms Average request latency in ms\n")
		fmt.Fprintf(w, "# TYPE gateway_avg_latency_ms gauge\n")
		fmt.Fprintf(w, "gateway_avg_latency_ms %f\n", avgLatency)

		m.histogramMu.RLock()
		for k, v := range m.histogram {
			fmt.Fprintf(w, "gateway_counter_%s %d\n", k, v.Load())
		}
		m.histogramMu.RUnlock()
	}
}

// JSONHandler serves metrics in JSON format.
func (m *Metrics) JSONHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(m.Snapshot())
	}
}

// MetricsMiddleware records request counts and latency.
func (m *Metrics) MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		m.IncRequests()
		next.ServeHTTP(w, r)
		m.RecordLatency(time.Since(start))
	})
}
