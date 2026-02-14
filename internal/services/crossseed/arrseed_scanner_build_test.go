// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package crossseed

import (
	"testing"

	"github.com/rs/zerolog"

	"github.com/autobrr/qui/internal/models"
	"github.com/autobrr/qui/internal/services/arr"
)

func TestBuildSeasonPackItem_BasicFields(t *testing.T) {
	scanner := &arrSeedScanner{}
	l := zerolog.Nop()

	series := arr.SonarrSeriesResponse{
		ID:               42,
		Title:            "Breaking Bad",
		Path:             "/tv/Breaking Bad",
		IMDbID:           "tt0903747",
		TVDbID:           81189,
		QualityProfileID: 7,
	}

	config := &models.ArrSeedInstanceConfig{
		ID:            1,
		ArrDockerPath: "/tv",
		HostDataPath:  "/data/tv",
	}

	files := []arrSeedResolvedFile{
		{
			ef: arr.SonarrEpisodeFileResponse{
				ID:                100,
				SeasonNumber:      2,
				Path:              "/tv/Breaking Bad/Season 02/ep1.mkv",
				Size:              1000000,
				CustomFormatScore: 50,
			},
			releaseName: "Breaking.Bad.S02E01.1080p-GRP",
			source:      "sceneName",
		},
		{
			ef: arr.SonarrEpisodeFileResponse{
				ID:                101,
				SeasonNumber:      2,
				Path:              "/tv/Breaking Bad/Season 02/ep2.mkv",
				Size:              2000000,
				CustomFormatScore: 80,
			},
			releaseName: "Breaking.Bad.S02E02.1080p-GRP",
			source:      "sceneName",
		},
	}

	item := scanner.buildSeasonPackItem(series, config, files, "Breaking.Bad.S02.1080p-GRP", &l)

	if item.ItemType != "season_pack" {
		t.Errorf("ItemType: got %q, want %q", item.ItemType, "season_pack")
	}
	if item.ArrInstanceType != "sonarr" {
		t.Errorf("ArrInstanceType: got %q, want %q", item.ArrInstanceType, "sonarr")
	}
	if item.ConfigID != 1 {
		t.Errorf("ConfigID: got %d, want 1", item.ConfigID)
	}
	if item.ReleaseName != "Breaking.Bad.S02.1080p-GRP" {
		t.Errorf("ReleaseName: got %q", item.ReleaseName)
	}
	if item.Title != "Breaking Bad" {
		t.Errorf("Title: got %q", item.Title)
	}
	if item.SeriesID != 42 {
		t.Errorf("SeriesID: got %d, want 42", item.SeriesID)
	}
	if item.SeasonNumber != 2 {
		t.Errorf("SeasonNumber: got %d, want 2", item.SeasonNumber)
	}
	if item.QualityProfileID != 7 {
		t.Errorf("QualityProfileID: got %d, want 7", item.QualityProfileID)
	}
}

func TestBuildSeasonPackItem_TotalSize(t *testing.T) {
	scanner := &arrSeedScanner{}
	l := zerolog.Nop()

	files := []arrSeedResolvedFile{
		{ef: arr.SonarrEpisodeFileResponse{ID: 1, SeasonNumber: 1, Path: "/tv/S/ep1.mkv", Size: 3000000}},
		{ef: arr.SonarrEpisodeFileResponse{ID: 2, SeasonNumber: 1, Path: "/tv/S/ep2.mkv", Size: 4000000}},
		{ef: arr.SonarrEpisodeFileResponse{ID: 3, SeasonNumber: 1, Path: "/tv/S/ep3.mkv", Size: 5000000}},
	}

	item := scanner.buildSeasonPackItem(
		arr.SonarrSeriesResponse{ID: 1, Title: "Show"},
		&models.ArrSeedInstanceConfig{ID: 1},
		files, "Show.S01-GRP", &l,
	)

	want := int64(12000000)
	if item.FileSize != want {
		t.Errorf("FileSize: got %d, want %d", item.FileSize, want)
	}
}

