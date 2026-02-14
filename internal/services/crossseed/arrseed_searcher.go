// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package crossseed

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/moistari/rls"
	"github.com/rs/zerolog"

	"github.com/autobrr/qui/internal/services/jackett"
)

// arrSeedSearchScope is the cache scope used for arr-seed searches.
const arrSeedSearchScope = "arr-seed"

// arrSeedSearcher searches indexers for release names.
type arrSeedSearcher struct {
	jackettService *jackett.Service
}

// searchRelease searches all configured indexers for a media item using
// structured Torznab parameters (title, season, episode, IDs) rather than
// blasting the full release name as the query.
func (s *arrSeedSearcher) searchRelease(
	ctx context.Context,
	item *ArrSeedMediaItem,
	l *zerolog.Logger,
) ([]jackett.SearchResult, error) {
	if s.jackettService == nil {
		return nil, nil
	}

	categories := arrSeedCategoriesForItem(item)
	sq := arrSeedBuildSearchQuery(item)

	resultsCh := make(chan *jackett.SearchResponse, 1)
	errCh := make(chan error, 1)

	req := &jackett.TorznabSearchRequest{
		Query:       sq.query,
		ReleaseName: item.ReleaseName,
		Categories:  categories,
		Season:      sq.season,
		Episode:     sq.episode,
		Year:        sq.year,
		OnAllComplete: func(response *jackett.SearchResponse, err error) {
			if err != nil {
				errCh <- err
				return
			}
			resultsCh <- response
		},
	}

	hasIDs := false
	if item.ExternalIDs.IMDbID != "" {
		req.IMDbID = item.ExternalIDs.IMDbID
		hasIDs = true
	}
	if item.ExternalIDs.TVDbID > 0 {
		req.TVDbID = strconv.Itoa(item.ExternalIDs.TVDbID)
		hasIDs = true
	}
	if item.ExternalIDs.TMDbID > 0 {
		req.TMDbID = item.ExternalIDs.TMDbID
		hasIDs = true
	}
	if hasIDs {
		req.OmitQueryForIDs = true
	}

	l.Debug().
		Str("release", item.ReleaseName).
		Str("query", sq.query).
		Bool("hasSeason", sq.season != nil).
		Bool("hasEpisode", sq.episode != nil).
		Int("year", sq.year).
		Bool("hasIDs", hasIDs).
		Str("itemType", item.ItemType).
		Msg("arrseed: searching indexers")

	searchCtx := jackett.WithSearchPriority(ctx, jackett.RateLimitPriorityBackground)

	if err := s.jackettService.SearchWithScope(searchCtx, req, arrSeedSearchScope); err != nil {
		return nil, fmt.Errorf("search indexers: %w", err)
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case err := <-errCh:
		return nil, err
	case resp := <-resultsCh:
		if resp == nil {
			return nil, nil
		}

		l.Debug().
			Str("release", item.ReleaseName).
			Int("results", len(resp.Results)).
			Msg("arrseed: search complete")

		return resp.Results, nil
	}
}

type arrSeedSearchQuery struct {
	query   string
	season  *int
	episode *int
	year    int
}

func arrSeedBuildSearchQuery(item *ArrSeedMediaItem) arrSeedSearchQuery {
	r := rls.ParseString(item.ReleaseName)

	query := strings.TrimSpace(item.Title)
	if query == "" {
		query = strings.TrimSpace(r.Title)
	}
	if query == "" {
		query = item.ReleaseName
	}

	var sq arrSeedSearchQuery
	sq.query = query

	switch item.ItemType {
	case "episode":
		if r.Series > 0 {
			s := r.Series
			sq.season = &s
		}
		if r.Episode > 0 {
			e := r.Episode
			sq.episode = &e
		}
	case "season_pack":
		if item.SeasonNumber > 0 {
			s := item.SeasonNumber
			sq.season = &s
		} else if r.Series > 0 {
			s := r.Series
			sq.season = &s
		}
	case "movie":
		sq.year = r.Year
	}

	return sq
}

func arrSeedCategoriesForItem(item *ArrSeedMediaItem) []int {
	if item == nil {
		return nil
	}
	switch item.ItemType {
	case "episode":
		return []int{5000}
	case "season_pack":
		return []int{5000}
	case "movie":
		return []int{2000}
	default:
		return nil
	}
}
