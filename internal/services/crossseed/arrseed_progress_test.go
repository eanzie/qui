// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package crossseed

import "testing"

func TestArrSeedProgressTracker_SetAndGet(t *testing.T) {
	tracker := newArrSeedProgressTracker()

	p := &ArrSeedScanProgress{RunID: 1, ConfigID: 10, Status: "running", Phase: "scanning"}
	tracker.set(1, p)

	got := tracker.get(1)
	if got == nil {
		t.Fatal("expected progress, got nil")
	}
	if got.ConfigID != 10 {
		t.Errorf("ConfigID: got %d, want 10", got.ConfigID)
	}
	if got.Status != "running" {
		t.Errorf("Status: got %q, want %q", got.Status, "running")
	}
}

func TestArrSeedProgressTracker_GetReturnsNilForMissing(t *testing.T) {
	tracker := newArrSeedProgressTracker()
	if got := tracker.get(999); got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestArrSeedProgressTracker_GetReturnsCopy(t *testing.T) {
	tracker := newArrSeedProgressTracker()
	p := &ArrSeedScanProgress{RunID: 1, ConfigID: 10, MatchesFound: 5}
	tracker.set(1, p)

	got := tracker.get(1)
	got.MatchesFound = 99

	original := tracker.get(1)
	if original.MatchesFound != 5 {
		t.Fatalf("get should return a copy, but original was modified to %d", original.MatchesFound)
	}
}

func TestArrSeedProgressTracker_Clear(t *testing.T) {
	tracker := newArrSeedProgressTracker()
	tracker.set(1, &ArrSeedScanProgress{RunID: 1, ConfigID: 10})
	tracker.clear(1)

	if got := tracker.get(1); got != nil {
		t.Fatalf("expected nil after clear, got %+v", got)
	}
}

func TestArrSeedProgressTracker_GetByConfigID(t *testing.T) {
	tracker := newArrSeedProgressTracker()
	tracker.set(1, &ArrSeedScanProgress{RunID: 1, ConfigID: 10, Phase: "scanning"})
	tracker.set(2, &ArrSeedScanProgress{RunID: 2, ConfigID: 20, Phase: "searching"})

	got := tracker.getByConfigID(20)
	if got == nil {
		t.Fatal("expected progress for configID 20")
	}
	if got.RunID != 2 {
		t.Errorf("RunID: got %d, want 2", got.RunID)
	}
	if got.Phase != "searching" {
		t.Errorf("Phase: got %q, want %q", got.Phase, "searching")
	}
}

func TestArrSeedProgressTracker_GetByConfigID_NotFound(t *testing.T) {
	tracker := newArrSeedProgressTracker()
	tracker.set(1, &ArrSeedScanProgress{RunID: 1, ConfigID: 10})

	if got := tracker.getByConfigID(999); got != nil {
		t.Fatalf("expected nil for missing configID, got %+v", got)
	}
}

func TestArrSeedProgressTracker_GetByConfigID_ReturnsCopy(t *testing.T) {
	tracker := newArrSeedProgressTracker()
	tracker.set(1, &ArrSeedScanProgress{RunID: 1, ConfigID: 10, TorrentsAdded: 3})

	got := tracker.getByConfigID(10)
	got.TorrentsAdded = 99

	original := tracker.getByConfigID(10)
	if original.TorrentsAdded != 3 {
		t.Fatalf("getByConfigID should return a copy, but original was modified to %d", original.TorrentsAdded)
	}
}

func TestArrSeedProgressTracker_MultipleRuns(t *testing.T) {
	tracker := newArrSeedProgressTracker()
	tracker.set(1, &ArrSeedScanProgress{RunID: 1, ConfigID: 10, ItemsProcessed: 5})
	tracker.set(2, &ArrSeedScanProgress{RunID: 2, ConfigID: 10, ItemsProcessed: 10})

	// Both should be retrievable by runID
	got1 := tracker.get(1)
	got2 := tracker.get(2)
	if got1 == nil || got2 == nil {
		t.Fatal("expected both runs to be retrievable")
	}
	if got1.ItemsProcessed != 5 {
		t.Errorf("run 1 items: got %d, want 5", got1.ItemsProcessed)
	}
	if got2.ItemsProcessed != 10 {
		t.Errorf("run 2 items: got %d, want 10", got2.ItemsProcessed)
	}
}
