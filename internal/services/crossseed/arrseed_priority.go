// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package crossseed

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/moistari/rls"

	"github.com/autobrr/qui/internal/models"
)

const (
	ArrSeedPriorityHighScore = 1 // Any item type with score >= cutoff
	ArrSeedPriorityNormal    = 2 // movie or season_pack without high score
	ArrSeedPriorityEpisode   = 3 // episode without high score
)

var (
	// arrSeedSeasonPattern matches S followed by 1-2 digits then a non-digit or end.
	// This avoids matching compact episode notation like S0401 (= S04E01).
	arrSeedSeasonPattern  = regexp.MustCompile(`(?i)\bS\d{1,2}(?:\D|$)`)
	arrSeedEpisodePattern = regexp.MustCompile(`(?i)E\d+`)
)

// ArrSeedIsSeasonPack returns true if the release name is a season pack
// (contains S## followed by a non-digit, and no E##).
// Compact episode notation like S0401 (S04E01) is NOT a season pack.
func ArrSeedIsSeasonPack(releaseName string) bool {
	return arrSeedSeasonPattern.MatchString(releaseName) && !arrSeedEpisodePattern.MatchString(releaseName)
}

var (
	// stripStandardEpisode removes standard episode notation: S01E05, S01E01-E03, S01E01E02
	arrSeedStripStandardEpisode = regexp.MustCompile(`(?i)(S\d{1,2})E\d+(?:[.\-]?E\d+)*`)
	// stripCompactEpisode removes compact episode notation: S0401 → S04
	arrSeedStripCompactEpisode = regexp.MustCompile(`(?i)(S\d{2})\d{2,}`)
)

// ArrSeedSynthesizeSeasonPackName strips episode notation and episode titles from an
// episode release name to produce a season-pack-style name for indexer searching.
func ArrSeedSynthesizeSeasonPackName(episodeName string) string {
	if result := arrSeedSynthesizeWithParser(episodeName); result != "" {
		return result
	}
	return arrSeedSynthesizeWithRegex(episodeName)
}

func arrSeedSynthesizeWithParser(episodeName string) string {
	r := rls.ParseString(episodeName)
	if r.Series == 0 {
		return ""
	}

	tags := r.Tags()

	seriesIdx := -1
	for i, tag := range tags {
		if tag.Is(rls.TagTypeSeries) {
			seriesIdx = i
			break
		}
	}
	if seriesIdx == -1 {
		return ""
	}

	var buf strings.Builder

	for i := 0; i < seriesIdx; i++ {
		fmt.Fprintf(&buf, "%o", tags[i])
	}

	fmt.Fprintf(&buf, "S%02d", r.Series)

	qualityStart := -1
	for i := seriesIdx + 1; i < len(tags); i++ {
		if arrSeedIsQualitySectionTag(tags[i]) {
			qualityStart = i
			break
		}
	}

	if qualityStart == -1 {
		return buf.String()
	}

	if qualityStart > 0 && tags[qualityStart-1].Is(rls.TagTypeDelim) {
		qualityStart--
	} else {
		buf.WriteString(".")
	}

	for i := qualityStart; i < len(tags); i++ {
		fmt.Fprintf(&buf, "%o", tags[i])
	}

	return buf.String()
}

func arrSeedIsQualitySectionTag(tag rls.Tag) bool {
	return tag.Is(
		rls.TagTypeResolution,
		rls.TagTypeSource,
		rls.TagTypeCodec,
		rls.TagTypeAudio,
		rls.TagTypeHDR,
		rls.TagTypeCollection,
		rls.TagTypeChannels,
	)
}

func arrSeedSynthesizeWithRegex(episodeName string) string {
	result := arrSeedStripStandardEpisode.ReplaceAllString(episodeName, "$1")
	if result != episodeName {
		return result
	}
	return arrSeedStripCompactEpisode.ReplaceAllString(episodeName, "$1")
}

// ArrSeedClassifyPriority assigns a priority tier to a media item.
// Any item type with score >= cutoff gets priority 1 (high score).
// Movies and season packs without high score get priority 2.
// Episodes without high score get priority 3.
func ArrSeedClassifyPriority(item *ArrSeedMediaItem, cutoffFormatScore int) int {
	if cutoffFormatScore > 0 && item.CustomFormatScore >= cutoffFormatScore {
		return ArrSeedPriorityHighScore
	}
	if item.ItemType == "movie" || item.ItemType == "season_pack" {
		return ArrSeedPriorityNormal
	}
	return ArrSeedPriorityEpisode
}

// arrSeedFilterAndSortByPriority applies type-based and quality-gate filters,
// then sorts by priority (ascending).
// Type filter: movies always pass, season packs always pass, episodes only if EnableEpisode.
// Quality gate: if EnableHighScoreOnly, only priority 1 items pass a second filter.
func arrSeedFilterAndSortByPriority(items []*ArrSeedMediaItem, settings *models.ArrSeedSettings) []*ArrSeedMediaItem {
	// Pass 1: type filter
	var filtered []*ArrSeedMediaItem
	for _, item := range items {
		switch item.ItemType {
		case "movie", "season_pack":
			filtered = append(filtered, item)
		case "episode":
			if settings.EnableEpisode {
				filtered = append(filtered, item)
			}
		default:
			filtered = append(filtered, item)
		}
	}

	// Pass 2: quality gate
	if settings.EnableHighScoreOnly {
		var gated []*ArrSeedMediaItem
		for _, item := range filtered {
			if item.Priority == ArrSeedPriorityHighScore {
				gated = append(gated, item)
			}
		}
		filtered = gated
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		return filtered[i].Priority < filtered[j].Priority
	})
	return filtered
}

// arrSeedFilterPendingItems applies the same type + quality-gate logic to DB-loaded items.
func arrSeedFilterPendingItems(items []*models.ArrSeedItem, settings *models.ArrSeedSettings) []*models.ArrSeedItem {
	// Pass 1: type filter
	var filtered []*models.ArrSeedItem
	for _, item := range items {
		switch item.ItemType {
		case "movie", "season_pack":
			filtered = append(filtered, item)
		case "episode":
			if settings.EnableEpisode {
				filtered = append(filtered, item)
			}
		default:
			filtered = append(filtered, item)
		}
	}

	// Pass 2: quality gate
	if settings.EnableHighScoreOnly {
		var gated []*models.ArrSeedItem
		for _, item := range filtered {
			if item.Priority == ArrSeedPriorityHighScore {
				gated = append(gated, item)
			}
		}
		filtered = gated
	}

	return filtered
}
