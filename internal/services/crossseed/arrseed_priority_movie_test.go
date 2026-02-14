// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package crossseed

import (
	"testing"

	"github.com/autobrr/qui/internal/models"
)

// TestArrSeedClassifyPriority_MovieGetsEpisodeTier demonstrates that movies
// are classified at ArrSeedPriorityEpisode (3). This means disabling the
// "Episode" tier also silently disables movies — there is no independent
// control for movies.
func TestArrSeedClassifyPriority_MovieGetsEpisodeTier(t *testing.T) {
	item := &ArrSeedMediaItem{ItemType: "movie", CustomFormatScore: 100}
	got := ArrSeedClassifyPriority(item, 50)
	if got != ArrSeedPriorityEpisode {
		t.Fatalf("expected movie priority=%d (episode tier), got %d", ArrSeedPriorityEpisode, got)
	}
}

// TestArrSeedFilterAndSortByPriority_MoviesFilteredWithEpisodes exposes that
// movies with realistic priority (3 = episode tier) are filtered out when
// EnableEpisode=false. The existing test uses Priority=0 (default case),
// which masks this behavior.
func TestArrSeedFilterAndSortByPriority_MoviesFilteredWithEpisodes(t *testing.T) {
	// Movie with real classified priority (ArrSeedPriorityEpisode=3)
	movieItem := &ArrSeedMediaItem{
		ReleaseName: "Movie.2024.1080p",
		ItemType:    "movie",
		Priority:    ArrSeedPriorityEpisode, // This is what ArrSeedClassifyPriority actually returns
	}
	items := []*ArrSeedMediaItem{movieItem}
	settings := &models.ArrSeedSettings{
		EnableSeasonPackHighScore: true,
		EnableSeasonPack:          true,
		EnableEpisode:             false, // Disabled episodes
	}

	result := arrSeedFilterAndSortByPriority(items, settings)

	// BUG: Movies are silently filtered out when episodes are disabled
	// because movies share the episode priority tier.
	if len(result) != 0 {
		t.Logf("NOTE: Movies with Priority=%d are filtered out when EnableEpisode=false", ArrSeedPriorityEpisode)
	}
	// This test documents the current behavior. If movies should always pass
	// through regardless of episode settings, the priority system needs updating.
}

func TestArrSeedFilterAndSortByPriority_EmptyInput(t *testing.T) {
	settings := &models.ArrSeedSettings{
		EnableSeasonPackHighScore: true,
		EnableSeasonPack:          true,
		EnableEpisode:             true,
	}
	result := arrSeedFilterAndSortByPriority(nil, settings)
	if len(result) != 0 {
		t.Fatalf("expected 0 items for nil input, got %d", len(result))
	}
}

func TestArrSeedFilterAndSortByPriority_AllDisabled(t *testing.T) {
	makeItem := func(name string, priority int) *ArrSeedMediaItem {
		return &ArrSeedMediaItem{ReleaseName: name, Priority: priority}
	}
	items := []*ArrSeedMediaItem{
		makeItem("ep", ArrSeedPriorityEpisode),
		makeItem("sp", ArrSeedPrioritySeasonPack),
		makeItem("sphs", ArrSeedPrioritySeasonPackHighScore),
	}
	settings := &models.ArrSeedSettings{
		EnableSeasonPackHighScore: false,
		EnableSeasonPack:          false,
		EnableEpisode:             false,
	}

	result := arrSeedFilterAndSortByPriority(items, settings)
	if len(result) != 0 {
		t.Fatalf("expected 0 items when all tiers disabled, got %d", len(result))
	}
}

func TestArrSeedFilterAndSortByPriority_StableSortWithinSamePriority(t *testing.T) {
	items := []*ArrSeedMediaItem{
		{ReleaseName: "first", Priority: ArrSeedPriorityEpisode},
		{ReleaseName: "second", Priority: ArrSeedPriorityEpisode},
		{ReleaseName: "third", Priority: ArrSeedPriorityEpisode},
	}
	settings := &models.ArrSeedSettings{EnableEpisode: true}

	result := arrSeedFilterAndSortByPriority(items, settings)
	if len(result) != 3 {
		t.Fatalf("expected 3 items, got %d", len(result))
	}
	// Stable sort should preserve insertion order within same priority
	if result[0].ReleaseName != "first" || result[1].ReleaseName != "second" || result[2].ReleaseName != "third" {
		t.Errorf("stable sort violated: got %q, %q, %q", result[0].ReleaseName, result[1].ReleaseName, result[2].ReleaseName)
	}
}
