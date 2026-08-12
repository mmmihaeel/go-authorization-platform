package observability

import (
	"fmt"
	"io"
	"sync/atomic"
)

type Metrics struct {
	decisionsTotal atomic.Uint64
	allowedTotal   atomic.Uint64
	deniedTotal    atomic.Uint64
	errorsTotal    atomic.Uint64
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
	}
	for _, line := range lines {
		if _, err := fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n%s %d\n", line.name, line.help, line.name, line.name, line.value); err != nil {
			return err
		}
	}
	return nil
}
