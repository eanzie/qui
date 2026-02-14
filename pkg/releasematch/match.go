// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package releasematch

import (
	"github.com/moistari/rls"

	"github.com/autobrr/qui/pkg/stringutils"
)

// MatchOptions controls matching behavior.
type MatchOptions struct {
	// FindIndividualEpisodes allows season packs to match individual episodes.
	// When false, season packs only match season packs and episodes only match episodes.
	FindIndividualEpisodes bool
}

// ReleasesMatch checks if two releases describe the same underlying content using
// fuzzy matching. This is the shared implementation used by both cross-seed and
// ARR Seed matching.
func ReleasesMatch(source, candidate *rls.Release, opts MatchOptions) bool {
	if source == candidate {
		return true
	}

	// Title should match closely but not necessarily exactly.
	// Use punctuation-stripping normalization to handle differences like
	// "Bob's Burgers" vs "Bobs.Burgers" (apostrophes lost in dot notation).
	sourceTitleNorm := stringutils.NormalizeForMatching(source.Title)
	candidateTitleNorm := stringutils.NormalizeForMatching(candidate.Title)

	if sourceTitleNorm == "" || candidateTitleNorm == "" {
		return false
	}

	// Require exact title match after normalization.
	if sourceTitleNorm != candidateTitleNorm {
		return false
	}

	isTV := source.Series > 0 || candidate.Series > 0

	// Artist must match for content with artist metadata (music, 0day scene radio shows, etc.)
	if source.Artist != "" && candidate.Artist != "" {
		sourceArtist := upper(source.Artist)
		candidateArtist := upper(candidate.Artist)
		if sourceArtist != candidateArtist {
			return false
		}
	}

	// Year should match if both are present.
	if source.Year > 0 && candidate.Year > 0 && source.Year != candidate.Year {
		return false
	}

	// For date-based releases (0day scene), require exact date match including month and day.
	if source.Year > 0 && source.Month > 0 && source.Day > 0 &&
		candidate.Year > 0 && candidate.Month > 0 && candidate.Day > 0 {
		if source.Month != candidate.Month || source.Day != candidate.Day {
			return false
		}
	}

	// For non-TV content where rls has inferred a concrete content type (movie, music,
	// audiobook, etc.), require the types to match.
	if !isTV && source.Type != 0 && candidate.Type != 0 && source.Type != candidate.Type {
		return false
	}

	// For TV shows, season and episode structure must match based on settings.
	if source.Series > 0 || candidate.Series > 0 {
		if source.Series > 0 && candidate.Series == 0 {
			return false
		}
		if candidate.Series > 0 && source.Series == 0 {
			return false
		}
		if source.Series > 0 && candidate.Series > 0 && source.Series != candidate.Series {
			return false
		}

		sourceIsPack := source.Series > 0 && source.Episode == 0
		candidateIsPack := candidate.Series > 0 && candidate.Episode == 0

		if !opts.FindIndividualEpisodes {
			// Strict matching: season packs only match season packs, episodes only match episodes
			if sourceIsPack != candidateIsPack {
				return false
			}
			if !sourceIsPack && !candidateIsPack && source.Episode != candidate.Episode {
				return false
			}
		} else {
			// Flexible matching: allow season packs to match individual episodes
			// But individual episodes still need exact episode matching
			if !sourceIsPack && !candidateIsPack && source.Episode != candidate.Episode {
				return false
			}
		}
	}

	// Group tags should match for proper compatibility.
	sourceGroup := upper(source.Group)
	candidateGroup := upper(candidate.Group)
	if sourceGroup != "" {
		if candidateGroup == "" || sourceGroup != candidateGroup {
			return false
		}
	}

	// Site field is used by anime releases where group is in brackets like [SubsPlease].
	sourceSite := upper(source.Site)
	candidateSite := upper(candidate.Site)
	if sourceSite != "" && candidateSite != "" && sourceSite != candidateSite {
		return false
	}

	// Sum field contains the CRC32 checksum for anime releases like [32ECE75A].
	sourceSum := upper(source.Sum)
	candidateSum := upper(candidate.Sum)
	if sourceSum != "" {
		if candidateSum == "" || sourceSum != candidateSum {
			return false
		}
	}

	// Source must be compatible if both are present.
	sourceSource := NormalizeSource(source.Source)
	candidateSource := NormalizeSource(candidate.Source)
	if !SourcesCompatible(sourceSource, candidateSource) {
		return false
	}

	// Resolution must match (1080p vs 2160p are different files).
	// Exception: empty resolution is allowed to match SD resolutions (480p, 576p, SD).
	sourceRes := upper(source.Resolution)
	candidateRes := upper(candidate.Resolution)
	if sourceRes != candidateRes {
		if !SDFallbackAllowed(sourceRes, candidateRes) {
			return false
		}
	}

	// Collection must match if either is present (NF vs AMZN vs Criterion are different sources).
	sourceCollection := upper(source.Collection)
	candidateCollection := upper(candidate.Collection)
	if sourceCollection != candidateCollection {
		return false
	}

	// Codec must match if both are present (AVC vs HEVC produce different files).
	if len(source.Codec) > 0 && len(candidate.Codec) > 0 {
		sourceCodec := JoinNormalizedCodecSlice(source.Codec)
		candidateCodec := JoinNormalizedCodecSlice(candidate.Codec)
		if sourceCodec != candidateCodec {
			return false
		}
	}

	// HDR must match if either is present (HDR vs SDR are different encodes).
	sourceHDR := JoinNormalizedSlice(source.HDR)
	candidateHDR := JoinNormalizedSlice(candidate.HDR)
	if sourceHDR != candidateHDR {
		return false
	}

	// Cut must match if both are present (Theatrical vs Extended are different versions).
	if len(source.Cut) > 0 && len(candidate.Cut) > 0 {
		sourceCut := JoinNormalizedSlice(source.Cut)
		candidateCut := JoinNormalizedSlice(candidate.Cut)
		if sourceCut != candidateCut {
			return false
		}
	}

	// Edition must match if both are present (Remastered vs Original are different).
	if len(source.Edition) > 0 && len(candidate.Edition) > 0 {
		sourceEdition := JoinNormalizedSlice(source.Edition)
		candidateEdition := JoinNormalizedSlice(candidate.Edition)
		if sourceEdition != candidateEdition {
			return false
		}
	}

	// Language must match (FRENCH vs ENGLISH are different audio/subs).
	// Exception: empty language is treated as equivalent to ENGLISH.
	sourceLanguage := JoinNormalizedSlice(source.Language)
	candidateLanguage := JoinNormalizedSlice(candidate.Language)
	if sourceLanguage != candidateLanguage {
		isEnglishOrEmpty := func(lang string) bool {
			return lang == "" || lang == "ENGLISH"
		}
		if !(isEnglishOrEmpty(sourceLanguage) && isEnglishOrEmpty(candidateLanguage)) {
			return false
		}
	}

	// Version must match if both are present (v2 often has different files than v1).
	sourceVersion := upper(source.Version)
	candidateVersion := upper(candidate.Version)
	if sourceVersion != "" && candidateVersion != "" && sourceVersion != candidateVersion {
		return false
	}

	// Disc must match if both are present (Disc1 vs Disc2 are different content).
	sourceDisc := upper(source.Disc)
	candidateDisc := upper(candidate.Disc)
	if sourceDisc != "" && candidateDisc != "" && sourceDisc != candidateDisc {
		return false
	}

	// Platform must match if both are present (Windows vs macOS are different binaries).
	sourcePlatform := upper(source.Platform)
	candidatePlatform := upper(candidate.Platform)
	if sourcePlatform != "" && candidatePlatform != "" && sourcePlatform != candidatePlatform {
		return false
	}

	// Architecture must match if both are present (x64 vs x86 are different binaries).
	sourceArch := upper(source.Arch)
	candidateArch := upper(candidate.Arch)
	if sourceArch != "" && candidateArch != "" && sourceArch != candidateArch {
		return false
	}

	// Certain variant tags must match for safe matching.
	if compatible, _ := CheckVariantsCompatible(source, candidate); !compatible {
		return false
	}

	return true
}
