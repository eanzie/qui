// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package crossseed

// ArrSeedMediaItem represents a single media file discovered from an ARR instance.
type ArrSeedMediaItem struct {
	ConfigID          int
	ArrInstanceType   string // "sonarr" or "radarr"
	ItemType          string // "episode", "movie", or "season_pack"
	ArrFileID         int    // Episode file ID or movie file ID
	ReleaseName       string // sceneName or history sourceTitle
	ArrFilePath       string // Docker path from ARR API
	HostFilePath      string // Mapped host path (first file for season packs)
	HostFiles         []HostFile // All files for season packs (nil for single-file items)
	FileSize          int64
	Title             string // Series/movie title for display
	ExternalIDs       ExternalIDs
	Priority          int // 1=season-pack-high-score, 2=season-pack, 3=episode
	SeasonNumber      int // For season packs
	SeriesID          int // Sonarr series ID (for post-seed actions)
	MovieID           int // Radarr movie ID (for post-seed actions)
	CustomFormatScore int // From Sonarr episode file
	QualityProfileID  int // From Sonarr series
	MinFormatScore    int // From quality profile (minimum custom format score threshold)
}

// HostFile represents a single file on disk within a season pack.
type HostFile struct {
	Path string
	Size int64
}

// ExternalIDs holds external identifiers for a media item.
type ExternalIDs struct {
	IMDbID string
	TMDbID int
	TVDbID int
}

// ArrSeedScanProgress represents the live progress of a running scan.
type ArrSeedScanProgress struct {
	RunID          int64  `json:"runId"`
	ConfigID       int    `json:"configId"`
	Status         string `json:"status"`
	Phase          string `json:"phase"`
	ItemsTotal     int    `json:"itemsTotal"`
	ItemsProcessed int    `json:"itemsProcessed"`
	MatchesFound   int    `json:"matchesFound"`
	TorrentsAdded  int    `json:"torrentsAdded"`
	CurrentItem    string `json:"currentItem,omitempty"`
}

// arrSeedInjectionOptions holds shared cross-seed settings needed for injection.
type arrSeedInjectionOptions struct {
	StartPaused                  bool
	Tags                         []string
	EnableSeasonPackUpgrade      bool
	SizeMismatchTolerancePercent float64
}

// ArrSeedInjectResult contains the result of an injection attempt.
type ArrSeedInjectResult struct {
	Success          bool
	TorrentHash      string
	ErrorMessage     string
	IsPartialUpgrade bool
	UnmatchedCount   int
}
