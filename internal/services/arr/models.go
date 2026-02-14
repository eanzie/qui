// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package arr

import (
	"github.com/autobrr/qui/internal/models"
)

// SystemStatusResponse represents the response from /api/v3/system/status (both Sonarr and Radarr)
type SystemStatusResponse struct {
	AppName string `json:"appName"`
	Version string `json:"version"`
}

// SonarrParseResponse represents the response from Sonarr's /api/v3/parse endpoint
type SonarrParseResponse struct {
	Title             string                   `json:"title"`
	ParsedEpisodeInfo *SonarrParsedEpisodeInfo `json:"parsedEpisodeInfo"`
	Series            *SonarrSeries            `json:"series"`
}

// SonarrParsedEpisodeInfo contains parsed episode information from Sonarr
type SonarrParsedEpisodeInfo struct {
	SeriesTitle       string `json:"seriesTitle"`
	SeasonNumber      int    `json:"seasonNumber"`
	EpisodeNumbers    []int  `json:"episodeNumbers"`
	AbsoluteEpisode   int    `json:"absoluteEpisodeNumber"`
	Quality           any    `json:"quality"`
	ReleaseGroup      string `json:"releaseGroup"`
	ReleaseHash       string `json:"releaseHash"`
	IsDaily           bool   `json:"isDaily"`
	IsAbsoluteNumber  bool   `json:"isAbsoluteNumbering"`
	IsPossibleSpecial bool   `json:"isPossibleSpecialEpisode"`
}

// SonarrSeries represents a series in Sonarr (contains external IDs)
type SonarrSeries struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	TVDbID   int    `json:"tvdbId"`
	TVMazeID int    `json:"tvMazeId"`
	TMDbID   int    `json:"tmdbId"`
	IMDbID   string `json:"imdbId"`
}

// RadarrParseResponse represents the response from Radarr's /api/v3/parse endpoint
type RadarrParseResponse struct {
	Title           string                 `json:"title"`
	ParsedMovieInfo *RadarrParsedMovieInfo `json:"parsedMovieInfo"`
	Movie           *RadarrMovie           `json:"movie"`
}

// RadarrParsedMovieInfo contains parsed movie information from Radarr
// Note: parsedMovieInfo can contain IDs even when movie is nil (extracted from release name)
type RadarrParsedMovieInfo struct {
	MovieTitle   string `json:"movieTitle"`
	Year         int    `json:"year"`
	IMDbID       string `json:"imdbId"`
	TMDbID       int    `json:"tmdbId"`
	Quality      any    `json:"quality"`
	ReleaseGroup string `json:"releaseGroup"`
	ReleaseHash  string `json:"releaseHash"`
}

// RadarrMovie represents a movie in Radarr (contains external IDs)
type RadarrMovie struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	TMDbID int    `json:"tmdbId"`
	IMDbID string `json:"imdbId"`
}

// ExtractExternalIDs extracts external IDs from a Sonarr parse response
func (r *SonarrParseResponse) ExtractExternalIDs() *models.ExternalIDs {
	if r.Series == nil {
		return nil
	}

	ids := &models.ExternalIDs{}

	// Extract IDs, treating 0 as "not present"
	if r.Series.TVDbID > 0 {
		ids.TVDbID = r.Series.TVDbID
	}
	if r.Series.TVMazeID > 0 {
		ids.TVMazeID = r.Series.TVMazeID
	}
	if r.Series.TMDbID > 0 {
		ids.TMDbID = r.Series.TMDbID
	}
	if r.Series.IMDbID != "" && r.Series.IMDbID != "0" {
		ids.IMDbID = r.Series.IMDbID
	}

	if ids.IsEmpty() {
		return nil
	}

	return ids
}

// SonarrSeriesResponse represents a series from Sonarr's /api/v3/series endpoint.
type SonarrSeriesResponse struct {
	ID               int    `json:"id"`
	Title            string `json:"title"`
	Path             string `json:"path"`
	TVDbID           int    `json:"tvdbId"`
	IMDbID           string `json:"imdbId"`
	QualityProfileID int    `json:"qualityProfileId"`
}

