// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package crossseed

import "sync"

// arrSeedProgressTracker holds in-memory progress state for running scans.
type arrSeedProgressTracker struct {
	mu       sync.Mutex
	progress map[int64]*ArrSeedScanProgress // keyed by runID
}

func newArrSeedProgressTracker() *arrSeedProgressTracker {
	return &arrSeedProgressTracker{
		progress: make(map[int64]*ArrSeedScanProgress),
	}
}

func (t *arrSeedProgressTracker) set(runID int64, p *ArrSeedScanProgress) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.progress[runID] = p
}

func (t *arrSeedProgressTracker) get(runID int64) *ArrSeedScanProgress {
	t.mu.Lock()
	defer t.mu.Unlock()
	if p, ok := t.progress[runID]; ok {
		cp := *p
		return &cp
	}
	return nil
}

func (t *arrSeedProgressTracker) clear(runID int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.progress, runID)
}

// getByConfigID returns progress for the first running scan matching a config ID.
func (t *arrSeedProgressTracker) getByConfigID(configID int) *ArrSeedScanProgress {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, p := range t.progress {
		if p.ConfigID == configID {
			cp := *p
			return &cp
		}
	}
	return nil
}
