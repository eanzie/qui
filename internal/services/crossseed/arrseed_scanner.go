// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package crossseed

import (
	"context"
	"strings"

	"github.com/rs/zerolog"

	"github.com/autobrr/qui/internal/models"
	"github.com/autobrr/qui/internal/services/arr"
)

// arrSeedResolvedFile pairs an episode file with its resolved release name.
type arrSeedResolvedFile struct {
	ef          arr.SonarrEpisodeFileResponse
	releaseName string
	source      string
}

// arrSeedScanner queries Sonarr/Radarr APIs and builds the list of seedable items.
type arrSeedScanner struct{}

// scanSonarrInstance scans a Sonarr instance for seedable items.
func (sc *arrSeedScanner) scanSonarrInstance(
	ctx context.Context,
	client *arr.Client,
	config *models.ArrSeedInstanceConfig,
	l *zerolog.Logger,
) ([]*ArrSeedMediaItem, error) {
	series, err := client.GetSeries(ctx)
	if err != nil {
		return nil, err
	}

	l.Debug().Int("seriesCount", len(series)).Msg("arrseed: fetched series from Sonarr")

	var items []*ArrSeedMediaItem

	skippedPath := 0
	for idx, s := range series {
		if ctx.Err() != nil {
			return items, ctx.Err()
		}

		if config.ArrDockerPath != "" && !arrSeedPathUnder(s.Path, config.ArrDockerPath) {
			skippedPath++
			l.Debug().Str("title", s.Title).Str("seriesPath", s.Path).Str("arrDockerPath", config.ArrDockerPath).Msg("arrseed: skipping series outside configured path")
			continue
		}

		l.Debug().Int("series", idx+1).Int("total", len(series)).Str("title", s.Title).Int("seriesID", s.ID).Msg("arrseed: fetching episode files")
		episodeFiles, err := client.GetEpisodeFiles(ctx, s.ID)
		if err != nil {
			l.Warn().Err(err).Int("seriesID", s.ID).Str("title", s.Title).Msg("arrseed: failed to get episode files")
			continue
		}
		l.Debug().Int("episodeFiles", len(episodeFiles)).Str("title", s.Title).Msg("arrseed: episode files fetched")

		var historyMap map[int]string
		needsHistory := false
		withScene := 0
		for _, ef := range episodeFiles {
			if ef.SceneName == "" {
				needsHistory = true
			} else {
				withScene++
			}
		}

		l.Debug().Int("withSceneName", withScene).Int("withoutSceneName", len(episodeFiles)-withScene).Str("title", s.Title).Msg("arrseed: scene name availability")

		if needsHistory {
			l.Debug().Int("seriesID", s.ID).Msg("arrseed: fetching series history for fallback release names")
			history, histErr := client.GetSeriesHistory(ctx, s.ID)
			if histErr != nil {
				l.Debug().Err(histErr).Int("seriesID", s.ID).Msg("arrseed: failed to get series history for fallback")
			} else {
				historyMap = make(map[int]string)
				for _, h := range history {
					if h.EpisodeFileID > 0 && h.SourceTitle != "" {
						if _, exists := historyMap[h.EpisodeFileID]; !exists {
							historyMap[h.EpisodeFileID] = h.SourceTitle
						}
					}
				}
				l.Debug().Int("historyEntries", len(historyMap)).Msg("arrseed: history fallback map built")
			}
		}

		skipped := 0
		var resolved []arrSeedResolvedFile
		for _, ef := range episodeFiles {
			releaseName := ef.SceneName
			source := "sceneName"
			if releaseName == "" && historyMap != nil {
				releaseName = historyMap[ef.ID]
				source = "history"
			}
			if releaseName == "" {
				skipped++
				continue
			}
			resolved = append(resolved, arrSeedResolvedFile{ef: ef, releaseName: releaseName, source: source})
		}

		type fileGroup struct {
			files []arrSeedResolvedFile
		}
		groups := make(map[string]*fileGroup)
		var groupOrder []string
		for _, rf := range resolved {
			g, exists := groups[rf.releaseName]
			if !exists {
				g = &fileGroup{}
				groups[rf.releaseName] = g
				groupOrder = append(groupOrder, rf.releaseName)
			}
			g.files = append(g.files, rf)
		}

		var individualEpisodes []arrSeedResolvedFile
		for _, releaseName := range groupOrder {
			g := groups[releaseName]

			if ArrSeedIsSeasonPack(releaseName) {
				items = append(items, sc.buildSeasonPackItem(s, config, g.files, releaseName, l))
			} else {
				individualEpisodes = append(individualEpisodes, g.files...)
			}
		}

		seasonFiles := make(map[int][]arrSeedResolvedFile)
		seasonUniqueGroups := make(map[int]map[string]bool)
		for _, rf := range individualEpisodes {
			sn := rf.ef.SeasonNumber
			if sn == 0 {
				continue
			}
			seasonFiles[sn] = append(seasonFiles[sn], rf)
			if seasonUniqueGroups[sn] == nil {
				seasonUniqueGroups[sn] = make(map[string]bool)
			}
			seasonUniqueGroups[sn][rf.ef.ReleaseGroup] = true
		}

		promotedSeasons := make(map[int]bool)
		for sn, files := range seasonFiles {
			uniqueGroups := seasonUniqueGroups[sn]
			if len(uniqueGroups) != 1 || len(files) < 2 {
				continue
			}
			var group string
			for g := range uniqueGroups {
				group = g
			}
			if group == "" {
				continue
			}

			synthName := ArrSeedSynthesizeSeasonPackName(files[0].releaseName)
			l.Info().
				Str("title", s.Title).
				Int("season", sn).
				Str("releaseGroup", group).
				Int("episodes", len(files)).
				Str("synthesizedName", synthName).
				Msg("arrseed: detected virtual season pack (same group for all episodes)")

			items = append(items, sc.buildSeasonPackItem(s, config, files, synthName, l))
			promotedSeasons[sn] = true
		}

		for _, rf := range individualEpisodes {
			if rf.ef.SeasonNumber != 0 && promotedSeasons[rf.ef.SeasonNumber] {
				continue
			}
			hostPath := arrSeedMapPath(rf.ef.Path, config.ArrDockerPath, config.HostDataPath)

			l.Debug().
				Str("release", rf.releaseName).
				Str("source", rf.source).
				Str("arrPath", rf.ef.Path).
				Str("hostPath", hostPath).
				Int64("size", rf.ef.Size).
				Msg("arrseed: discovered episode")

			items = append(items, &ArrSeedMediaItem{
				ConfigID:        config.ID,
				ArrInstanceType: "sonarr",
				ItemType:        "episode",
				ArrFileID:       rf.ef.ID,
				ReleaseName:     rf.releaseName,
				ArrFilePath:     rf.ef.Path,
				HostFilePath:    hostPath,
				FileSize:        rf.ef.Size,
				Title:           s.Title,
				ExternalIDs: ExternalIDs{
					IMDbID: s.IMDbID,
					TVDbID: s.TVDbID,
				},
				SeriesID:          s.ID,
				QualityProfileID:  s.QualityProfileID,
				CustomFormatScore: rf.ef.CustomFormatScore,
			})
		}
		if skipped > 0 {
			l.Debug().Int("skipped", skipped).Str("title", s.Title).Msg("arrseed: skipped episodes without release name")
		}
	}

	if skippedPath > 0 {
		l.Info().Int("skippedPath", skippedPath).Str("arrDockerPath", config.ArrDockerPath).Msg("arrseed: skipped series outside configured path")
	}
	l.Info().Int("items", len(items)).Msg("arrseed: Sonarr scan complete")
	return items, nil
}

