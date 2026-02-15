// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package crossseed

import (
	"testing"

	"github.com/autobrr/qui/internal/models"
)

// TestArrSeedClassifyPriority_MovieGetsNormalPriority verifies that movies
// without high score are classified at ArrSeedPriorityNormal (2), distinct
// from episodes (3). Movies are no longer silently filtered when episodes
// are disabled.
func TestArrSeedClassifyPriority_MovieGetsNormalPriority(t *testing.T) {
	item := &ArrSeedMediaItem{ItemType: "movie", CustomFormatScore: 0}
	got := ArrSeedClassifyPriority(item, 0)
	if got != ArrSeedPriorityNormal {
		t.Fatalf("expected movie priority=%d (normal), got %d", ArrSeedPriorityNormal, got)
	}
}

// TestArrSeedClassifyPriority_MovieHighScore verifies that movies with score
// >= cutoff get high score priority.
func TestArrSeedClassifyPriority_MovieHighScore(t *testing.T) {
	item := &ArrSeedMediaItem{ItemType: "movie", CustomFormatScore: 100}
	got := ArrSeedClassifyPriority(item, 50)
	if got != ArrSeedPriorityHighScore {
		t.Fatalf("expected movie priority=%d (high score), got %d", ArrSeedPriorityHighScore, got)
	}
}

// TestArrSeedFilterAndSortByPriority_MoviesPassWhenEpisodesDisabled confirms
// the fix: movies now pass through even when EnableEpisode=false because
// filtering is type-based, not priority-based.
func TestArrSeedFilterAndSortByPriority_MoviesPassWhenEpisodesDisabled(t *testing.T) {
	movieItem := &ArrSeedMediaItem{
		ReleaseName: "Movie.2024.1080p",
		ItemType:    "movie",
		Priority:    ArrSeedPriorityNormal,
	}
	items := []*ArrSeedMediaItem{movieItem}
	settings := &models.ArrSeedSettings{
		EnableEpisode: false,
	}

	result := arrSeedFilterAndSortByPriority(items, settings)

	if len(result) != 1 {
		t.Fatalf("expected 1 item (movie should pass when episodes disabled), got %d", len(result))
	}
	if result[0].ReleaseName != "Movie.2024.1080p" {
		t.Errorf("expected movie to pass through, got %q", result[0].ReleaseName)
	}
}

func TestArrSeedFilterAndSortByPriority_EmptyInput(t *testing.T) {
	settings := &models.ArrSeedSettings{
		EnableEpisode: true,
	}
	result := arrSeedFilterAndSortByPriority(nil, settings)
	if len(result) != 0 {
		t.Fatalf("expected 0 items for nil input, got %d", len(result))
	}
}

func TestArrSeedFilterAndSortByPriority_AllDisabledStillPassesMoviesAndSeasonPacks(t *testing.T) {
	items := []*ArrSeedMediaItem{
		{ReleaseName: "ep", ItemType: "episode", Priority: ArrSeedPriorityEpisode},
		{ReleaseName: "sp", ItemType: "season_pack", Priority: ArrSeedPriorityNormal},
		{ReleaseName: "movie", ItemType: "movie", Priority: ArrSeedPriorityNormal},
	}
	settings := &models.ArrSeedSettings{
		EnableEpisode: false,
	}

	result := arrSeedFilterAndSortByPriority(items, settings)
	if len(result) != 2 {
		t.Fatalf("expected 2 items (season pack + movie), got %d", len(result))
	}
}

func TestArrSeedFilterAndSortByPriority_StableSortWithinSamePriority(t *testing.T) {
	items := []*ArrSeedMediaItem{
		{ReleaseName: "first", ItemType: "episode", Priority: ArrSeedPriorityEpisode},
		{ReleaseName: "second", ItemType: "episode", Priority: ArrSeedPriorityEpisode},
		{ReleaseName: "third", ItemType: "episode", Priority: ArrSeedPriorityEpisode},
	}
	settings := &models.ArrSeedSettings{EnableEpisode: true}

	result := arrSeedFilterAndSortByPriority(items, settings)
	if len(result) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result))
	}
	if result[0].ReleaseName != "first" || result[1].ReleaseName != "second" || result[2].ReleaseName != "third" {
		t.Errorf("stable sort violated: got %q, %q, %q", result[0].ReleaseName, result[1].ReleaseName, result[2].ReleaseName)
	}
}

func TestArrSeedFilterPendingItems(t *testing.T) {
	t.Run("movies and season packs always pass", func(t *testing.T) {
		items := []*models.ArrSeedItem{
			{ItemType: "movie", Priority: ArrSeedPriorityNormal},
			{ItemType: "season_pack", Priority: ArrSeedPriorityNormal},
			{ItemType: "episode", Priority: ArrSeedPriorityEpisode},
		}
		settings := &models.ArrSeedSettings{EnableEpisode: false}

		result := arrSeedFilterPendingItems(items, settings)
		if len(result) != 2 {
			t.Fatalf("expected 2 items, got %d", len(result))
		}
	})

	t.Run("high score only gate", func(t *testing.T) {
		items := []*models.ArrSeedItem{
			{ItemType: "movie", Priority: ArrSeedPriorityHighScore},
			{ItemType: "movie", Priority: ArrSeedPriorityNormal},
			{ItemType: "season_pack", Priority: ArrSeedPriorityHighScore},
			{ItemType: "season_pack", Priority: ArrSeedPriorityNormal},
		}
		settings := &models.ArrSeedSettings{EnableHighScoreOnly: true}

		result := arrSeedFilterPendingItems(items, settings)
		if len(result) != 2 {
			t.Fatalf("expected 2 high-score items, got %d", len(result))
		}
	})
}
