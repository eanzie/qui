// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package crossseed

import (
	"testing"
)

// --- arrSeedShouldIgnoreFile false positives ---

// BUG: arrSeedShouldIgnoreFile uses strings.Contains on the full path
// for keyword matching. This causes false positives when the keyword
// appears in a show/movie title.
func TestArrSeedShouldIgnoreFile_FalsePositives(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		ignore        bool
		isFalseIgnore bool // true if this case demonstrates a false positive bug
	}{
		{
			name:          "sample in show title",
			path:          "Trailer.Park.Boys.S01E01/ep.mkv",
			ignore:        true,
			isFalseIgnore: true, // BUG: "trailer" in title triggers ignore
		},
		{
			name:          "bonus in movie title",
			path:          "The.Bonus.Army.2024/movie.mkv",
			ignore:        true,
			isFalseIgnore: true, // BUG: "bonus" in title triggers ignore
		},
		{
			name:          "proof in movie title",
			path:          "Burden.of.Proof.S01E01/ep.mkv",
			ignore:        true,
			isFalseIgnore: true, // BUG: "proof" in title triggers ignore
		},
		{
			name:   "legitimate Sample directory",
			path:   "Movie.2024/Sample/sample.mkv",
			ignore: true,
		},
		{
			name:   "legitimate Extras directory",
			path:   "Movie.2024/Extras/behind_the_scenes.mkv",
			ignore: true,
		},
		{
			name:   "clean path no keywords",
			path:   "Movie.2024/Movie.2024.1080p.mkv",
			ignore: false,
		},
		{
			name:   "extension check case sensitivity",
			path:   "Movie/Movie.Srt",
			ignore: true,
		},
		{
			name:   "tar.gz compound extension",
			path:   "Archive/file.tar.gz",
			ignore: false, // tar.gz is not an ignored extension (it's an archive ext checked elsewhere)
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := arrSeedShouldIgnoreFile(tt.path)
			if got != tt.ignore {
				t.Errorf("arrSeedShouldIgnoreFile(%q) = %v, want %v", tt.path, got, tt.ignore)
			}
			if tt.isFalseIgnore {
				t.Logf("BUG: %q is incorrectly ignored because keyword appears in title, not a directory", tt.path)
			}
		})
	}
}

// --- arrSeedBuildAddOptions ---

func TestArrSeedBuildAddOptions_IncludesRootFolderFalse(t *testing.T) {
	opts := arrSeedBuildAddOptions("", &arrSeedInjectionOptions{}, "/save")
	if opts["root_folder"] != "false" {
		t.Errorf("root_folder: got %q, want %q", opts["root_folder"], "false")
	}
}

func TestArrSeedBuildAddOptions_AutoTMMAlwaysFalse(t *testing.T) {
	opts := arrSeedBuildAddOptions("cat", &arrSeedInjectionOptions{StartPaused: true, Tags: []string{"t1"}}, "/save")
	if opts["autoTMM"] != "false" {
		t.Errorf("autoTMM: got %q, want %q", opts["autoTMM"], "false")
	}
}

func TestArrSeedBuildAddOptions_ContentLayoutOriginal(t *testing.T) {
	opts := arrSeedBuildAddOptions("", &arrSeedInjectionOptions{}, "/save")
	if opts["contentLayout"] != "Original" {
		t.Errorf("contentLayout: got %q, want %q", opts["contentLayout"], "Original")
	}
}

func TestArrSeedBuildAddOptions_AllOptions(t *testing.T) {
	opts := arrSeedBuildAddOptions("TV.cross", &arrSeedInjectionOptions{
		StartPaused: true,
		Tags:        []string{"cross-seed", "arr-seed"},
	}, "/mnt/save")

	expected := map[string]string{
		"autoTMM":       "false",
		"contentLayout": "Original",
		"root_folder":   "false",
		"savepath":      "/mnt/save",
		"category":      "TV.cross",
		"tags":          "cross-seed,arr-seed",
		"paused":        "true",
		"stopped":       "true",
	}
	for k, v := range expected {
		if opts[k] != v {
			t.Errorf("%s: got %q, want %q", k, opts[k], v)
		}
	}
}

