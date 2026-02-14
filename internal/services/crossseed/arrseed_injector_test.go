// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package crossseed

import (
	"testing"
)

// --- Build Add Options ---

func TestArrSeedBuildAddOptions_Defaults(t *testing.T) {
	injOpts := &arrSeedInjectionOptions{
		StartPaused: false,
	}
	opts := arrSeedBuildAddOptions("", injOpts, "/save/path")

	if opts["autoTMM"] != "false" {
		t.Errorf("autoTMM: got %q", opts["autoTMM"])
	}
	if opts["contentLayout"] != "Original" {
		t.Errorf("contentLayout: got %q", opts["contentLayout"])
	}
	if opts["savepath"] != "/save/path" {
		t.Errorf("savepath: got %q", opts["savepath"])
	}
	if _, ok := opts["category"]; ok {
		t.Errorf("expected no category when empty, got %q", opts["category"])
	}
	if _, ok := opts["paused"]; ok {
		t.Errorf("expected no paused when StartPaused=false")
	}
}

func TestArrSeedBuildAddOptions_WithCategory(t *testing.T) {
	injOpts := &arrSeedInjectionOptions{}
	opts := arrSeedBuildAddOptions("TV.cross", injOpts, "/save/path")

	if opts["category"] != "TV.cross" {
		t.Errorf("category: got %q, want %q", opts["category"], "TV.cross")
	}
}

func TestArrSeedBuildAddOptions_StartPaused(t *testing.T) {
	injOpts := &arrSeedInjectionOptions{
		StartPaused: true,
	}
	opts := arrSeedBuildAddOptions("", injOpts, "/save/path")

	if opts["paused"] != "true" {
		t.Errorf("paused: got %q, want %q", opts["paused"], "true")
	}
	if opts["stopped"] != "true" {
		t.Errorf("stopped: got %q, want %q", opts["stopped"], "true")
	}
}

func TestArrSeedBuildAddOptions_WithTags(t *testing.T) {
	injOpts := &arrSeedInjectionOptions{
		Tags: []string{"cross-seed", "automated"},
	}
	opts := arrSeedBuildAddOptions("", injOpts, "/save/path")

	if opts["tags"] != "cross-seed,automated" {
		t.Errorf("tags: got %q, want %q", opts["tags"], "cross-seed,automated")
	}
}

func TestArrSeedBuildAddOptions_EmptyTags(t *testing.T) {
	injOpts := &arrSeedInjectionOptions{
		Tags: []string{},
	}
	opts := arrSeedBuildAddOptions("", injOpts, "/save/path")

	if _, ok := opts["tags"]; ok {
		t.Errorf("expected no tags key for empty tags slice")
	}
}

// --- File Filtering ---

func TestArrSeedShouldIgnoreFile_Extensions(t *testing.T) {
	tests := []struct {
		path   string
		ignore bool
	}{
		// Ignored extensions
		{"MovieName/movie.nfo", true},
		{"MovieName/movie.srt", true},
		{"MovieName/movie.sub", true},
		{"MovieName/movie.idx", true},
		{"MovieName/movie.ass", true},
		{"MovieName/movie.ssa", true},
		{"MovieName/movie.sup", true},
		{"MovieName/movie.vtt", true},
		{"MovieName/movie.txt", true},
		{"MovieName/movie.srr", true},

		// Case insensitive
		{"MovieName/movie.NFO", true},
		{"MovieName/movie.SRT", true},
		{"MovieName/movie.Txt", true},

		// Not ignored — media files
		{"MovieName/movie.mkv", false},
		{"MovieName/movie.mp4", false},
		{"MovieName/movie.avi", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := arrSeedShouldIgnoreFile(tt.path); got != tt.ignore {
				t.Errorf("arrSeedShouldIgnoreFile(%q) = %v, want %v", tt.path, got, tt.ignore)
			}
		})
	}
}

func TestArrSeedShouldIgnoreFile_PathKeywords(t *testing.T) {
	tests := []struct {
		path   string
		ignore bool
	}{
		{"MovieName/Sample/movie.mkv", true},
		{"MovieName/sample/movie.mkv", true},
		{"MovieName/!Sample/movie.mkv", true},
		{"MovieName/Proof/proof.jpg", true},
		{"MovieName/Extras/featurette.mkv", true},
		{"MovieName/Bonus/bonus.mkv", true},
		{"MovieName/Trailer/trailer.mkv", true},
		{"MovieName/Featurette/behind.mkv", true},

		// Not ignored — normal media paths
		{"MovieName/movie.mkv", false},
		{"MovieName/Season 1/episode.mkv", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := arrSeedShouldIgnoreFile(tt.path); got != tt.ignore {
				t.Errorf("arrSeedShouldIgnoreFile(%q) = %v, want %v", tt.path, got, tt.ignore)
			}
		})
	}
}

