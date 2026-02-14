// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package crossseed

import (
	"testing"

	"github.com/autobrr/qui/internal/models"
)

func TestArrSeedIsSeasonPack(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"season pack", "Show.Name.S01.1080p.BluRay-GROUP", true},
		{"single episode", "Show.Name.S01E01.1080p", false},
		{"multi-season pack", "Show.S01-S02.1080p", true},
		{"multi-episode", "Show.S01E01-E03", false},
		{"movie no season", "Movie.2024.1080p", false},
		{"empty string", "", false},
		{"season pack with complete", "Show.S02.Complete.720p", true},
		{"compact episode S0401", "The.Wire.S0401.1080p.BluRay-GROUP", false},
		{"compact episode S0113", "Show.S0113.720p.x264-GROUP", false},
		{"compact 3-digit S101", "Show.S101.720p-GROUP", false},
		{"single digit season", "Show.S1.1080p-GROUP", true},
		{"season 9 no episode", "Snapped.S09.720p.x264-P2P", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ArrSeedIsSeasonPack(tt.input)
			if got != tt.expected {
				t.Errorf("ArrSeedIsSeasonPack(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestArrSeedSynthesizeSeasonPackName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"standard episode", "Show.S01E05.1080p.WEB-DL-NTb", "Show.S01.1080p.WEB-DL-NTb"},
		{"multi episode", "Show.S01E01-E03.1080p-GROUP", "Show.S01.1080p-GROUP"},
		{"compact S0401", "The.Wire.S0401.1080p.BluRay.DD5.1.x264-CtrlHD", "The.Wire.S04.1080p.BluRay.DD5.1.x264-CtrlHD"},
		{"compact S0113", "Show.S0113.720p-GROUP", "Show.S01.720p-GROUP"},
		{"already season pack", "Show.S01.1080p-GROUP", "Show.S01.1080p-GROUP"},
		{"no season info", "Movie.2024.1080p", "Movie.2024.1080p"},
		{"episode title stripped", "Its.in.the.Game.Madden.NFL.S01E01.CAN.A.COMPUTER.MAKE.YOU.CRY.1080p.AMZN.WEB-DL.DDP5.1.H.264-Kitsune", "Its.in.the.Game.Madden.NFL.S01.1080p.AMZN.WEB-DL.DDP5.1.H.264-Kitsune"},
		{"episode title with source only", "Show.Name.S02E03.Episode.Title.WEB-DL-GROUP", "Show.Name.S02.WEB-DL-GROUP"},
		{"season pack no episode title", "Snapped.S09E05.720p.x264-P2P", "Snapped.S09.720p.x264-P2P"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ArrSeedSynthesizeSeasonPackName(tt.input)
			if got != tt.expected {
				t.Errorf("ArrSeedSynthesizeSeasonPackName(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestArrSeedClassifyPriority(t *testing.T) {
	tests := []struct {
		name        string
		item        *ArrSeedMediaItem
		cutoffScore int
		expected    int
	}{
		{
			name:        "season pack with score >= threshold",
			item:        &ArrSeedMediaItem{ItemType: "season_pack", CustomFormatScore: 100},
			cutoffScore: 50,
			expected:    ArrSeedPrioritySeasonPackHighScore,
		},
		{
			name:        "season pack with score < threshold",
			item:        &ArrSeedMediaItem{ItemType: "season_pack", CustomFormatScore: 30},
			cutoffScore: 50,
			expected:    ArrSeedPrioritySeasonPack,
		},
		{
			name:        "season pack with cutoff 0 means no CF upgrade threshold set",
			item:        &ArrSeedMediaItem{ItemType: "season_pack", CustomFormatScore: 100},
			cutoffScore: 0,
			expected:    ArrSeedPrioritySeasonPack,
		},
		{
			name:        "episode",
			item:        &ArrSeedMediaItem{ItemType: "episode"},
			cutoffScore: 50,
			expected:    ArrSeedPriorityEpisode,
		},
		{
			name:        "movie uses default priority",
			item:        &ArrSeedMediaItem{ItemType: "movie"},
			cutoffScore: 0,
			expected:    ArrSeedPriorityEpisode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ArrSeedClassifyPriority(tt.item, tt.cutoffScore)
			if got != tt.expected {
				t.Errorf("ArrSeedClassifyPriority() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestArrSeedFilterAndSortByPriority(t *testing.T) {
	makeItem := func(name string, priority int) *ArrSeedMediaItem {
		return &ArrSeedMediaItem{ReleaseName: name, Priority: priority}
	}

	t.Run("all tiers enabled, sorted by priority", func(t *testing.T) {
		items := []*ArrSeedMediaItem{
			makeItem("episode1", ArrSeedPriorityEpisode),
			makeItem("season_high", ArrSeedPrioritySeasonPackHighScore),
			makeItem("season_normal", ArrSeedPrioritySeasonPack),
		}
		settings := &models.ArrSeedSettings{
			EnableSeasonPackHighScore: true,
			EnableSeasonPack:          true,
			EnableEpisode:             true,
		}

		result := arrSeedFilterAndSortByPriority(items, settings)

		if len(result) != 3 {
			t.Fatalf("expected 3 items, got %d", len(result))
		}
		if result[0].ReleaseName != "season_high" {
			t.Errorf("expected first item 'season_high', got %q", result[0].ReleaseName)
		}
		if result[1].ReleaseName != "season_normal" {
			t.Errorf("expected second item 'season_normal', got %q", result[1].ReleaseName)
		}
		if result[2].ReleaseName != "episode1" {
			t.Errorf("expected third item 'episode1', got %q", result[2].ReleaseName)
		}
	})

	t.Run("episode disabled, episodes filtered out", func(t *testing.T) {
		items := []*ArrSeedMediaItem{
			makeItem("episode1", ArrSeedPriorityEpisode),
			makeItem("season_high", ArrSeedPrioritySeasonPackHighScore),
			makeItem("season_normal", ArrSeedPrioritySeasonPack),
		}
		settings := &models.ArrSeedSettings{
			EnableSeasonPackHighScore: true,
			EnableSeasonPack:          true,
			EnableEpisode:             false,
		}

		result := arrSeedFilterAndSortByPriority(items, settings)

		if len(result) != 2 {
			t.Fatalf("expected 2 items, got %d", len(result))
		}
	})

	t.Run("movie items always pass through", func(t *testing.T) {
		movieItem := &ArrSeedMediaItem{ReleaseName: "Movie.2024.1080p", Priority: 0}
		items := []*ArrSeedMediaItem{
			makeItem("episode1", ArrSeedPriorityEpisode),
			movieItem,
		}
		settings := &models.ArrSeedSettings{
			EnableSeasonPackHighScore: false,
			EnableSeasonPack:          false,
			EnableEpisode:             false,
		}

		result := arrSeedFilterAndSortByPriority(items, settings)

		if len(result) != 1 {
			t.Fatalf("expected 1 item (movie), got %d", len(result))
		}
		if result[0].ReleaseName != "Movie.2024.1080p" {
			t.Errorf("expected movie to pass through, got %q", result[0].ReleaseName)
		}
	})
}
