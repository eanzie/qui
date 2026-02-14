// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package releasematch

import (
	"testing"

	"github.com/moistari/rls"
	"github.com/stretchr/testify/require"
)

func TestVariantOverridesReleaseVariants(t *testing.T) {
	release := rls.Release{
		Collection: "IMAX",
		Other:      []string{"HYBRiD REMUX"},
	}

	variants := strictVariantOverrides.releaseVariants(&release)
	_, hasIMAX := variants["IMAX"]
	require.True(t, hasIMAX, "expected IMAX variant to be detected: %#v", variants)
	_, hasHYBRID := variants["HYBRID"]
	require.True(t, hasHYBRID, "expected HYBRID variant to be detected: %#v", variants)

	multiVariant := rls.Release{
		Collection: "IMAX",
		Other:      []string{"HYBRiD"},
	}
	multiVariants := strictVariantOverrides.releaseVariants(&multiVariant)
	require.Len(t, multiVariants, 2, "expected both IMAX and HYBRID variants")
	_, hasIMAX = multiVariants["IMAX"]
	require.True(t, hasIMAX, "expected IMAX variant to be detected for multiVariant: %#v", multiVariants)
	_, hasHYBRID = multiVariants["HYBRID"]
	require.True(t, hasHYBRID, "expected HYBRID variant to be detected for multiVariant: %#v", multiVariants)

	compositeVariant := rls.Release{
		Other: []string{"IMAX.HYBRiD.REMUX"},
	}
	compositeVariants := strictVariantOverrides.releaseVariants(&compositeVariant)
	require.Len(t, compositeVariants, 1, "expected only HYBRID variant from composite entry")
	_, hasHYBRID = compositeVariants["HYBRID"]
	require.True(t, hasHYBRID, "expected HYBRID token to be extracted from composite entry: %#v", compositeVariants)

	tokenEdge := rls.Release{
		Other: []string{"IMAX..HYBRID", ""},
	}
	tokenEdgeVariants := strictVariantOverrides.releaseVariants(&tokenEdge)
	require.Len(t, tokenEdgeVariants, 1, "expected only valid HYBRID token from edge case")
	_, hasHYBRID = tokenEdgeVariants["HYBRID"]
	require.True(t, hasHYBRID, "expected HYBRID token to survive edge tokenization: %#v", tokenEdgeVariants)

	plain := rls.Release{Collection: "", Other: []string{"READNFO"}}
	plainVariants := strictVariantOverrides.releaseVariants(&plain)
	require.Empty(t, plainVariants, "expected no variants")
}

func TestCheckVariantsCompatible_StrictVariants(t *testing.T) {
	base := rls.Release{
		Title:      "The Conjuring Last Rites",
		Year:       2025,
		Source:     "BLURAY",
		Resolution: "1080P",
		Collection: "IMAX",
	}

	nonVariant := rls.Release{
		Title:      base.Title,
		Year:       base.Year,
		Source:     base.Source,
		Resolution: base.Resolution,
	}

	compatible, _ := CheckVariantsCompatible(&base, &nonVariant)
	require.False(t, compatible, "IMAX should not match vanilla release")

	imaxCandidate := nonVariant
	imaxCandidate.Collection = "IMAX"
	compatible, _ = CheckVariantsCompatible(&base, &imaxCandidate)
	require.True(t, compatible, "matching IMAX releases should be compatible")

	hybridCandidate := nonVariant
	hybridCandidate.Other = []string{"HYBRiD"}
	compatible, _ = CheckVariantsCompatible(&nonVariant, &hybridCandidate)
	require.False(t, compatible, "HYBRID variant should not match vanilla release")
	compatible, _ = CheckVariantsCompatible(&hybridCandidate, &nonVariant)
	require.False(t, compatible, "HYBRID mismatch must be symmetric")
}

