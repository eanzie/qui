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

func TestArrSeedMapPath_ReplacesPrefix(t *testing.T) {
	got := arrSeedMapPath("/mnt/media/tv/Show/Season 01/ep.mkv", "/mnt/media/tv", "/mnt/media/tv/data")
	want := "/mnt/media/tv/data/Show/Season 01/ep.mkv"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestArrSeedMapPath_EmptyPaths_ReturnsOriginal(t *testing.T) {
	got := arrSeedMapPath("/mnt/media/tv/Show/ep.mkv", "", "")
	if got != "/mnt/media/tv/Show/ep.mkv" {
		t.Fatalf("expected original path, got %q", got)
	}
}

func TestArrSeedMapPath_NoMatch_ReturnsOriginal(t *testing.T) {
	got := arrSeedMapPath("/other/path/file.mkv", "/mnt/media/tv", "/mnt/media/tv/data")
	if got != "/other/path/file.mkv" {
		t.Fatalf("expected original path, got %q", got)
	}
}

func TestArrSeedMapPath_OnlyFirstOccurrence(t *testing.T) {
	got := arrSeedMapPath("/data/data/file.mkv", "/data", "/newdata")
	want := "/newdata/data/file.mkv"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestArrSeedScanSonarrInstance_BuildsItems(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/series", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]arr.SonarrSeriesResponse{
			{ID: 1, Title: "Test Show", Path: "/tv/Test Show", IMDbID: "tt1234567", TVDbID: 12345},
		})
	})
	mux.HandleFunc("/api/v3/episodefile", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]arr.SonarrEpisodeFileResponse{
			{
				ID:           100,
				SeriesID:     1,
				SeasonNumber: 1,
				Path:         "/tv/Test Show/Season 01/ep.mkv",
				Size:         1000000,
				SceneName:    "Test.Show.S01E01.720p-GRP",
			},
			{
				ID:           101,
				SeriesID:     1,
				SeasonNumber: 1,
				Path:         "/tv/Test Show/Season 01/ep2.mkv",
				Size:         2000000,
				SceneName:    "", // Will need history fallback
			},
		})
	})
	mux.HandleFunc("/api/v3/history/series", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]arr.SonarrHistoryRecord{
			{
				ID:            1,
				EpisodeFileID: 101,
				SeriesID:      1,
				SourceTitle:   "Test.Show.S01E02.720p-GRP",
				EventType:     "grabbed",
			},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := arr.NewClient(srv.URL, "testkey", nil, nil, "sonarr", 30)
	config := &models.ArrSeedInstanceConfig{
		ID:            1,
		ArrDockerPath: "/tv",
		HostDataPath:  "/data/tv",
	}

	scanner := &arrSeedScanner{}
	l := zerolog.Nop()
	items, err := scanner.scanSonarrInstance(context.Background(), client, config, &l)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// First item: has sceneName directly
	if items[0].ReleaseName != "Test.Show.S01E01.720p-GRP" {
		t.Errorf("item 0 release name: got %q, want %q", items[0].ReleaseName, "Test.Show.S01E01.720p-GRP")
	}
	if items[0].HostFilePath != "/data/tv/Test Show/Season 01/ep.mkv" {
		t.Errorf("item 0 host path: got %q, want %q", items[0].HostFilePath, "/data/tv/Test Show/Season 01/ep.mkv")
	}
	if items[0].ExternalIDs.IMDbID != "tt1234567" {
		t.Errorf("item 0 IMDbID: got %q, want %q", items[0].ExternalIDs.IMDbID, "tt1234567")
	}

	// Second item: sceneName from history fallback
	if items[1].ReleaseName != "Test.Show.S01E02.720p-GRP" {
		t.Errorf("item 1 release name: got %q, want %q", items[1].ReleaseName, "Test.Show.S01E02.720p-GRP")
	}
}

func TestArrSeedScanSonarrInstance_SkipsFilesWithoutReleaseName(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/series", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]arr.SonarrSeriesResponse{
			{ID: 1, Title: "Show", Path: "/tv/Show"},
		})
	})
	mux.HandleFunc("/api/v3/episodefile", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]arr.SonarrEpisodeFileResponse{
			{ID: 100, SeriesID: 1, Path: "/tv/Show/ep.mkv", Size: 1000, SceneName: ""},
		})
	})
	mux.HandleFunc("/api/v3/history/series", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]arr.SonarrHistoryRecord{})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := arr.NewClient(srv.URL, "testkey", nil, nil, "sonarr", 30)
	config := &models.ArrSeedInstanceConfig{ID: 1}
	scanner := &arrSeedScanner{}
	l := zerolog.Nop()

	items, err := scanner.scanSonarrInstance(context.Background(), client, config, &l)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected 0 items (no release names), got %d", len(items))
	}
}

