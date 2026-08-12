package observability

import (
	"fmt"
	"io"
	"math"
	"sync/atomic"

	"github.com/mmmihaeel/go-authorization-platform/internal/authorization"
)

var latencyBuckets = [...]float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2000}

type histogram struct {
	count           atomic.Uint64
	sumMicroseconds atomic.Uint64
	buckets         [len(latencyBuckets)]atomic.Uint64
}

func (h *histogram) observe(milliseconds float64) {
	if milliseconds < 0 || math.IsNaN(milliseconds) || math.IsInf(milliseconds, 0) {
		return
	}
	h.count.Add(1)
	h.sumMicroseconds.Add(uint64(milliseconds * 1000))
	for index, upperBound := range latencyBuckets {
		if milliseconds <= upperBound {
			h.buckets[index].Add(1)
		}
	}
}

type Metrics struct {
	decisionsTotal     atomic.Uint64
	allowedTotal       atomic.Uint64
	deniedTotal        atomic.Uint64
	errorsTotal        atomic.Uint64
	auditRecordedTotal atomic.Uint64
	auditFailedTotal   atomic.Uint64
	decisionLatency    histogram
	openFGALatency     histogram
	opaLatency         histogram
}

func (m *Metrics) RecordLatency(latency authorization.LatencyMillis) {
	m.decisionLatency.observe(latency.Total)
	m.openFGALatency.observe(latency.OpenFGA)
	m.opaLatency.observe(latency.OPA)
}

func (m *Metrics) RecordAudit(recorded bool) {
	if recorded {
		m.auditRecordedTotal.Add(1)
	} else {
		m.auditFailedTotal.Add(1)
	}
}

func NewMetrics() *Metrics { return &Metrics{} }

func (m *Metrics) RecordDecision(allowed bool, failed bool) {
	m.decisionsTotal.Add(1)
	if failed {
		m.errorsTotal.Add(1)
	}
	if allowed {
		m.allowedTotal.Add(1)
	} else {
		m.deniedTotal.Add(1)
	}
}

func (m *Metrics) WritePrometheus(w io.Writer) error {
	lines := []struct {
		name, help string
		value      uint64
	}{
		{"authz_decisions_total", "Total authorization decisions.", m.decisionsTotal.Load()},
		{"authz_decisions_allowed_total", "Allowed authorization decisions.", m.allowedTotal.Load()},
		{"authz_decisions_denied_total", "Denied authorization decisions.", m.deniedTotal.Load()},
		{"authz_decision_errors_total", "Authorization evaluations with dependency or validation errors.", m.errorsTotal.Load()},
		{"authz_audit_recorded_total", "Authorization decisions atomically persisted with an outbox event.", m.auditRecordedTotal.Load()},
		{"authz_audit_failures_total", "Authorization decisions that were not persisted.", m.auditFailedTotal.Load()},
	}
	for _, line := range lines {
		if _, err := fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n%s %d\n", line.name, line.help, line.name, line.name, line.value); err != nil {
			return err
		}
	}
	for _, metric := range []struct {
		name, help string
		histogram  *histogram
	}{
		{"authz_decision_duration_milliseconds", "End-to-end authorization decision latency in milliseconds.", &m.decisionLatency},
		{"authz_openfga_duration_milliseconds", "OpenFGA check latency in milliseconds.", &m.openFGALatency},
		{"authz_opa_duration_milliseconds", "OPA evaluation latency in milliseconds.", &m.opaLatency},
	} {
		if _, err := fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s histogram\n", metric.name, metric.help, metric.name); err != nil {
			return err
		}
		for index, upperBound := range latencyBuckets {
			if _, err := fmt.Fprintf(w, "%s_bucket{le=\"%g\"} %d\n", metric.name, upperBound, metric.histogram.buckets[index].Load()); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "%s_bucket{le=\"+Inf\"} %d\n%s_sum %g\n%s_count %d\n",
			metric.name, metric.histogram.count.Load(), metric.name, float64(metric.histogram.sumMicroseconds.Load())/1000,
			metric.name, metric.histogram.count.Load()); err != nil {
			return err
		}
	}
	return nil
}
