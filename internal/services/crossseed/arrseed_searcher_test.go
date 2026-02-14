// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package crossseed

import (
	"testing"
)

func TestArrSeedCategoriesForItem_Episode(t *testing.T) {
	cats := arrSeedCategoriesForItem(&ArrSeedMediaItem{ItemType: "episode"})
	if len(cats) != 1 || cats[0] != 5000 {
		t.Fatalf("expected [5000], got %v", cats)
	}
}

func TestArrSeedCategoriesForItem_SeasonPack(t *testing.T) {
	cats := arrSeedCategoriesForItem(&ArrSeedMediaItem{ItemType: "season_pack"})
	if len(cats) != 1 || cats[0] != 5000 {
		t.Fatalf("expected [5000], got %v", cats)
	}
}

func TestArrSeedCategoriesForItem_Movie(t *testing.T) {
	cats := arrSeedCategoriesForItem(&ArrSeedMediaItem{ItemType: "movie"})
	if len(cats) != 1 || cats[0] != 2000 {
		t.Fatalf("expected [2000], got %v", cats)
	}
}

func TestArrSeedCategoriesForItem_Unknown(t *testing.T) {
	cats := arrSeedCategoriesForItem(&ArrSeedMediaItem{ItemType: "unknown"})
	if cats != nil {
		t.Fatalf("expected nil, got %v", cats)
	}
}

func TestArrSeedCategoriesForItem_Nil(t *testing.T) {
	cats := arrSeedCategoriesForItem(nil)
	if cats != nil {
		t.Fatalf("expected nil, got %v", cats)
	}
}

func TestArrSeedBuildSearchQuery_Episode(t *testing.T) {
	item := &ArrSeedMediaItem{
		ItemType:    "episode",
		ReleaseName: "The.Wire.S04E01.1080p.BluRay.DD5.1.x264-CtrlHD",
		Title:       "The Wire",
	}
	sq := arrSeedBuildSearchQuery(item)

	if sq.query != "The Wire" {
		t.Errorf("query: got %q, want %q", sq.query, "The Wire")
	}
	if sq.season == nil || *sq.season != 4 {
		t.Errorf("season: got %v, want 4", sq.season)
	}
	if sq.episode == nil || *sq.episode != 1 {
		t.Errorf("episode: got %v, want 1", sq.episode)
	}
}

func TestArrSeedBuildSearchQuery_SeasonPack(t *testing.T) {
	item := &ArrSeedMediaItem{
		ItemType:     "season_pack",
		ReleaseName:  "Snapped.S09.720p.x264-P2P",
		Title:        "Snapped",
		SeasonNumber: 9,
	}
	sq := arrSeedBuildSearchQuery(item)

	if sq.query != "Snapped" {
		t.Errorf("query: got %q, want %q", sq.query, "Snapped")
	}
	if sq.season == nil || *sq.season != 9 {
		t.Errorf("season: got %v, want 9", sq.season)
	}
	if sq.episode != nil {
		t.Errorf("episode should be nil for season pack, got %v", sq.episode)
	}
}

func TestArrSeedBuildSearchQuery_SeasonPackUsesItemSeasonNumber(t *testing.T) {
	// Virtual season pack where the synthesized name might not parse well
	item := &ArrSeedMediaItem{
		ItemType:     "season_pack",
		ReleaseName:  "Its.in.the.Game.Madden.NFL.S01.1080p.AMZN.WEB-DL.DDP5.1.H.264-Kitsune",
		Title:        "Its in the Game Madden NFL",
		SeasonNumber: 1,
	}
	sq := arrSeedBuildSearchQuery(item)

	if sq.query != "Its in the Game Madden NFL" {
		t.Errorf("query: got %q, want %q", sq.query, "Its in the Game Madden NFL")
	}
	if sq.season == nil || *sq.season != 1 {
		t.Errorf("season: got %v, want 1", sq.season)
	}
}

func TestArrSeedBuildSearchQuery_Movie(t *testing.T) {
	item := &ArrSeedMediaItem{
		ItemType:    "movie",
		ReleaseName: "Inception.2010.1080p.BluRay.x264-GROUP",
		Title:       "Inception",
	}
	sq := arrSeedBuildSearchQuery(item)

	if sq.query != "Inception" {
		t.Errorf("query: got %q, want %q", sq.query, "Inception")
	}
	if sq.year != 2010 {
		t.Errorf("year: got %d, want 2010", sq.year)
	}
	if sq.season != nil {
		t.Errorf("season should be nil for movie, got %v", sq.season)
	}
	if sq.episode != nil {
		t.Errorf("episode should be nil for movie, got %v", sq.episode)
	}
}

func TestArrSeedBuildSearchQuery_FallsBackToReleaseTitleWhenNoArrTitle(t *testing.T) {
	item := &ArrSeedMediaItem{
		ItemType:    "episode",
		ReleaseName: "Show.Name.S01E05.1080p.WEB-DL-NTb",
		Title:       "", // no ARR title
	}
	sq := arrSeedBuildSearchQuery(item)

	if sq.query != "Show Name" {
		t.Errorf("query: got %q, want %q (parsed from release)", sq.query, "Show Name")
	}
}

func TestArrSeedBuildSearchQuery_FallsBackToReleaseNameWhenNothingParsed(t *testing.T) {
	item := &ArrSeedMediaItem{
		ItemType:    "episode",
		ReleaseName: "weird-name",
		Title:       "",
	}
	sq := arrSeedBuildSearchQuery(item)

	// When parser can't extract title and no ARR title, use full release name
	if sq.query == "" {
		t.Error("query should not be empty")
	}
}