// scanRadarrInstance scans a Radarr instance for seedable items.
func (sc *arrSeedScanner) scanRadarrInstance(
	ctx context.Context,
	client *arr.Client,
	config *models.ArrSeedInstanceConfig,
	l *zerolog.Logger,
) ([]*ArrSeedMediaItem, error) {
	movies, err := client.GetMovies(ctx)
	if err != nil {
		return nil, err
	}

	l.Debug().Int("movieCount", len(movies)).Msg("arrseed: fetched movies from Radarr")

	var items []*ArrSeedMediaItem

	noFile := 0
	noRelease := 0
	skippedPath := 0
	for _, m := range movies {
		if ctx.Err() != nil {
			return items, ctx.Err()
		}

		if !m.HasFile || m.MovieFile == nil {
			noFile++
			continue
		}

		if config.ArrDockerPath != "" && !arrSeedPathUnder(m.Path, config.ArrDockerPath) {
			skippedPath++
			l.Debug().Str("title", m.Title).Str("moviePath", m.Path).Str("arrDockerPath", config.ArrDockerPath).Msg("arrseed: skipping movie outside configured path")
			continue
		}

		releaseName := m.MovieFile.SceneName
		source := "sceneName"

		if releaseName == "" {
			l.Debug().Int("movieID", m.ID).Str("title", m.Title).Msg("arrseed: no sceneName, fetching movie history")
			history, histErr := client.GetMovieHistory(ctx, m.ID)
			if histErr != nil {
				l.Debug().Err(histErr).Int("movieID", m.ID).Msg("arrseed: failed to get movie history for fallback")
			} else {
				for _, h := range history {
					if h.SourceTitle != "" {
						releaseName = h.SourceTitle
						source = "history"
						break
					}
				}
			}
		}

		if releaseName == "" {
			noRelease++
			l.Debug().Str("title", m.Title).Int("movieID", m.ID).Msg("arrseed: skipping movie, no release name found")
			continue
		}

		hostPath := arrSeedMapPath(m.MovieFile.Path, config.ArrDockerPath, config.HostDataPath)

		l.Debug().
			Str("release", releaseName).
			Str("source", source).
			Str("title", m.Title).
			Str("arrPath", m.MovieFile.Path).
			Str("hostPath", hostPath).
			Int64("size", m.MovieFile.Size).
			Msg("arrseed: discovered movie")

		items = append(items, &ArrSeedMediaItem{
			ConfigID:        config.ID,
			ArrInstanceType: "radarr",
			ItemType:        "movie",
			ArrFileID:       m.MovieFile.ID,
			ReleaseName:     releaseName,
			ArrFilePath:     m.MovieFile.Path,
			HostFilePath:    hostPath,
			FileSize:        m.MovieFile.Size,
			Title:           m.Title,
			ExternalIDs: ExternalIDs{
				IMDbID: m.IMDbID,
				TMDbID: m.TMDbID,
			},
			MovieID:           m.ID,
			QualityProfileID:  m.QualityProfileID,
			CustomFormatScore: m.MovieFile.CustomFormatScore,
		})
	}

	l.Info().
		Int("items", len(items)).
		Int("noFile", noFile).
		Int("noReleaseName", noRelease).
		Int("skippedPath", skippedPath).
		Msg("arrseed: Radarr scan complete")
	return items, nil
}

