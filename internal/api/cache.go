package api

import (
	"context"
	"sync"
	"time"

	"github.com/gscarneiro/eafc-bot/internal/store"
)

type snapshotCache struct {
	mu          sync.Mutex
	snapshot    store.Snapshot
	loadedAt    time.Time
	lastSuccess time.Time
	valid       bool
}

func (s *Server) loadSnapshot(ctx context.Context) (store.Snapshot, bool, error) {
	if s.CacheTTL > 0 {
		status := JobStatus{}
		if s.Status != nil {
			status = s.Status()
		}
		s.cache.mu.Lock()
		valid := s.cache.valid && time.Since(s.cache.loadedAt) < s.CacheTTL
		if valid && status.LastSuccess != nil && !status.LastSuccess.Equal(s.cache.lastSuccess) {
			valid = false
		}
		if valid {
			snapshot := s.cache.snapshot
			s.cache.mu.Unlock()
			return snapshot, true, nil
		}
		s.cache.mu.Unlock()
	}

	snapshot, ok, err := s.Store.LatestSnapshot(ctx, s.Cycle)
	if err != nil || !ok || s.CacheTTL <= 0 {
		return snapshot, ok, err
	}
	lastSuccess := time.Time{}
	if s.Status != nil {
		if status := s.Status(); status.LastSuccess != nil {
			lastSuccess = *status.LastSuccess
		}
	}
	s.cache.mu.Lock()
	s.cache.snapshot = snapshot
	s.cache.loadedAt = time.Now()
	s.cache.lastSuccess = lastSuccess
	s.cache.valid = true
	s.cache.mu.Unlock()
	return snapshot, ok, nil
}
