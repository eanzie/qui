// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package crossseed

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/rs/zerolog"

	"github.com/autobrr/qui/internal/models"
	"github.com/autobrr/qui/internal/services/arr"
)

func TestTriggerArrImport_Sonarr_RescanSeries(t *testing.T) {
	var commandSent string
	var commandParams map[string]any
	var pollCount atomic.Int32

	mux := http.NewServeMux()

	// POST /api/v3/command — send command
	mux.HandleFunc("/api/v3/command", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST to /api/v3/command, got %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		commandSent, _ = body["name"].(string)
		commandParams = body
		json.NewEncoder(w).Encode(arr.CommandResponse{ID: 1, Name: commandSent, Status: "started"})
	})

	// GET /api/v3/command/1 — poll status
	mux.HandleFunc("/api/v3/command/1", func(w http.ResponseWriter, r *http.Request) {
		n := pollCount.Add(1)
		status := "started"
		if n >= 2 {
			status = "completed"
		}
		json.NewEncoder(w).Encode(arr.CommandResponse{ID: 1, Name: "RescanSeries", Status: status})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := arr.NewClient(ts.URL, "testkey", nil, nil, models.ArrInstanceTypeSonarr, 30)
	r := &ArrSeedRunner{}
	l := zerolog.Nop()

	item := &ArrSeedMediaItem{
		ArrInstanceType: "sonarr",
		SeriesID:        42,
	}

	r.triggerArrImport(context.Background(), client, item, &l)

	if commandSent != "RescanSeries" {
		t.Errorf("expected command 'RescanSeries', got %q", commandSent)
	}
	seriesID, _ := commandParams["seriesId"].(float64)
	if int(seriesID) != 42 {
		t.Errorf("expected seriesId=42, got %v", commandParams["seriesId"])
	}
	if pollCount.Load() < 2 {
		t.Errorf("expected at least 2 poll calls, got %d", pollCount.Load())
	}
}

func TestTriggerArrImport_Radarr_RescanMovie(t *testing.T) {
	var commandSent string
	var commandParams map[string]any

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/command", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		commandSent, _ = body["name"].(string)
		commandParams = body
		json.NewEncoder(w).Encode(arr.CommandResponse{ID: 2, Name: commandSent, Status: "started"})
	})
	mux.HandleFunc("/api/v3/command/2", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(arr.CommandResponse{ID: 2, Name: "RescanMovie", Status: "completed"})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := arr.NewClient(ts.URL, "testkey", nil, nil, models.ArrInstanceTypeRadarr, 30)
	r := &ArrSeedRunner{}
	l := zerolog.Nop()

	item := &ArrSeedMediaItem{
		ArrInstanceType: "radarr",
		MovieID:         99,
	}

	r.triggerArrImport(context.Background(), client, item, &l)

	if commandSent != "RescanMovie" {
		t.Errorf("expected command 'RescanMovie', got %q", commandSent)
	}
	movieID, _ := commandParams["movieId"].(float64)
	if int(movieID) != 99 {
		t.Errorf("expected movieId=99, got %v", commandParams["movieId"])
	}
}

func TestTriggerArrImport_CommandFails_ReturnsWithoutError(t *testing.T) {
	var pollCount atomic.Int32

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/command", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(arr.CommandResponse{ID: 3, Name: "RescanSeries", Status: "started"})
	})
	mux.HandleFunc("/api/v3/command/3", func(w http.ResponseWriter, r *http.Request) {
		n := pollCount.Add(1)
		status := "started"
		if n >= 2 {
			status = "failed"
		}
		json.NewEncoder(w).Encode(arr.CommandResponse{ID: 3, Name: "RescanSeries", Status: status})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := arr.NewClient(ts.URL, "testkey", nil, nil, models.ArrInstanceTypeSonarr, 30)
	r := &ArrSeedRunner{}
	l := zerolog.Nop()

	item := &ArrSeedMediaItem{
		ArrInstanceType: "sonarr",
		SeriesID:        1,
	}

	// Should return without panicking; the method logs but does not return an error.
	r.triggerArrImport(context.Background(), client, item, &l)

	if pollCount.Load() < 2 {
		t.Errorf("expected at least 2 poll calls before failure, got %d", pollCount.Load())
	}
}

func TestTriggerArrImport_UnknownArrType_ReturnsEarly(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		fmt.Fprintln(w, "{}")
	}))
	defer ts.Close()

	client := arr.NewClient(ts.URL, "testkey", nil, nil, "lidarr", 30)
	r := &ArrSeedRunner{}
	l := zerolog.Nop()

	item := &ArrSeedMediaItem{
		ArrInstanceType: "lidarr",
	}

	r.triggerArrImport(context.Background(), client, item, &l)

	if callCount != 0 {
		t.Errorf("expected no HTTP calls for unknown arr type, got %d", callCount)
	}
}