// SonarrEpisodeFileResponse represents an episode file from Sonarr's /api/v3/episodefile endpoint.
type SonarrEpisodeFileResponse struct {
	ID                int    `json:"id"`
	SeriesID          int    `json:"seriesId"`
	SeasonNumber      int    `json:"seasonNumber"`
	RelativePath      string `json:"relativePath"`
	Path              string `json:"path"`
	Size              int64  `json:"size"`
	SceneName         string `json:"sceneName"`
	ReleaseGroup      string `json:"releaseGroup"`
	CustomFormatScore int    `json:"customFormatScore"`
}

// SonarrEpisodeResponse represents an episode from Sonarr's /api/v3/episode endpoint.
type SonarrEpisodeResponse struct {
	ID            int  `json:"id"`
	SeriesID      int  `json:"seriesId"`
	EpisodeFileID int  `json:"episodeFileId"`
	SeasonNumber  int  `json:"seasonNumber"`
	EpisodeNumber int  `json:"episodeNumber"`
	Monitored     bool `json:"monitored"`
}

// SonarrHistoryRecord represents a history entry from Sonarr's /api/v3/history/series endpoint.
type SonarrHistoryRecord struct {
	ID              int    `json:"id"`
	EpisodeID       int    `json:"episodeId"`
	SeriesID        int    `json:"seriesId"`
	SourceTitle     string `json:"sourceTitle"`
	EventType       string `json:"eventType"`
	EpisodeFileID   int    `json:"episodeFileId,omitempty"`
}

// RadarrMovieResponse represents a movie from Radarr's /api/v3/movie endpoint.
type RadarrMovieResponse struct {
	ID        int                    `json:"id"`
	Title     string                 `json:"title"`
	Path      string                 `json:"path"`
	TMDbID    int                    `json:"tmdbId"`
	IMDbID    string                 `json:"imdbId"`
	HasFile   bool                   `json:"hasFile"`
	MovieFile *RadarrMovieFileInline `json:"movieFile,omitempty"`
}

// RadarrMovieFileInline represents the inline movie file in a Radarr movie response.
type RadarrMovieFileInline struct {
	ID           int    `json:"id"`
	RelativePath string `json:"relativePath"`
	Path         string `json:"path"`
	Size         int64  `json:"size"`
	SceneName    string `json:"sceneName"`
	ReleaseGroup string `json:"releaseGroup"`
}

// RadarrHistoryRecord represents a history entry from Radarr's /api/v3/history/movie endpoint.
type RadarrHistoryRecord struct {
	ID          int    `json:"id"`
	MovieID     int    `json:"movieId"`
	SourceTitle string `json:"sourceTitle"`
	EventType   string `json:"eventType"`
}

// QualityProfileResponse represents a quality profile from /api/v3/qualityprofile.
type QualityProfileResponse struct {
	ID                 int    `json:"id"`
	Name               string `json:"name"`
	MinFormatScore     int    `json:"minFormatScore"`
	CutoffFormatScore  int    `json:"cutoffFormatScore"`
	UpgradeAllowed     bool   `json:"upgradeAllowed"`
}

// TagResponse represents a tag from /api/v3/tag.
type TagResponse struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

// CommandResponse represents a Sonarr/Radarr command response.
type CommandResponse struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"` // "queued", "started", "completed", "failed"
}

// ExtractExternalIDs extracts external IDs from a Radarr parse response
func (r *RadarrParseResponse) ExtractExternalIDs() *models.ExternalIDs {
	ids := &models.ExternalIDs{}

	// First try to get IDs from the matched movie (most reliable)
	if r.Movie != nil {
		if r.Movie.TMDbID > 0 {
			ids.TMDbID = r.Movie.TMDbID
		}
		if r.Movie.IMDbID != "" && r.Movie.IMDbID != "0" {
			ids.IMDbID = r.Movie.IMDbID
		}
	}

	// If movie is nil or missing IDs, try parsedMovieInfo (can have IDs from release name)
	if r.ParsedMovieInfo != nil {
		if ids.TMDbID == 0 && r.ParsedMovieInfo.TMDbID > 0 {
			ids.TMDbID = r.ParsedMovieInfo.TMDbID
		}
		if ids.IMDbID == "" && r.ParsedMovieInfo.IMDbID != "" && r.ParsedMovieInfo.IMDbID != "0" {
			ids.IMDbID = r.ParsedMovieInfo.IMDbID
		}
	}

	if ids.IsEmpty() {
		return nil
	}

	return ids
}
