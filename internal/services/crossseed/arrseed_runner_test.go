// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package crossseed

import (
	"testing"
	"time"

	"github.com/autobrr/qui/internal/models"
)

// --- isDueForScan ---

func TestIsDueForScan_NilLastScan(t *testing.T) {
	r := &ArrSeedRunner{}
	config := &models.ArrSeedInstanceConfig{
		ScanIntervalMinutes: 1440,
		LastScanAt:          nil,
	}
	if !r.isDueForScan(config) {
		t.Error("expected due when LastScanAt is nil (never scanned)")
	}
}

func TestIsDueForScan_IntervalElapsed(t *testing.T) {
	r := &ArrSeedRunner{}
	past := time.Now().Add(-2 * time.Hour)
	config := &models.ArrSeedInstanceConfig{
		ScanIntervalMinutes: 60, // 1 hour interval
		LastScanAt:          &past,
	}
	if !r.isDueForScan(config) {
		t.Error("expected due when interval has elapsed")
	}
}

func TestIsDueForScan_IntervalNotElapsed(t *testing.T) {
	r := &ArrSeedRunner{}
	recent := time.Now().Add(-5 * time.Minute)
	config := &models.ArrSeedInstanceConfig{
		ScanIntervalMinutes: 60,
		LastScanAt:          &recent,
	}
	if r.isDueForScan(config) {
		t.Error("expected not due when interval hasn't elapsed")
	}
}

func TestIsDueForScan_ZeroInterval_AlwaysDue(t *testing.T) {
	// BUG EXPOSURE: With ScanIntervalMinutes=0, nextScan == LastScanAt,
	// so time.Now().After(lastScanAt) is always true, causing scans every minute.
	r := &ArrSeedRunner{}
	recent := time.Now().Add(-1 * time.Second)
	config := &models.ArrSeedInstanceConfig{
		ScanIntervalMinutes: 0,
		LastScanAt:          &recent,
	}
	if !r.isDueForScan(config) {
		t.Error("zero interval makes scan always due (potential bug: may cause scan on every scheduler tick)")
	}
}

func TestIsDueForScan_ExactBoundary(t *testing.T) {
	r := &ArrSeedRunner{}
	// Set last scan exactly the interval ago, minus a tiny margin
	past := time.Now().Add(-60*time.Minute - 1*time.Second)
	config := &models.ArrSeedInstanceConfig{
		ScanIntervalMinutes: 60,
		LastScanAt:          &past,
	}
	if !r.isDueForScan(config) {
		t.Error("expected due when just past interval boundary")
	}
}
