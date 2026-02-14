// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package crossseed

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"

	"github.com/autobrr/qui/internal/models"
	"github.com/autobrr/qui/internal/services/arr"
)

// --- Sonarr path filtering ---

func TestArrSeedScanSonarrInstance_SkipsSeriesOutsideDockerPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/series", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]arr.SonarrSeriesResponse{
			{ID: 1, Title: "Inside", Path: "/tv/Inside"},
			{ID: 2, Title: "Outside", Path: "/movies/Outside"}, // Different root
		})
	})
	mux.HandleFunc("/api/v3/episodefile", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]arr.SonarrEpisodeFileResponse{
			{ID: 100, SeriesID: 1, SeasonNumber: 1, Path: "/tv/Inside/ep.mkv", Size: 1000, SceneName: "Inside.S01E01-GRP"},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := arr.NewClient(srv.URL, "key", nil, nil, "sonarr", 30)
	config := &models.ArrSeedInstanceConfig{
		ID:            1,
		ArrDockerPath: "/tv",
	}
	scanner := &arrSeedScanner{}
	l := zerolog.Nop()

	items, err := scanner.scanSonarrInstance(context.Background(), client, config, &l)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only "Inside" should be scanned
	if len(items) != 1 {
		t.Fatalf("expected 1 item (series outside path skipped), got %d", len(items))
	}
	if items[0].Title != "Inside" {
		t.Errorf("expected title 'Inside', got %q", items[0].Title)
	}
}

func TestArrSeedScanSonarrInstance_EmptyDockerPathScansAll(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/series", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]arr.SonarrSeriesResponse{
			{ID: 1, Title: "Show A", Path: "/tv/ShowA"},
			{ID: 2, Title: "Show B", Path: "/movies/ShowB"},
		})
	})
	mux.HandleFunc("/api/v3/episodefile", func(w http.ResponseWriter, r *http.Request) {
		seriesID := r.URL.Query().Get("seriesId")
		if seriesID == "1" {
			json.NewEncoder(w).Encode([]arr.SonarrEpisodeFileResponse{
				{ID: 100, SeriesID: 1, SeasonNumber: 1, Path: "/tv/ShowA/ep.mkv", Size: 1000, SceneName: "ShowA.S01E01-GRP"},
			})
		} else {
			json.NewEncoder(w).Encode([]arr.SonarrEpisodeFileResponse{
				{ID: 200, SeriesID: 2, SeasonNumber: 1, Path: "/movies/ShowB/ep.mkv", Size: 2000, SceneName: "ShowB.S01E01-GRP"},
			})
		}
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := arr.NewClient(srv.URL, "key", nil, nil, "sonarr", 30)
	config := &models.ArrSeedInstanceConfig{
		ID:            1,
		ArrDockerPath: "", // Empty = scan all
	}
	scanner := &arrSeedScanner{}
	l := zerolog.Nop()

	items, err := scanner.scanSonarrInstance(context.Background(), client, config, &l)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items (no path filter), got %d", len(items))
	}
}

// --- Virtual season pack promotion ---

func TestArrSeedScanSonarrInstance_VirtualSeasonPackPromotion(t *testing.T) {
	// When all episodes in a season have the same release group,
	// they should be promoted to a virtual season pack.
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/series", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]arr.SonarrSeriesResponse{
			{ID: 1, Title: "Test Show", Path: "/tv/Test Show"},
		})
	})
	mux.HandleFunc("/api/v3/episodefile", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]arr.SonarrEpisodeFileResponse{
			{ID: 100, SeriesID: 1, SeasonNumber: 1, Path: "/tv/Test Show/S01E01.mkv", Size: 1000, SceneName: "Test.Show.S01E01.720p-GRP", ReleaseGroup: "GRP"},
			{ID: 101, SeriesID: 1, SeasonNumber: 1, Path: "/tv/Test Show/S01E02.mkv", Size: 2000, SceneName: "Test.Show.S01E02.720p-GRP", ReleaseGroup: "GRP"},
			{ID: 102, SeriesID: 1, SeasonNumber: 1, Path: "/tv/Test Show/S01E03.mkv", Size: 3000, SceneName: "Test.Show.S01E03.720p-GRP", ReleaseGroup: "GRP"},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := arr.NewClient(srv.URL, "key", nil, nil, "sonarr", 30)
	config := &models.ArrSeedInstanceConfig{ID: 1}
	scanner := &arrSeedScanner{}
	l := zerolog.Nop()

	items, err := scanner.scanSonarrInstance(context.Background(), client, config, &l)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 3 episodes from same group should be promoted to 1 season pack
	// (individual episodes are skipped when promoted)
	seasonPacks := 0
	episodes := 0
	for _, item := range items {
		switch item.ItemType {
		case "season_pack":
			seasonPacks++
		case "episode":
			episodes++
		}
	}

	if seasonPacks != 1 {
		t.Errorf("expected 1 virtual season pack, got %d", seasonPacks)
	}
	if episodes != 0 {
		t.Errorf("expected 0 individual episodes (promoted to pack), got %d", episodes)
	}
}