func TestCheckVariantsCompatible_IMAXVsHybridMismatch(t *testing.T) {
	imaxRelease := rls.Release{
		Title:      "The Conjuring Last Rites",
		Year:       2025,
		Source:     "BLURAY",
		Resolution: "1080P",
		Collection: "IMAX",
	}
	hybridRelease := rls.Release{
		Title:      imaxRelease.Title,
		Year:       imaxRelease.Year,
		Source:     imaxRelease.Source,
		Resolution: imaxRelease.Resolution,
		Other:      []string{"HYBRiD"},
	}

	compatible, _ := CheckVariantsCompatible(&imaxRelease, &hybridRelease)
	require.False(t, compatible, "IMAX should not match HYBRID")
	compatible, _ = CheckVariantsCompatible(&hybridRelease, &imaxRelease)
	require.False(t, compatible, "HYBRID vs IMAX mismatch must be symmetric")
}

func TestCheckVariantsCompatible_REPACKAllowedForSeasonPacks(t *testing.T) {
	seasonPack := rls.Release{
		Title:      "The Show",
		Series:     1,
		Episode:    0,
		Source:     "BLURAY",
		Resolution: "1080P",
		Group:      "GROUP",
	}

	seasonPackRepack := rls.Release{
		Title:      "The Show",
		Series:     1,
		Episode:    0,
		Source:     "BLURAY",
		Resolution: "1080P",
		Group:      "GROUP",
		Other:      []string{"REPACK"},
	}

	compatible, _ := CheckVariantsCompatible(&seasonPack, &seasonPackRepack)
	require.True(t, compatible, "season pack should match REPACK season pack")
	compatible, _ = CheckVariantsCompatible(&seasonPackRepack, &seasonPack)
	require.True(t, compatible, "REPACK season pack should match vanilla season pack")
}

func TestCheckVariantsCompatible_REPACKBlockedForEpisodes(t *testing.T) {
	episode := rls.Release{
		Title:      "The Show",
		Series:     1,
		Episode:    5,
		Source:     "BLURAY",
		Resolution: "1080P",
		Group:      "GROUP",
	}

	episodeRepack := rls.Release{
		Title:      "The Show",
		Series:     1,
		Episode:    5,
		Source:     "BLURAY",
		Resolution: "1080P",
		Group:      "GROUP",
		Other:      []string{"REPACK"},
	}

	compatible, _ := CheckVariantsCompatible(&episode, &episodeRepack)
	require.False(t, compatible, "vanilla episode should NOT match REPACK episode")
	compatible, _ = CheckVariantsCompatible(&episodeRepack, &episode)
	require.False(t, compatible, "REPACK episode should NOT match vanilla episode")

	compatible, _ = CheckVariantsCompatible(&episodeRepack, &episodeRepack)
	require.True(t, compatible, "REPACK episodes should match each other")
}

func TestCheckVariantsCompatible_PROPERBlockedForMovies(t *testing.T) {
	movie := rls.Release{
		Title:      "The Movie",
		Year:       2025,
		Source:     "BLURAY",
		Resolution: "1080P",
		Group:      "GROUP",
	}

	movieProper := rls.Release{
		Title:      "The Movie",
		Year:       2025,
		Source:     "BLURAY",
		Resolution: "1080P",
		Group:      "GROUP",
		Other:      []string{"PROPER"},
	}

	compatible, _ := CheckVariantsCompatible(&movie, &movieProper)
	require.False(t, compatible, "vanilla movie should NOT match PROPER movie")
	compatible, _ = CheckVariantsCompatible(&movieProper, &movie)
	require.False(t, compatible, "PROPER movie should NOT match vanilla movie")
}

func TestCheckVariantsCompatible_IMAXBlockedEvenForSeasonPacks(t *testing.T) {
	seasonPack := rls.Release{
		Title:      "The Show",
		Series:     1,
		Episode:    0,
		Source:     "BLURAY",
		Resolution: "1080P",
		Group:      "GROUP",
	}

	seasonPackIMAX := rls.Release{
		Title:      "The Show",
		Series:     1,
		Episode:    0,
		Source:     "BLURAY",
		Resolution: "1080P",
		Group:      "GROUP",
		Collection: "IMAX",
	}

	compatible, _ := CheckVariantsCompatible(&seasonPack, &seasonPackIMAX)
	require.False(t, compatible, "vanilla season pack should NOT match IMAX season pack")
	compatible, _ = CheckVariantsCompatible(&seasonPackIMAX, &seasonPack)
	require.False(t, compatible, "IMAX season pack should NOT match vanilla season pack")
}
