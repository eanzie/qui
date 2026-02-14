// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package crossseed

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/autobrr/qui/internal/models"
	"github.com/autobrr/qui/internal/services/arr"
)

// arrSeedPostSeedExecutor runs actions after a successful cross-seed injection.
type arrSeedPostSeedExecutor struct{}

// execute runs configured post-seed actions for a successfully seeded item.
func (e *arrSeedPostSeedExecutor) execute(
	ctx context.Context,
	client *arr.Client,
	item *ArrSeedMediaItem,
	config *models.ArrSeedInstanceConfig,
	l *zerolog.Logger,
) {
	if !config.UnmonitorAfterSeed && config.TagAfterSeed == "" {
		return
	}

	switch item.ArrInstanceType {
	case "sonarr":
		if item.SeriesID <= 0 {
			l.Warn().Msg("arrseed: no series ID, skipping post-seed actions")
			return
		}
		e.executeSonarr(ctx, client, item, config, l)
	case "radarr":
		if item.MovieID <= 0 {
			l.Warn().Msg("arrseed: no movie ID, skipping post-seed actions")
			return
		}
		e.executeRadarr(ctx, client, item, config, l)
	}
}

func (e *arrSeedPostSeedExecutor) executeSonarr(
	ctx context.Context,
	client *arr.Client,
	item *ArrSeedMediaItem,
	config *models.ArrSeedInstanceConfig,
	l *zerolog.Logger,
) {
	if config.TagAfterSeed != "" {
		e.tagSeries(ctx, client, item.SeriesID, config.TagAfterSeed, l)
	}

	if !config.UnmonitorAfterSeed {
		return
	}

	switch item.ItemType {
	case "episode":
		e.unmonitorEpisode(ctx, client, item, l)
	case "season_pack":
		e.unmonitorSeason(ctx, client, item, l)
	}
}

func (e *arrSeedPostSeedExecutor) unmonitorEpisode(
	ctx context.Context,
	client *arr.Client,
	item *ArrSeedMediaItem,
	l *zerolog.Logger,
) {
	episodes, err := client.GetEpisodes(ctx, item.SeriesID)
	if err != nil {
		l.Warn().Err(err).Int("seriesID", item.SeriesID).Msg("arrseed: failed to get episodes for unmonitor")
		return
	}

	var episodeID int
	for _, ep := range episodes {
		if ep.EpisodeFileID == item.ArrFileID {
			episodeID = ep.ID
			break
		}
	}
	if episodeID == 0 {
		l.Warn().Int("episodeFileID", item.ArrFileID).Msg("arrseed: could not find episode for file ID")
		return
	}

	if err := client.SetEpisodesMonitored(ctx, []int{episodeID}, false); err != nil {
		l.Warn().Err(err).Int("episodeID", episodeID).Msg("arrseed: failed to unmonitor episode")
		return
	}
	l.Info().Int("episodeID", episodeID).Int("episodeFileID", item.ArrFileID).Msg("arrseed: episode unmonitored")
}

func (e *arrSeedPostSeedExecutor) unmonitorSeason(
	ctx context.Context,
	client *arr.Client,
	item *ArrSeedMediaItem,
	l *zerolog.Logger,
) {
	episodes, err := client.GetEpisodes(ctx, item.SeriesID)
	if err != nil {
		l.Warn().Err(err).Int("seriesID", item.SeriesID).Msg("arrseed: failed to get episodes for season unmonitor")
		return
	}

	var seasonEpisodeIDs []int
	for _, ep := range episodes {
		if ep.SeasonNumber == item.SeasonNumber {
			seasonEpisodeIDs = append(seasonEpisodeIDs, ep.ID)
		}
	}

	if len(seasonEpisodeIDs) > 0 {
		if err := client.SetEpisodesMonitored(ctx, seasonEpisodeIDs, false); err != nil {
			l.Warn().Err(err).Int("season", item.SeasonNumber).Msg("arrseed: failed to unmonitor season episodes")
		} else {
			l.Info().Int("season", item.SeasonNumber).Int("episodes", len(seasonEpisodeIDs)).Msg("arrseed: season episodes unmonitored")
		}
	}

	raw, err := client.GetSeriesByID(ctx, item.SeriesID)
	if err != nil {
		l.Warn().Err(err).Int("seriesID", item.SeriesID).Msg("arrseed: failed to get series for season unmonitor")
		return
	}

	var series map[string]any
	if err := json.Unmarshal(raw, &series); err != nil {
		l.Warn().Err(err).Msg("arrseed: failed to unmarshal series JSON")
		return
	}

	allUnmonitored := arrSeedSetSeasonMonitored(series, item.SeasonNumber, false)

	if allUnmonitored {
		series["monitored"] = false
		l.Info().Int("seriesID", item.SeriesID).Msg("arrseed: all seasons unmonitored, unmonitoring series")
	}

	if err := client.UpdateSeriesFields(ctx, item.SeriesID, series); err != nil {
		l.Warn().Err(err).Int("seriesID", item.SeriesID).Msg("arrseed: failed to update series after season unmonitor")
		return
	}
	l.Info().Int("seriesID", item.SeriesID).Int("season", item.SeasonNumber).Msg("arrseed: season unmonitored")
}

