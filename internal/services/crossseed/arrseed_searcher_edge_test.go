// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package crossseed

import (
	"testing"
)

// --- arrSeedBuildSearchQuery edge cases ---

func TestArrSeedBuildSearchQuery_SeasonPackZeroSeasonNumber(t *testing.T) {
	// Season 0 in Sonarr = "Specials". The code skips setting season
	// when SeasonNumber=0 and falls through to parse from release name.
	item := &ArrSeedMediaItem{
		ItemType:     "season_pack",
		ReleaseName:  "Show.S00.1080p-GRP",
		Title:        "Show",
		SeasonNumber: 0, // Specials
	}
	sq := arrSeedBuildSearchQuery(item)

	if sq.query != "Show" {
		t.Errorf("query: got %q, want %q", sq.query, "Show")
	}
	// SeasonNumber=0 is skipped by the code, so it falls through to parse from release
	// This test documents the behavior: specials may not get correct season parameter
	if sq.season != nil {
		t.Logf("season is set to %d (parsed from release name)", *sq.season)
	}
}

func TestArrSeedBuildSearchQuery_MovieWithoutYear(t *testing.T) {
	item := &ArrSeedMediaItem{
		ItemType:    "movie",
		ReleaseName: "SomeMovie.BluRay.1080p-GROUP",
		Title:       "SomeMovie",
	}
	sq := arrSeedBuildSearchQuery(item)

	if sq.query != "SomeMovie" {
		t.Errorf("query: got %q, want %q", sq.query, "SomeMovie")
	}
	if sq.year != 0 {
		t.Errorf("expected year=0 when no year in release, got %d", sq.year)
	}
}

func TestArrSeedBuildSearchQuery_WhitespaceTitle(t *testing.T) {
	item := &ArrSeedMediaItem{
		ItemType:    "episode",
		ReleaseName: "Show.S01E01.720p-GRP",
		Title:       "  ",
	}
	sq := arrSeedBuildSearchQuery(item)

	// TrimSpace("  ") = "", so it should fall back to parsed title
	if sq.query == "" || sq.query == "  " {
		t.Errorf("query should fall back from whitespace-only title, got %q", sq.query)
	}
}

func TestArrSeedBuildSearchQuery_EpisodeNoParsedSeason(t *testing.T) {
	item := &ArrSeedMediaItem{
		ItemType:    "episode",
		ReleaseName: "just-a-name",
		Title:       "Just A Name",
	}
	sq := arrSeedBuildSearchQuery(item)

	if sq.season != nil {
		t.Errorf("expected nil season when release has no S##, got %d", *sq.season)
	}
	if sq.episode != nil {
		t.Errorf("expected nil episode when release has no E##, got %d", *sq.episode)
	}
}

func TestArrSeedBuildSearchQuery_SeasonPackPrefersItemSeasonOverParsed(t *testing.T) {
	item := &ArrSeedMediaItem{
		ItemType:     "season_pack",
		ReleaseName:  "Show.S03.1080p-GRP",
		Title:        "Show",
		SeasonNumber: 5, // Differs from the S03 in the name
	}
	sq := arrSeedBuildSearchQuery(item)

	if sq.season == nil {
		t.Fatal("expected season to be set")
	}
	// Item's SeasonNumber (5) should take priority over parsed S03
	if *sq.season != 5 {
		t.Errorf("expected season=%d from item, got %d", 5, *sq.season)
	}
}

// --- arrSeedCategoriesForItem edge cases ---

func TestArrSeedCategoriesForItem_EmptyItemType(t *testing.T) {
	cats := arrSeedCategoriesForItem(&ArrSeedMediaItem{ItemType: ""})
	if cats != nil {
		t.Fatalf("expected nil for empty item type, got %v", cats)
	}
}

// --- arrSeedIsSeasonPack edge cases ---

func TestArrSeedIsSeasonPack_MultiSeasonRange(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"S01-S03 range", "Show.S01-S03.1080p-GRP", true},
		{"daily show no season", "Show.2024.01.15.720p-GRP", false},
		{"special char in title", "What.If...S01.720p-GRP", true},
		{"S0 (season zero specials)", "Show.S0.1080p-GRP", true}, // S0 matches \d{1,2} — regex treats it as valid 1-digit season
		{"S00 specials", "Show.S00.1080p-GRP", true},
		{"multi-digit season S12", "Show.S12.1080p-GRP", true},
		{"three-digit season S100", "Show.S100.1080p-GRP", false}, // \d{1,2} won't match 3 digits
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

// --- ArrSeedSynthesizeSeasonPackName edge cases ---

func TestArrSeedSynthesizeSeasonPackName_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"daily date episode", "Show.2024.01.15.720p-GRP", "Show.2024.01.15.720p-GRP"},
		{"no dots (space separated)", "Show Name S01E05 720p WEB-DL-GRP", "Show Name S01 720p WEB-DL-GRP"},
		{"double episode E01E02", "Show.S01E01E02.1080p-GRP", "Show.S01.1080p-GRP"},
		{"empty string", "", ""},
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