func TestArrSeedScanSonarrInstance_NoPromotionMixedGroups(t *testing.T) {
	// When episodes have different release groups, no promotion occurs.
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/series", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]arr.SonarrSeriesResponse{
			{ID: 1, Title: "Test Show", Path: "/tv/Test Show"},
		})
	})
	mux.HandleFunc("/api/v3/episodefile", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]arr.SonarrEpisodeFileResponse{
			{ID: 100, SeriesID: 1, SeasonNumber: 1, Path: "/tv/Test Show/S01E01.mkv", Size: 1000, SceneName: "Test.Show.S01E01.720p-GRP1", ReleaseGroup: "GRP1"},
			{ID: 101, SeriesID: 1, SeasonNumber: 1, Path: "/tv/Test Show/S01E02.mkv", Size: 2000, SceneName: "Test.Show.S01E02.720p-GRP2", ReleaseGroup: "GRP2"},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := arr.NewClient(srv.URL, "key", nil, nil, "sonarr", 30)
	config := &models.ArrSeedInstanceConfig{ID: 1}
	scanner := &arrSeedScanner{}
	l := zerolog.Nop()

	items, err := scanner.scanSonarrInstance(context.Background(), client, config, &l)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Mixed groups = no promotion, items stay as individual episodes
	for _, item := range items {
		if item.ItemType == "season_pack" {
			t.Error("expected no season pack promotion with mixed release groups")
		}
	}
	if len(items) != 2 {
		t.Errorf("expected 2 individual episodes, got %d", len(items))
	}
}

func TestArrSeedScanSonarrInstance_NoPromotionSingleEpisode(t *testing.T) {
	// A single episode should never be promoted to a season pack
	// even if it has a release group (requires >= 2 files).
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/series", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]arr.SonarrSeriesResponse{
			{ID: 1, Title: "Test", Path: "/tv/Test"},
		})
	})
	mux.HandleFunc("/api/v3/episodefile", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]arr.SonarrEpisodeFileResponse{
			{ID: 100, SeriesID: 1, SeasonNumber: 1, Path: "/tv/Test/ep.mkv", Size: 1000, SceneName: "Test.S01E01.720p-GRP", ReleaseGroup: "GRP"},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := arr.NewClient(srv.URL, "key", nil, nil, "sonarr", 30)
	config := &models.ArrSeedInstanceConfig{ID: 1}
	scanner := &arrSeedScanner{}
	l := zerolog.Nop()

	items, err := scanner.scanSonarrInstance(context.Background(), client, config, &l)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ItemType != "episode" {
		t.Errorf("single episode should not be promoted, got %q", items[0].ItemType)
	}
}

func TestArrSeedScanSonarrInstance_NoPromotionEmptyGroup(t *testing.T) {
	// Empty release group should not trigger promotion.
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/series", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]arr.SonarrSeriesResponse{
			{ID: 1, Title: "Test", Path: "/tv/Test"},
		})
	})
	mux.HandleFunc("/api/v3/episodefile", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]arr.SonarrEpisodeFileResponse{
			{ID: 100, SeriesID: 1, SeasonNumber: 1, Path: "/tv/Test/ep1.mkv", Size: 1000, SceneName: "Test.S01E01.720p", ReleaseGroup: ""},
			{ID: 101, SeriesID: 1, SeasonNumber: 1, Path: "/tv/Test/ep2.mkv", Size: 2000, SceneName: "Test.S01E02.720p", ReleaseGroup: ""},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := arr.NewClient(srv.URL, "key", nil, nil, "sonarr", 30)
	config := &models.ArrSeedInstanceConfig{ID: 1}
	scanner := &arrSeedScanner{}
	l := zerolog.Nop()

	items, err := scanner.scanSonarrInstance(context.Background(), client, config, &l)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, item := range items {
		if item.ItemType == "season_pack" {
			t.Error("expected no season pack promotion with empty release group")
		}
	}
}

