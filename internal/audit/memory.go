package audit

import (
	"context"
	"sync"

	"github.com/mmmihaeel/go-authorization-platform/internal/authorization"
)

type MemoryRecorder struct {
	mu      sync.RWMutex
	records []authorization.AuditRecord
}

func NewMemoryRecorder() *MemoryRecorder { return &MemoryRecorder{} }

func (r *MemoryRecorder) RecordDecision(_ context.Context, record authorization.AuditRecord) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.records = append(r.records, record)
	return nil
}

func (r *MemoryRecorder) Records() []authorization.AuditRecord {
	r.mu.RLock()
	defer r.mu.RUnlock()
	copied := make([]authorization.AuditRecord, len(r.records))
	copy(copied, r.records)
	return copied
}