func (sc *arrSeedScanner) buildSeasonPackItem(
	s arr.SonarrSeriesResponse,
	config *models.ArrSeedInstanceConfig,
	files []arrSeedResolvedFile,
	releaseName string,
	l *zerolog.Logger,
) *ArrSeedMediaItem {
	first := files[0]
	var totalSize int64
	maxCFScore := 0
	for _, rf := range files {
		totalSize += rf.ef.Size
		if rf.ef.CustomFormatScore > maxCFScore {
			maxCFScore = rf.ef.CustomFormatScore
		}
	}

	hostPath := arrSeedMapPath(first.ef.Path, config.ArrDockerPath, config.HostDataPath)
	syntheticID := s.ID*10000 + first.ef.SeasonNumber

	hostFiles := make([]HostFile, 0, len(files))
	for _, rf := range files {
		hostFiles = append(hostFiles, HostFile{
			Path: arrSeedMapPath(rf.ef.Path, config.ArrDockerPath, config.HostDataPath),
			Size: rf.ef.Size,
		})
	}

	l.Debug().
		Str("release", releaseName).
		Str("source", first.source).
		Int("fileCount", len(files)).
		Int64("totalSize", totalSize).
		Int("seasonNumber", first.ef.SeasonNumber).
		Msg("arrseed: discovered season pack")

	return &ArrSeedMediaItem{
		ConfigID:        config.ID,
		ArrInstanceType: "sonarr",
		ItemType:        "season_pack",
		ArrFileID:       syntheticID,
		ReleaseName:     releaseName,
		ArrFilePath:     first.ef.Path,
		HostFilePath:    hostPath,
		HostFiles:       hostFiles,
		FileSize:        totalSize,
		Title:           s.Title,
		ExternalIDs: ExternalIDs{
			IMDbID: s.IMDbID,
			TVDbID: s.TVDbID,
		},
		SeasonNumber:      first.ef.SeasonNumber,
		SeriesID:          s.ID,
		QualityProfileID:  s.QualityProfileID,
		CustomFormatScore: maxCFScore,
	}
}

// arrSeedPathUnder reports whether path is under the directory root.
// It ensures a directory boundary so "/mnt/media/tv" does not match "/mnt/media/tv2".
func arrSeedPathUnder(path, root string) bool {
	prefix := strings.TrimRight(root, "/") + "/"
	return strings.HasPrefix(path, prefix) || path == strings.TrimRight(root, "/")
}

// arrSeedMapPath replaces the ARR docker path prefix with the host data path.
func arrSeedMapPath(arrPath, arrDockerPath, hostDataPath string) string {
	if arrDockerPath == "" || hostDataPath == "" {
		return arrPath
	}
	return strings.Replace(arrPath, arrDockerPath, hostDataPath, 1)
}