// --- Radarr path filtering ---

func TestArrSeedScanRadarrInstance_SkipsMoviesOutsideDockerPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/movie", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]arr.RadarrMovieResponse{
			{
				ID: 1, Title: "Inside Movie", Path: "/movies/Inside", HasFile: true,
				MovieFile: &arr.RadarrMovieFileInline{ID: 100, Path: "/movies/Inside/movie.mkv", Size: 5000, SceneName: "Inside.2024-GRP"},
			},
			{
				ID: 2, Title: "Outside Movie", Path: "/tv/Outside", HasFile: true,
				MovieFile: &arr.RadarrMovieFileInline{ID: 200, Path: "/tv/Outside/movie.mkv", Size: 5000, SceneName: "Outside.2024-GRP"},
			},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := arr.NewClient(srv.URL, "key", nil, nil, "radarr", 30)
	config := &models.ArrSeedInstanceConfig{
		ID:            1,
		ArrDockerPath: "/movies",
	}
	scanner := &arrSeedScanner{}
	l := zerolog.Nop()

	items, err := scanner.scanRadarrInstance(context.Background(), client, config, &l)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item (outside path skipped), got %d", len(items))
	}
	if items[0].Title != "Inside Movie" {
		t.Errorf("expected 'Inside Movie', got %q", items[0].Title)
	}
}

func TestArrSeedScanRadarrInstance_SkipsMoviesWithNoSceneNameAndNoHistory(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/movie", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]arr.RadarrMovieResponse{
			{
				ID: 1, Title: "No Release", Path: "/movies/NoRelease", HasFile: true,
				MovieFile: &arr.RadarrMovieFileInline{ID: 100, Path: "/movies/NoRelease/movie.mkv", Size: 5000, SceneName: ""},
			},
		})
	})
	mux.HandleFunc("/api/v3/history/movie", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]arr.RadarrHistoryRecord{})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := arr.NewClient(srv.URL, "key", nil, nil, "radarr", 30)
	config := &models.ArrSeedInstanceConfig{ID: 1}
	scanner := &arrSeedScanner{}
	l := zerolog.Nop()

	items, err := scanner.scanRadarrInstance(context.Background(), client, config, &l)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items (no release name), got %d", len(items))
	}
}

// --- Sonarr: season pack already in release name ---

func TestArrSeedScanSonarrInstance_ExplicitSeasonPackInReleaseName(t *testing.T) {
	// When the sceneName is already a season pack (S01 without episode),
	// it should be treated as a season pack directly, not promoted.
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/series", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]arr.SonarrSeriesResponse{
			{ID: 1, Title: "Show", Path: "/tv/Show"},
		})
	})
	mux.HandleFunc("/api/v3/episodefile", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]arr.SonarrEpisodeFileResponse{
			{ID: 100, SeriesID: 1, SeasonNumber: 1, Path: "/tv/Show/ep1.mkv", Size: 1000, SceneName: "Show.S01.1080p-GRP", ReleaseGroup: "GRP"},
			{ID: 101, SeriesID: 1, SeasonNumber: 1, Path: "/tv/Show/ep2.mkv", Size: 2000, SceneName: "Show.S01.1080p-GRP", ReleaseGroup: "GRP"},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := arr.NewClient(srv.URL, "key", nil, nil, "sonarr", 30)
	config := &models.ArrSeedInstanceConfig{ID: 1}
	scanner := &arrSeedScanner{}
	l := zerolog.Nop()

	items, err := scanner.scanSonarrInstance(context.Background(), client, config, &l)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both files share the same season pack scene name, they should be grouped
	// into one season pack item
	seasonPacks := 0
	for _, item := range items {
		if item.ItemType == "season_pack" {
			seasonPacks++
			if item.FileSize != 3000 {
				t.Errorf("season pack total size: got %d, want 3000", item.FileSize)
			}
		}
	}
	if seasonPacks != 1 {
		t.Errorf("expected 1 season pack from grouped files, got %d", seasonPacks)
	}
}
