// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package crossseed

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"

	"github.com/autobrr/qui/internal/models"
	"github.com/autobrr/qui/internal/services/arr"
)

// --- resolveOrCreateTag ---

func TestResolveOrCreateTag_FindsExisting(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/tag", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		json.NewEncoder(w).Encode([]arr.TagResponse{
			{ID: 5, Label: "cross-seeded"},
			{ID: 10, Label: "other"},
		})
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := arr.NewClient(ts.URL, "testkey", nil, nil, models.ArrInstanceTypeSonarr, 30)
	e := &arrSeedPostSeedExecutor{}
	l := zerolog.Nop()

	tagID, err := e.resolveOrCreateTag(context.Background(), client, "cross-seeded", &l)
	if err != nil {
		t.Fatalf("resolveOrCreateTag: %v", err)
	}
	if tagID != 5 {
		t.Errorf("expected tagID=5, got %d", tagID)
	}
}

func TestResolveOrCreateTag_CreatesNew(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/tag", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode([]arr.TagResponse{
				{ID: 1, Label: "existing"},
			})
		case http.MethodPost:
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Label string `json:"label"`
			}
			json.Unmarshal(body, &req)
			if req.Label != "new-tag" {
				t.Errorf("expected label 'new-tag', got %q", req.Label)
			}
			json.NewEncoder(w).Encode(arr.TagResponse{ID: 42, Label: "new-tag"})
		default:
			t.Errorf("unexpected method %s", r.Method)
		}
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := arr.NewClient(ts.URL, "testkey", nil, nil, models.ArrInstanceTypeSonarr, 30)
	e := &arrSeedPostSeedExecutor{}
	l := zerolog.Nop()

	tagID, err := e.resolveOrCreateTag(context.Background(), client, "new-tag", &l)
	if err != nil {
		t.Fatalf("resolveOrCreateTag: %v", err)
	}
	if tagID != 42 {
		t.Errorf("expected tagID=42, got %d", tagID)
	}
}

// --- execute routing ---

func TestExecute_NoActionsConfigured_NoHTTPCalls(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := arr.NewClient(ts.URL, "testkey", nil, nil, models.ArrInstanceTypeSonarr, 30)
	e := &arrSeedPostSeedExecutor{}
	l := zerolog.Nop()

	config := &models.ArrSeedInstanceConfig{
		UnmonitorAfterSeed: false,
		TagAfterSeed:       "",
	}
	item := &ArrSeedMediaItem{
		ArrInstanceType: "sonarr",
		SeriesID:        1,
	}

	e.execute(context.Background(), client, item, config, &l)

	if callCount != 0 {
		t.Errorf("expected no HTTP calls when no actions configured, got %d", callCount)
	}
}

func TestExecute_SonarrInvalidSeriesID_NoHTTPCalls(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := arr.NewClient(ts.URL, "testkey", nil, nil, models.ArrInstanceTypeSonarr, 30)
	e := &arrSeedPostSeedExecutor{}
	l := zerolog.Nop()

	config := &models.ArrSeedInstanceConfig{
		UnmonitorAfterSeed: true,
		TagAfterSeed:       "seeded",
	}
	item := &ArrSeedMediaItem{
		ArrInstanceType: "sonarr",
		SeriesID:        0,
	}

	e.execute(context.Background(), client, item, config, &l)

	if callCount != 0 {
		t.Errorf("expected no HTTP calls for invalid seriesID, got %d", callCount)
	}
}

func TestExecute_RadarrInvalidMovieID_NoHTTPCalls(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := arr.NewClient(ts.URL, "testkey", nil, nil, models.ArrInstanceTypeRadarr, 30)
	e := &arrSeedPostSeedExecutor{}
	l := zerolog.Nop()

	config := &models.ArrSeedInstanceConfig{
		UnmonitorAfterSeed: true,
	}
	item := &ArrSeedMediaItem{
		ArrInstanceType: "radarr",
		MovieID:         -1,
	}

	e.execute(context.Background(), client, item, config, &l)

	if callCount != 0 {
		t.Errorf("expected no HTTP calls for invalid movieID, got %d", callCount)
	}
}

// --- executeRadarr ---

func TestExecuteRadarr_UnmonitorMovie(t *testing.T) {
	var putBody map[string]any

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/movie/10", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// UpdateMovieFields first fetches the movie.
			json.NewEncoder(w).Encode(map[string]any{
				"id":        10,
				"title":     "Test Movie",
				"monitored": true,
				"tags":      []int{},
			})
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &putBody)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := arr.NewClient(ts.URL, "testkey", nil, nil, models.ArrInstanceTypeRadarr, 30)
	e := &arrSeedPostSeedExecutor{}
	l := zerolog.Nop()

	config := &models.ArrSeedInstanceConfig{
		UnmonitorAfterSeed: true,
		TagAfterSeed:       "",
	}
	item := &ArrSeedMediaItem{
		ArrInstanceType: "radarr",
		MovieID:         10,
	}

	e.executeRadarr(context.Background(), client, item, config, &l)

	if putBody == nil {
		t.Fatal("expected PUT request to movie endpoint")
	}
	monitored, ok := putBody["monitored"]
	if !ok {
		t.Fatal("expected 'monitored' field in PUT body")
	}
	if monitored != false {
		t.Errorf("expected monitored=false, got %v", monitored)
	}
}

func TestExecuteRadarr_TagMovie(t *testing.T) {
	var finalPutBody map[string]any

	mux := http.NewServeMux()

	// GET tags
	mux.HandleFunc("/api/v3/tag", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// Tag does not exist yet.
			json.NewEncoder(w).Encode([]arr.TagResponse{})
		case http.MethodPost:
			json.NewEncoder(w).Encode(arr.TagResponse{ID: 7, Label: "cross-seeded"})
		}
	})

	// GET movie (returns existing tags)
	mux.HandleFunc("/api/v3/movie/10", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(map[string]any{
				"id":    10,
				"title": "Test Movie",
				"tags":  []int{3},
			})
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &finalPutBody)
			w.WriteHeader(http.StatusOK)
		}
	})

	ts := httptest.NewServer(mux)
	defer ts.Close()

	client := arr.NewClient(ts.URL, "testkey", nil, nil, models.ArrInstanceTypeRadarr, 30)
	e := &arrSeedPostSeedExecutor{}
	l := zerolog.Nop()

	config := &models.ArrSeedInstanceConfig{
		UnmonitorAfterSeed: false,
		TagAfterSeed:       "cross-seeded",
	}
	item := &ArrSeedMediaItem{
		ArrInstanceType: "radarr",
		MovieID:         10,
	}

	e.executeRadarr(context.Background(), client, item, config, &l)

	if finalPutBody == nil {
		t.Fatal("expected PUT request to movie endpoint")
	}

	tagsRaw, ok := finalPutBody["tags"]
	if !ok {
		t.Fatal("expected 'tags' field in PUT body")
	}
	tags, ok := tagsRaw.([]any)
	if !ok {
		t.Fatalf("expected tags to be []any, got %T", tagsRaw)
	}
	// Should contain both existing tag 3 and new tag 7.
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d: %v", len(tags), tags)
	}

	found3, found7 := false, false
	for _, v := range tags {
		n, _ := v.(float64)
		if int(n) == 3 {
			found3 = true
		}
		if int(n) == 7 {
			found7 = true
		}
	}
	if !found3 || !found7 {
		t.Errorf("expected tags [3, 7], got %v", tags)
	}
}