// --- arrSeedIsArchiveTorrent edge cases ---

func TestArrSeedIsArchiveTorrent_TiedSizes(t *testing.T) {
	// When a .rar and .mkv have the same size, the last one in the list
	// wins because > (not >=) is used. This means order matters.
	files := []arrSeedTorrentFile{
		{Path: "Movie/movie.rar", Size: 5000000000},
		{Path: "Movie/movie.mkv", Size: 5000000000},
	}
	result := arrSeedIsArchiveTorrent(files)
	// mkv comes second with same size, but > means it doesn't replace rar
	// So the rar stays as "largest" since it was first with that size
	if !result {
		t.Error("expected archive=true when rar is found first with tied size")
	}
}

func TestArrSeedIsArchiveTorrent_TiedSizesMkvFirst(t *testing.T) {
	// Reverse order: mkv first, rar second with same size
	files := []arrSeedTorrentFile{
		{Path: "Movie/movie.mkv", Size: 5000000000},
		{Path: "Movie/movie.rar", Size: 5000000000},
	}
	result := arrSeedIsArchiveTorrent(files)
	// rar comes second but size is NOT > mkv, so mkv stays as largest
	if result {
		t.Error("expected archive=false when mkv was seen first with tied size")
	}
}

func TestArrSeedIsArchiveTorrent_AllIgnoredFiles(t *testing.T) {
	files := []arrSeedTorrentFile{
		{Path: "Movie/movie.nfo", Size: 1000},
		{Path: "Movie/movie.srt", Size: 50000},
		{Path: "Movie/movie.txt", Size: 500},
	}
	result := arrSeedIsArchiveTorrent(files)
	if result {
		t.Error("expected archive=false when all files are ignored")
	}
}

func TestArrSeedIsArchiveTorrent_NilFiles(t *testing.T) {
	result := arrSeedIsArchiveTorrent(nil)
	if result {
		t.Error("expected archive=false for nil file list")
	}
}

// --- arrSeedMapPath edge cases ---

func TestArrSeedMapPath_TrailingSlashOnDockerPath(t *testing.T) {
	got := arrSeedMapPath("/mnt/tv/Show/ep.mkv", "/mnt/tv/", "/data/tv/")
	// strings.Replace will replace "/mnt/tv/" but the path has "/mnt/tv/Show"
	// so it will match and replace correctly
	want := "/data/tv/Show/ep.mkv"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestArrSeedMapPath_OverlappingPrefixes(t *testing.T) {
	// Docker path is a substring of the host path
	got := arrSeedMapPath("/media/tv/Show/ep.mkv", "/media", "/media/data")
	want := "/media/data/tv/Show/ep.mkv"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestArrSeedMapPath_IdenticalPaths(t *testing.T) {
	got := arrSeedMapPath("/data/tv/Show/ep.mkv", "/data/tv", "/data/tv")
	if got != "/data/tv/Show/ep.mkv" {
		t.Errorf("expected same path when docker and host paths match, got %q", got)
	}
}

func TestArrSeedMapPath_OnlyDockerPathEmpty(t *testing.T) {
	got := arrSeedMapPath("/data/tv/ep.mkv", "", "/host")
	if got != "/data/tv/ep.mkv" {
		t.Errorf("expected original path when docker path empty, got %q", got)
	}
}

func TestArrSeedMapPath_OnlyHostPathEmpty(t *testing.T) {
	got := arrSeedMapPath("/data/tv/ep.mkv", "/data", "")
	if got != "/data/tv/ep.mkv" {
		t.Errorf("expected original path when host path empty, got %q", got)
	}
}
