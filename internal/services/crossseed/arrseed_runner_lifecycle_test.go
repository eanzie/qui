// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package crossseed

import (
	"context"
	"path/filepath"
	"sync"
	"testing"

	"github.com/autobrr/qui/internal/database"
	"github.com/autobrr/qui/internal/models"
)

// --- getConfigMutex ---

func TestGetConfigMutex_SameIDReturnsSameMutex(t *testing.T) {
	r := &ArrSeedRunner{
		configMu: make(map[int]*sync.Mutex),
	}

	m1 := r.getConfigMutex(42)
	m2 := r.getConfigMutex(42)

	if m1 != m2 {
		t.Error("expected same mutex for same configID")
	}
}

func TestGetConfigMutex_DifferentIDReturnsDifferentMutex(t *testing.T) {
	r := &ArrSeedRunner{
		configMu: make(map[int]*sync.Mutex),
	}

	m1 := r.getConfigMutex(1)
	m2 := r.getConfigMutex(2)

	if m1 == m2 {
		t.Error("expected different mutexes for different configIDs")
	}
}

// --- isStopped ---

func TestIsStopped_FalseWhenNoFlag(t *testing.T) {
	r := &ArrSeedRunner{
		stopFlags: make(map[int64]struct{}),
	}

	if r.isStopped(99) {
		t.Error("expected isStopped=false when no flag set")
	}
}

func TestIsStopped_TrueAfterFlagSet(t *testing.T) {
	r := &ArrSeedRunner{
		stopFlags: make(map[int64]struct{}),
	}

	r.cancelMu.Lock()
	r.stopFlags[99] = struct{}{}
	r.cancelMu.Unlock()

	if !r.isStopped(99) {
		t.Error("expected isStopped=true after setting flag")
	}
}

// --- stopScan / cancelScan with real DB ---

func newTestRunnerWithDB(t *testing.T) (*ArrSeedRunner, *models.ArrSeedStore, *models.ArrInstanceStore) {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "lifecycle_test.db")
	db, err := database.New(dbPath)
	if err != nil {
		t.Fatalf("database.New: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	store := models.NewArrSeedStore(db)
	arrStore, err := models.NewArrInstanceStore(db, []byte("01234567890123456789012345678901"))
	if err != nil {
		t.Fatalf("NewArrInstanceStore: %v", err)
	}

	r := &ArrSeedRunner{
		store:       store,
		cancelFuncs: make(map[int64]context.CancelFunc),
		stopFlags:   make(map[int64]struct{}),
		progress:    newArrSeedProgressTracker(),
		configMu:    make(map[int]*sync.Mutex),
	}

	return r, store, arrStore
}

func createTestConfig(t *testing.T, ctx context.Context, store *models.ArrSeedStore, arrStore *models.ArrInstanceStore) *models.ArrSeedInstanceConfig {
	t.Helper()

	inst, err := arrStore.Create(ctx, models.ArrInstanceTypeSonarr, "test-sonarr", "http://localhost:8989", "key123", nil, nil, true, 1, 15)
	if err != nil {
		t.Fatalf("arrStore.Create: %v", err)
	}

	cfg, err := store.CreateConfig(ctx, &models.ArrSeedInstanceConfig{
		ArrInstanceID:       inst.ID,
		Enabled:             true,
		TargetQbitInstanceID: 0,
		Category:            "tv",
		ScanIntervalMinutes: 1440,
	})
	if err != nil {
		t.Fatalf("store.CreateConfig: %v", err)
	}

	return cfg
}

func TestStopScan_SetsStopFlag(t *testing.T) {
	ctx := context.Background()
	r, store, arrStore := newTestRunnerWithDB(t)
	cfg := createTestConfig(t, ctx, store, arrStore)

	runID, err := store.CreateRunIfNoActive(ctx, cfg.ID, "manual")
	if err != nil {
		t.Fatalf("CreateRunIfNoActive: %v", err)
	}

	// Register a cancel func so the run is "active" in memory.
	_, cancel := context.WithCancel(ctx)
	defer cancel()
	r.cancelFuncs[runID] = cancel

	// Set progress so stopScan can update status.
	r.progress.set(runID, &ArrSeedScanProgress{
		RunID:    runID,
		ConfigID: cfg.ID,
		Status:   "running",
	})

	if err := r.stopScan(ctx, cfg.ID); err != nil {
		t.Fatalf("stopScan: %v", err)
	}

	// Verify stop flag is set.
	if !r.isStopped(runID) {
		t.Error("expected stop flag to be set after stopScan")
	}

	// Verify progress status updated to "stopping".
	p := r.progress.get(runID)
	if p == nil {
		t.Fatal("expected progress to still exist")
	}
	if p.Status != "stopping" {
		t.Errorf("expected progress status 'stopping', got %q", p.Status)
	}
}

func TestCancelScan_CancelsFuncAndUpdatesDB(t *testing.T) {
	ctx := context.Background()
	r, store, arrStore := newTestRunnerWithDB(t)
	cfg := createTestConfig(t, ctx, store, arrStore)

	runID, err := store.CreateRunIfNoActive(ctx, cfg.ID, "manual")
	if err != nil {
		t.Fatalf("CreateRunIfNoActive: %v", err)
	}

	// Register a cancel func.
	cancelCtx, cancel := context.WithCancel(ctx)
	r.cancelFuncs[runID] = cancel
	r.progress.set(runID, &ArrSeedScanProgress{
		RunID:    runID,
		ConfigID: cfg.ID,
		Status:   "running",
	})

	if err := r.cancelScan(ctx, cfg.ID); err != nil {
		t.Fatalf("cancelScan: %v", err)
	}

	// Verify the context was cancelled.
	if cancelCtx.Err() == nil {
		t.Error("expected cancel context to be done after cancelScan")
	}

	// Verify progress was cleared.
	if p := r.progress.get(runID); p != nil {
		t.Error("expected progress to be cleared after cancelScan")
	}

	// Verify run status in DB is "cancelled".
	run, err := store.GetActiveRun(ctx, cfg.ID)
	if err != nil {
		t.Fatalf("GetActiveRun: %v", err)
	}
	// After cancellation, GetActiveRun should return nil (no active run).
	if run != nil {
		t.Errorf("expected no active run after cancel, got status %q", run.Status)
	}
}

func TestStopScan_NoActiveRun_ReturnsNil(t *testing.T) {
	ctx := context.Background()
	r, store, arrStore := newTestRunnerWithDB(t)
	cfg := createTestConfig(t, ctx, store, arrStore)

	// No run created — stopScan should return nil gracefully.
	if err := r.stopScan(ctx, cfg.ID); err != nil {
		t.Fatalf("expected nil error for no active run, got: %v", err)
	}
}

func TestCancelScan_NoActiveRun_ReturnsNil(t *testing.T) {
	ctx := context.Background()
	r, store, arrStore := newTestRunnerWithDB(t)
	cfg := createTestConfig(t, ctx, store, arrStore)

	if err := r.cancelScan(ctx, cfg.ID); err != nil {
		t.Fatalf("expected nil error for no active run, got: %v", err)
	}
}