func arrSeedSetSeasonMonitored(series map[string]any, seasonNumber int, monitored bool) bool {
	seasonsRaw, ok := series["seasons"]
	if !ok {
		return false
	}

	seasons, ok := seasonsRaw.([]any)
	if !ok {
		return false
	}

	allUnmonitored := true
	for _, s := range seasons {
		season, ok := s.(map[string]any)
		if !ok {
			continue
		}

		sn, _ := season["seasonNumber"].(float64)
		if int(sn) == seasonNumber {
			season["monitored"] = monitored
		}

		if m, _ := season["monitored"].(bool); m {
			allUnmonitored = false
		}
	}

	return allUnmonitored
}

func (e *arrSeedPostSeedExecutor) tagSeries(
	ctx context.Context,
	client *arr.Client,
	seriesID int,
	tagLabel string,
	l *zerolog.Logger,
) {
	tagID, err := e.resolveOrCreateTag(ctx, client, tagLabel, l)
	if err != nil {
		l.Warn().Err(err).Str("tag", tagLabel).Msg("arrseed: failed to resolve/create tag")
		return
	}

	raw, err := client.GetSeriesByID(ctx, seriesID)
	if err != nil {
		l.Warn().Err(err).Int("seriesID", seriesID).Msg("arrseed: failed to get series for tag update")
		return
	}

	existingTags := arrSeedExtractTagsFromJSON(raw)
	if arrSeedContainsInt(existingTags, tagID) {
		return
	}

	existingTags = append(existingTags, tagID)
	if err := client.UpdateSeriesFields(ctx, seriesID, map[string]any{"tags": existingTags}); err != nil {
		l.Warn().Err(err).Int("seriesID", seriesID).Msg("arrseed: failed to tag series")
		return
	}
	l.Info().Int("seriesID", seriesID).Str("tag", tagLabel).Msg("arrseed: series tagged")
}

func (e *arrSeedPostSeedExecutor) executeRadarr(
	ctx context.Context,
	client *arr.Client,
	item *ArrSeedMediaItem,
	config *models.ArrSeedInstanceConfig,
	l *zerolog.Logger,
) {
	updates := make(map[string]any)

	if config.UnmonitorAfterSeed {
		updates["monitored"] = false
		l.Info().Msg("arrseed: will unmonitor movie after seed")
	}

	if config.TagAfterSeed != "" {
		tagID, err := e.resolveOrCreateTag(ctx, client, config.TagAfterSeed, l)
		if err != nil {
			l.Warn().Err(err).Str("tag", config.TagAfterSeed).Msg("arrseed: failed to resolve/create tag")
		} else {
			raw, err := client.GetMovieByID(ctx, item.MovieID)
			if err != nil {
				l.Warn().Err(err).Int("movieID", item.MovieID).Msg("arrseed: failed to get movie for tag update")
			} else {
				existingTags := arrSeedExtractTagsFromJSON(raw)
				if !arrSeedContainsInt(existingTags, tagID) {
					existingTags = append(existingTags, tagID)
				}
				updates["tags"] = existingTags
			}
		}
	}

	if len(updates) == 0 {
		return
	}

	if err := client.UpdateMovieFields(ctx, item.MovieID, updates); err != nil {
		l.Warn().Err(err).Int("movieID", item.MovieID).Msg("arrseed: failed to update movie")
		return
	}
	l.Info().Int("movieID", item.MovieID).Msg("arrseed: post-seed movie update complete")
}

func (e *arrSeedPostSeedExecutor) resolveOrCreateTag(ctx context.Context, client *arr.Client, label string, l *zerolog.Logger) (int, error) {
	tags, err := client.GetTags(ctx)
	if err != nil {
		return 0, fmt.Errorf("get tags: %w", err)
	}

	for _, t := range tags {
		if t.Label == label {
			l.Debug().Int("tagID", t.ID).Str("label", label).Msg("arrseed: found existing tag")
			return t.ID, nil
		}
	}

	newTag, err := client.CreateTag(ctx, label)
	if err != nil {
		return 0, fmt.Errorf("create tag: %w", err)
	}
	l.Info().Int("tagID", newTag.ID).Str("label", label).Msg("arrseed: created new tag")
	return newTag.ID, nil
}

func arrSeedExtractTagsFromJSON(raw json.RawMessage) []int {
	var obj struct {
		Tags []int `json:"tags"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	return obj.Tags
}

func arrSeedContainsInt(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}