func TestArrSeedIsArchiveTorrent(t *testing.T) {
	tests := []struct {
		name    string
		files   []arrSeedTorrentFile
		archive bool
	}{
		{
			name: "single mkv file",
			files: []arrSeedTorrentFile{
				{Path: "Movie/movie.mkv", Size: 5000000000},
			},
			archive: false,
		},
		{
			name: "single rar file",
			files: []arrSeedTorrentFile{
				{Path: "Movie/movie.rar", Size: 5000000000},
			},
			archive: true,
		},
		{
			name: "rar with nfo",
			files: []arrSeedTorrentFile{
				{Path: "Movie/movie.rar", Size: 5000000000},
				{Path: "Movie/movie.nfo", Size: 1000},
			},
			archive: true,
		},
		{
			name: "mkv with nfo (not archive)",
			files: []arrSeedTorrentFile{
				{Path: "Movie/movie.mkv", Size: 5000000000},
				{Path: "Movie/movie.nfo", Size: 1000},
			},
			archive: false,
		},
		{
			name: "multi-part rar (r00 series)",
			files: []arrSeedTorrentFile{
				{Path: "Movie/movie.rar", Size: 100000000},
				{Path: "Movie/movie.r00", Size: 100000000},
				{Path: "Movie/movie.r01", Size: 100000000},
				{Path: "Movie/movie.r02", Size: 50000000},
			},
			archive: true,
		},
		{
			name: "zip archive",
			files: []arrSeedTorrentFile{
				{Path: "Movie/movie.zip", Size: 5000000000},
			},
			archive: true,
		},
		{
			name: "7z archive",
			files: []arrSeedTorrentFile{
				{Path: "Movie/movie.7z", Size: 5000000000},
			},
			archive: true,
		},
		{
			name: "tar.gz archive",
			files: []arrSeedTorrentFile{
				{Path: "Movie/movie.tar.gz", Size: 5000000000},
			},
			archive: true,
		},
		{
			name: "tar.xz archive",
			files: []arrSeedTorrentFile{
				{Path: "Movie/movie.tar.xz", Size: 5000000000},
			},
			archive: true,
		},
		{
			name: "tar.bz2 archive",
			files: []arrSeedTorrentFile{
				{Path: "Movie/movie.tar.bz2", Size: 5000000000},
			},
			archive: true,
		},
		{
			name:    "empty file list",
			files:   []arrSeedTorrentFile{},
			archive: false,
		},
		{
			name: "only ignored files",
			files: []arrSeedTorrentFile{
				{Path: "Movie/movie.nfo", Size: 1000},
				{Path: "Movie/movie.srt", Size: 50000},
			},
			archive: false,
		},
		{
			name: "mkv larger than rar — not archive",
			files: []arrSeedTorrentFile{
				{Path: "Movie/movie.mkv", Size: 5000000000},
				{Path: "Movie/extras.rar", Size: 100000},
			},
			archive: false,
		},
		{
			name: "rar larger than mkv — archive",
			files: []arrSeedTorrentFile{
				{Path: "Movie/movie.rar", Size: 5000000000},
				{Path: "Movie/sample.mkv", Size: 100000},
			},
			archive: true,
		},
		{
			name: "case insensitive RAR",
			files: []arrSeedTorrentFile{
				{Path: "Movie/MOVIE.RAR", Size: 5000000000},
			},
			archive: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := arrSeedIsArchiveTorrent(tt.files); got != tt.archive {
				t.Errorf("arrSeedIsArchiveTorrent() = %v, want %v", got, tt.archive)
			}
		})
	}
}

func TestBuildArrSeedArchiveExtSet(t *testing.T) {
	exts := buildArrSeedArchiveExtSet()

	// Core extensions
	for _, ext := range []string{".rar", ".zip", ".7z"} {
		if !exts[ext] {
			t.Errorf("expected %q in archive extensions", ext)
		}
	}

	// Multi-part rar extensions
	for _, ext := range []string{".r00", ".r01", ".r09", ".r10", ".r50", ".r99"} {
		if !exts[ext] {
			t.Errorf("expected %q in archive extensions", ext)
		}
	}

	// Non-archive
	for _, ext := range []string{".mkv", ".mp4", ".avi", ".nfo"} {
		if exts[ext] {
			t.Errorf("did not expect %q in archive extensions", ext)
		}
	}
}