func TestBuildSeasonPackItem_MaxCustomFormatScore(t *testing.T) {
	scanner := &arrSeedScanner{}
	l := zerolog.Nop()

	files := []arrSeedResolvedFile{
		{ef: arr.SonarrEpisodeFileResponse{ID: 1, SeasonNumber: 1, Path: "/tv/S/ep1.mkv", Size: 1000, CustomFormatScore: 30}},
		{ef: arr.SonarrEpisodeFileResponse{ID: 2, SeasonNumber: 1, Path: "/tv/S/ep2.mkv", Size: 1000, CustomFormatScore: 90}},
		{ef: arr.SonarrEpisodeFileResponse{ID: 3, SeasonNumber: 1, Path: "/tv/S/ep3.mkv", Size: 1000, CustomFormatScore: 60}},
	}

	item := scanner.buildSeasonPackItem(
		arr.SonarrSeriesResponse{ID: 1, Title: "Show"},
		&models.ArrSeedInstanceConfig{ID: 1},
		files, "Show.S01-GRP", &l,
	)

	if item.CustomFormatScore != 90 {
		t.Errorf("CustomFormatScore: got %d, want 90 (max of all files)", item.CustomFormatScore)
	}
}

func TestBuildSeasonPackItem_PathMapping(t *testing.T) {
	scanner := &arrSeedScanner{}
	l := zerolog.Nop()

	files := []arrSeedResolvedFile{
		{ef: arr.SonarrEpisodeFileResponse{ID: 1, SeasonNumber: 1, Path: "/mnt/tv/Show/Season 01/ep1.mkv", Size: 1000}},
		{ef: arr.SonarrEpisodeFileResponse{ID: 2, SeasonNumber: 1, Path: "/mnt/tv/Show/Season 01/ep2.mkv", Size: 2000}},
	}

	config := &models.ArrSeedInstanceConfig{
		ID:            1,
		ArrDockerPath: "/mnt/tv",
		HostDataPath:  "/host/media/tv",
	}

	item := scanner.buildSeasonPackItem(
		arr.SonarrSeriesResponse{ID: 1, Title: "Show"},
		config, files, "Show.S01-GRP", &l,
	)

	// HostFilePath should be mapped from the first file
	wantPath := "/host/media/tv/Show/Season 01/ep1.mkv"
	if item.HostFilePath != wantPath {
		t.Errorf("HostFilePath: got %q, want %q", item.HostFilePath, wantPath)
	}

	// HostFiles should contain all files with mapped paths
	if len(item.HostFiles) != 2 {
		t.Fatalf("HostFiles count: got %d, want 2", len(item.HostFiles))
	}
	if item.HostFiles[0].Path != "/host/media/tv/Show/Season 01/ep1.mkv" {
		t.Errorf("HostFiles[0].Path: got %q", item.HostFiles[0].Path)
	}
	if item.HostFiles[1].Path != "/host/media/tv/Show/Season 01/ep2.mkv" {
		t.Errorf("HostFiles[1].Path: got %q", item.HostFiles[1].Path)
	}
	if item.HostFiles[0].Size != 1000 || item.HostFiles[1].Size != 2000 {
		t.Errorf("HostFiles sizes: got %d, %d", item.HostFiles[0].Size, item.HostFiles[1].Size)
	}
}

func TestBuildSeasonPackItem_SyntheticID(t *testing.T) {
	scanner := &arrSeedScanner{}
	l := zerolog.Nop()

	files := []arrSeedResolvedFile{
		{ef: arr.SonarrEpisodeFileResponse{ID: 1, SeasonNumber: 3, Path: "/tv/S/ep.mkv", Size: 1000}},
	}

	item := scanner.buildSeasonPackItem(
		arr.SonarrSeriesResponse{ID: 5, Title: "Show"},
		&models.ArrSeedInstanceConfig{ID: 1},
		files, "Show.S03-GRP", &l,
	)

	// Synthetic ID = seriesID * 10000 + seasonNumber
	want := 5*10000 + 3
	if item.ArrFileID != want {
		t.Errorf("ArrFileID (synthetic): got %d, want %d", item.ArrFileID, want)
	}
}

func TestBuildSeasonPackItem_ExternalIDs(t *testing.T) {
	scanner := &arrSeedScanner{}
	l := zerolog.Nop()

	files := []arrSeedResolvedFile{
		{ef: arr.SonarrEpisodeFileResponse{ID: 1, SeasonNumber: 1, Path: "/tv/S/ep.mkv", Size: 1000}},
	}

	item := scanner.buildSeasonPackItem(
		arr.SonarrSeriesResponse{ID: 1, Title: "Show", IMDbID: "tt1234567", TVDbID: 99999},
		&models.ArrSeedInstanceConfig{ID: 1},
		files, "Show.S01-GRP", &l,
	)

	if item.ExternalIDs.IMDbID != "tt1234567" {
		t.Errorf("IMDbID: got %q", item.ExternalIDs.IMDbID)
	}
	if item.ExternalIDs.TVDbID != 99999 {
		t.Errorf("TVDbID: got %d", item.ExternalIDs.TVDbID)
	}
}