func TestArrSeedScanRadarrInstance_BuildsItems(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/movie", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]arr.RadarrMovieResponse{
			{
				ID:      1,
				Title:   "Test Movie",
				Path:    "/movies/Test Movie (2024)",
				TMDbID:  99999,
				IMDbID:  "tt9999999",
				HasFile: true,
				MovieFile: &arr.RadarrMovieFileInline{
					ID:        200,
					Path:      "/movies/Test Movie (2024)/Test.Movie.2024.1080p.mkv",
					Size:      5000000,
					SceneName: "Test.Movie.2024.1080p-GRP",
				},
			},
			{
				ID:      2,
				Title:   "No File Movie",
				HasFile: false,
			},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := arr.NewClient(srv.URL, "testkey", nil, nil, "radarr", 30)
	config := &models.ArrSeedInstanceConfig{
		ID:            1,
		ArrDockerPath: "/movies",
		HostDataPath:  "/data/movies",
	}

	scanner := &arrSeedScanner{}
	l := zerolog.Nop()
	items, err := scanner.scanRadarrInstance(context.Background(), client, config, &l)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(items) != 1 {
		t.Fatalf("expected 1 item (movie without file skipped), got %d", len(items))
	}

	if items[0].ReleaseName != "Test.Movie.2024.1080p-GRP" {
		t.Errorf("release name: got %q, want %q", items[0].ReleaseName, "Test.Movie.2024.1080p-GRP")
	}
	if items[0].HostFilePath != "/data/movies/Test Movie (2024)/Test.Movie.2024.1080p.mkv" {
		t.Errorf("host path: got %q", items[0].HostFilePath)
	}
	if items[0].ExternalIDs.TMDbID != 99999 {
		t.Errorf("TMDbID: got %d, want 99999", items[0].ExternalIDs.TMDbID)
	}
	if items[0].ItemType != "movie" {
		t.Errorf("item type: got %q, want %q", items[0].ItemType, "movie")
	}
}

func TestArrSeedScanRadarrInstance_FallsBackToHistory(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/movie", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]arr.RadarrMovieResponse{
			{
				ID:      1,
				Title:   "Movie",
				HasFile: true,
				MovieFile: &arr.RadarrMovieFileInline{
					ID:        200,
					Path:      "/movies/Movie/file.mkv",
					Size:      5000000,
					SceneName: "",
				},
			},
		})
	})
	mux.HandleFunc("/api/v3/history/movie", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]arr.RadarrHistoryRecord{
			{ID: 1, MovieID: 1, SourceTitle: "Movie.2024.BluRay-GRP", EventType: "grabbed"},
		})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := arr.NewClient(srv.URL, "testkey", nil, nil, "radarr", 30)
	config := &models.ArrSeedInstanceConfig{ID: 1}
	scanner := &arrSeedScanner{}
	l := zerolog.Nop()

	items, err := scanner.scanRadarrInstance(context.Background(), client, config, &l)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].ReleaseName != "Movie.2024.BluRay-GRP" {
		t.Errorf("release name: got %q, want %q", items[0].ReleaseName, "Movie.2024.BluRay-GRP")
	}
}

func TestArrSeedScanSonarrInstance_CancelledContext(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v3/series", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]arr.SonarrSeriesResponse{
			{ID: 1, Title: "Show", Path: "/tv/Show"},
			{ID: 2, Title: "Show2", Path: "/tv/Show2"},
		})
	})
	mux.HandleFunc("/api/v3/episodefile", func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode([]arr.SonarrEpisodeFileResponse{})
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := arr.NewClient(srv.URL, "testkey", nil, nil, "sonarr", 30)
	config := &models.ArrSeedInstanceConfig{ID: 1}
	scanner := &arrSeedScanner{}
	l := zerolog.Nop()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := scanner.scanSonarrInstance(ctx, client, config, &l)
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}
