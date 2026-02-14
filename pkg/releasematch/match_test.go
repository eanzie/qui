// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package releasematch

import (
	"testing"

	"github.com/moistari/rls"
	"github.com/stretchr/testify/require"
)

func parse(name string) *rls.Release {
	r := rls.ParseString(name)
	return &r
}

// --- Identity and basic field tests (ported from arrseed/matcher_test.go) ---

func TestReleasesMatch_IdenticalReleases(t *testing.T) {
	r := parse("The.Wire.S04E01.1080p.BluRay.DD5.1.x264-CtrlHD")
	require.True(t, ReleasesMatch(r, r, MatchOptions{}), "identical pointer should match")
}

func TestReleasesMatch_SameReleaseDifferentTracker(t *testing.T) {
	src := parse("The.Wire.S04E01.1080p.BluRay.DD5.1.x264-CtrlHD")
	cand := parse("The.Wire.S04E01.1080p.BluRay.DD5.1.x264-CtrlHD")
	require.True(t, ReleasesMatch(src, cand, MatchOptions{}), "identical releases should match")
}

func TestReleasesMatch_DifferentResolution(t *testing.T) {
	src := parse("Show.S01E01.1080p.WEB-DL.x264-GROUP")
	cand := parse("Show.S01E01.720p.WEB-DL.x264-GROUP")
	require.False(t, ReleasesMatch(src, cand, MatchOptions{}), "different resolution should not match")
}

func TestReleasesMatch_DifferentSource(t *testing.T) {
	src := parse("Show.S01E01.1080p.BluRay.x264-GROUP")
	cand := parse("Show.S01E01.1080p.WEB-DL.x264-GROUP")
	require.False(t, ReleasesMatch(src, cand, MatchOptions{}), "BluRay vs WEB-DL should not match")
}

func TestReleasesMatch_DifferentGroup(t *testing.T) {
	src := parse("Show.S01E01.1080p.WEB-DL.x264-CtrlHD")
	cand := parse("Show.S01E01.1080p.WEB-DL.x264-NTb")
	require.False(t, ReleasesMatch(src, cand, MatchOptions{}), "different group should not match")
}

func TestReleasesMatch_DifferentCodec(t *testing.T) {
	src := parse("Show.S01E01.1080p.WEB-DL.x264-GROUP")
	cand := parse("Show.S01E01.1080p.WEB-DL.x265-GROUP")
	require.False(t, ReleasesMatch(src, cand, MatchOptions{}), "x264 vs x265 should not match")
}

func TestReleasesMatch_DifferentEpisode(t *testing.T) {
	src := parse("Show.S01E01.1080p.WEB-DL.x264-GROUP")
	cand := parse("Show.S01E02.1080p.WEB-DL.x264-GROUP")
	require.False(t, ReleasesMatch(src, cand, MatchOptions{}), "different episode should not match")
}

func TestReleasesMatch_DifferentSeason(t *testing.T) {
	src := parse("Show.S01E01.1080p.WEB-DL.x264-GROUP")
	cand := parse("Show.S02E01.1080p.WEB-DL.x264-GROUP")
	require.False(t, ReleasesMatch(src, cand, MatchOptions{}), "different season should not match")
}

func TestReleasesMatch_DifferentTitle(t *testing.T) {
	src := parse("Show.A.S01E01.1080p.WEB-DL.x264-GROUP")
	cand := parse("Show.B.S01E01.1080p.WEB-DL.x264-GROUP")
	require.False(t, ReleasesMatch(src, cand, MatchOptions{}), "different title should not match")
}

func TestReleasesMatch_DifferentYear(t *testing.T) {
	src := parse("Movie.2020.1080p.BluRay.x264-GROUP")
	cand := parse("Movie.2021.1080p.BluRay.x264-GROUP")
	require.False(t, ReleasesMatch(src, cand, MatchOptions{}), "different year should not match")
}

func TestReleasesMatch_DifferentHDR(t *testing.T) {
	src := parse("Show.S01E01.2160p.WEB-DL.DV.HDR.x265-GROUP")
	cand := parse("Show.S01E01.2160p.WEB-DL.x265-GROUP")
	require.False(t, ReleasesMatch(src, cand, MatchOptions{}), "HDR vs non-HDR should not match")
}

func TestReleasesMatch_DifferentCollection(t *testing.T) {
	src := parse("Show.S01E01.1080p.AMZN.WEB-DL.x264-GROUP")
	cand := parse("Show.S01E01.1080p.NF.WEB-DL.x264-GROUP")
	require.False(t, ReleasesMatch(src, cand, MatchOptions{}), "AMZN vs NF should not match")
}

func TestReleasesMatch_SeasonPackVsEpisode(t *testing.T) {
	src := parse("Show.S01.1080p.BluRay.x264-GROUP")
	cand := parse("Show.S01E01.1080p.BluRay.x264-GROUP")
	require.False(t, ReleasesMatch(src, cand, MatchOptions{}), "season pack vs episode should not match")
}

func TestReleasesMatch_MovieMatch(t *testing.T) {
	src := parse("Inception.2010.1080p.BluRay.x264-GROUP")
	cand := parse("Inception.2010.1080p.BluRay.x264-GROUP")
	require.True(t, ReleasesMatch(src, cand, MatchOptions{}), "identical movie releases should match")
}

func TestReleasesMatch_EmptyTitleRejects(t *testing.T) {
	src := &rls.Release{Title: "", Resolution: "1080p"}
	cand := &rls.Release{Title: "", Resolution: "1080p"}
	require.False(t, ReleasesMatch(src, cand, MatchOptions{}), "empty titles should not match")
}

func TestReleasesMatch_CandidateNoGroup(t *testing.T) {
	src := parse("Show.S01E01.1080p.WEB-DL.x264-GROUP")
	cand := parse("Show.S01E01.1080p.WEB-DL.x264")
	require.False(t, ReleasesMatch(src, cand, MatchOptions{}), "candidate missing group should not match when source has group")
}

func TestReleasesMatch_SourceNoGroupMatchesAny(t *testing.T) {
	src := &rls.Release{Title: "Show", Series: 1, Episode: 1, Resolution: "1080p", Source: "WEB-DL", Codec: []string{"x264"}}
	cand := parse("Show.S01E01.1080p.WEB-DL.x264-GROUP")
	require.True(t, ReleasesMatch(src, cand, MatchOptions{}), "source without group should match any candidate group")
}

func TestReleasesMatch_MissingSourceBothSides(t *testing.T) {
	src := &rls.Release{Title: "Show", Series: 1, Episode: 1, Resolution: "1080p", Group: "GROUP"}
	cand := parse("Show.S01E01.1080p.x264-GROUP")
	require.True(t, ReleasesMatch(src, cand, MatchOptions{}), "missing source on one side should be permissive")
}

func TestReleasesMatch_MissingCodecBothSides(t *testing.T) {
	src := parse("Show.S01E01.1080p.WEB-DL-GROUP")
	cand := parse("Show.S01E01.1080p.WEB-DL.x264-GROUP")
	require.True(t, ReleasesMatch(src, cand, MatchOptions{}), "missing codec on one side should be permissive")
}

func TestReleasesMatch_SeasonPackMatch(t *testing.T) {
	src := parse("Show.S01.1080p.BluRay.x264-GROUP")
	cand := parse("Show.S01.1080p.BluRay.x264-GROUP")
	require.True(t, ReleasesMatch(src, cand, MatchOptions{}), "identical season packs should match")
}

func TestReleasesMatch_TVSourceVsMovie(t *testing.T) {
	src := parse("Show.S01E01.1080p.WEB-DL.x264-GROUP")
	cand := parse("Show.2021.1080p.WEB-DL.x264-GROUP")
	require.False(t, ReleasesMatch(src, cand, MatchOptions{}), "TV release should not match movie release")
}

// --- Web source compatibility ---

func TestReleasesMatch_WebCompatibility(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		cand     string
		expected bool
	}{
		{"WEB matches WEB-DL", "Show.S01E01.1080p.WEB.x264-GROUP", "Show.S01E01.1080p.WEB-DL.x264-GROUP", true},
		{"WEB matches WEBRip", "Show.S01E01.1080p.WEB.x264-GROUP", "Show.S01E01.1080p.WEBRip.x264-GROUP", true},
		{"WEB-DL does not match WEBRip", "Show.S01E01.1080p.WEB-DL.x264-GROUP", "Show.S01E01.1080p.WEBRip.x264-GROUP", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := parse(tt.source)
			cand := parse(tt.cand)
			got := ReleasesMatch(src, cand, MatchOptions{})
			require.Equalf(t, tt.expected, got, "ReleasesMatch = %v, want %v", got, tt.expected)
		})
	}
}

// --- Codec aliases ---

func TestReleasesMatch_CodecAliases(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		cand     string
		expected bool
	}{
		{"x264 matches H.264", "Show.S01E01.1080p.WEB-DL.x264-GROUP", "Show.S01E01.1080p.WEB-DL.H.264-GROUP", true},
		{"x265 matches HEVC", "Show.S01E01.2160p.WEB-DL.x265-GROUP", "Show.S01E01.2160p.WEB-DL.HEVC-GROUP", true},
		{"x264 does not match x265", "Show.S01E01.1080p.WEB-DL.x264-GROUP", "Show.S01E01.1080p.WEB-DL.x265-GROUP", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := parse(tt.source)
			cand := parse(tt.cand)
			got := ReleasesMatch(src, cand, MatchOptions{})
			require.Equalf(t, tt.expected, got, "ReleasesMatch = %v, want %v", got, tt.expected)
		})
	}
}

// --- Cut, Edition, Language, Version ---

func TestReleasesMatch_DifferentCut(t *testing.T) {
	src := parse("Movie.2020.Extended.1080p.BluRay.x264-GROUP")
	cand := parse("Movie.2020.Theatrical.1080p.BluRay.x264-GROUP")
	require.False(t, ReleasesMatch(src, cand, MatchOptions{}), "Extended vs Theatrical should not match")
}

func TestReleasesMatch_CutOnlyOneSide(t *testing.T) {
	src := parse("Movie.2020.1080p.BluRay.x264-GROUP")
	cand := parse("Movie.2020.Extended.1080p.BluRay.x264-GROUP")
	require.True(t, ReleasesMatch(src, cand, MatchOptions{}), "cut only on one side should be permissive")
}

func TestReleasesMatch_DifferentEdition(t *testing.T) {
	src := &rls.Release{Title: "Movie", Year: 2020, Resolution: "1080p", Source: "BluRay", Codec: []string{"x264"}, Group: "GROUP", Edition: []string{"Remastered"}}
	cand := &rls.Release{Title: "Movie", Year: 2020, Resolution: "1080p", Source: "BluRay", Codec: []string{"x264"}, Group: "GROUP", Edition: []string{"Anniversary"}}
	require.False(t, ReleasesMatch(src, cand, MatchOptions{}), "Remastered vs Anniversary should not match")
}

func TestReleasesMatch_EditionOnlyOneSide(t *testing.T) {
	src := &rls.Release{Title: "Movie", Year: 2020, Resolution: "1080p", Source: "BluRay", Codec: []string{"x264"}, Group: "GROUP"}
	cand := &rls.Release{Title: "Movie", Year: 2020, Resolution: "1080p", Source: "BluRay", Codec: []string{"x264"}, Group: "GROUP", Edition: []string{"Remastered"}}
	require.True(t, ReleasesMatch(src, cand, MatchOptions{}), "edition only on one side should be permissive")
}

func TestReleasesMatch_DifferentLanguage(t *testing.T) {
	src := parse("Movie.2020.FRENCH.1080p.BluRay.x264-GROUP")
	cand := parse("Movie.2020.GERMAN.1080p.BluRay.x264-GROUP")
	require.False(t, ReleasesMatch(src, cand, MatchOptions{}), "FRENCH vs GERMAN should not match")
}

func TestReleasesMatch_LanguageEmptyVsEnglish(t *testing.T) {
	src := parse("Movie.2020.1080p.BluRay.x264-GROUP")
	cand := parse("Movie.2020.ENGLISH.1080p.BluRay.x264-GROUP")
	require.True(t, ReleasesMatch(src, cand, MatchOptions{}), "empty language should match ENGLISH")
}

func TestReleasesMatch_LanguageFrenchVsEmpty(t *testing.T) {
	src := parse("Movie.2020.FRENCH.1080p.BluRay.x264-GROUP")
	cand := parse("Movie.2020.1080p.BluRay.x264-GROUP")
	require.False(t, ReleasesMatch(src, cand, MatchOptions{}), "FRENCH vs empty (English) should not match")
}

func TestReleasesMatch_DifferentVersion(t *testing.T) {
	src := &rls.Release{Title: "Show", Series: 1, Episode: 1, Resolution: "1080p", Source: "WEB-DL", Codec: []string{"x264"}, Group: "GROUP", Version: "v2"}
	cand := &rls.Release{Title: "Show", Series: 1, Episode: 1, Resolution: "1080p", Source: "WEB-DL", Codec: []string{"x264"}, Group: "GROUP", Version: "v1"}
	require.False(t, ReleasesMatch(src, cand, MatchOptions{}), "v2 vs v1 should not match")
}

func TestReleasesMatch_VersionOnlyOneSide(t *testing.T) {
	src := &rls.Release{Title: "Show", Series: 1, Episode: 1, Resolution: "1080p", Source: "WEB-DL", Codec: []string{"x264"}, Group: "GROUP"}
	cand := &rls.Release{Title: "Show", Series: 1, Episode: 1, Resolution: "1080p", Source: "WEB-DL", Codec: []string{"x264"}, Group: "GROUP", Version: "v2"}
	require.True(t, ReleasesMatch(src, cand, MatchOptions{}), "version only on one side should be permissive")
}

// --- Variant tests (IMAX, HYBRID, REPACK, PROPER) ---

func TestReleasesMatch_IMAXMismatch(t *testing.T) {
	src := parse("Movie.2020.IMAX.1080p.WEB-DL.x264-GROUP")
	cand := parse("Movie.2020.1080p.WEB-DL.x264-GROUP")
	require.False(t, ReleasesMatch(src, cand, MatchOptions{}), "IMAX vs non-IMAX should not match")
}

func TestReleasesMatch_IMAXMatch(t *testing.T) {
	src := parse("Movie.2020.IMAX.1080p.WEB-DL.x264-GROUP")
	cand := parse("Movie.2020.IMAX.1080p.WEB-DL.x264-GROUP")
	require.True(t, ReleasesMatch(src, cand, MatchOptions{}), "matching IMAX releases should match")
}

func TestReleasesMatch_HybridMismatch(t *testing.T) {
	src := parse("Movie.2020.HYBRID.1080p.BluRay.x264-GROUP")
	cand := parse("Movie.2020.1080p.BluRay.x264-GROUP")
	require.False(t, ReleasesMatch(src, cand, MatchOptions{}), "HYBRID vs non-HYBRID should not match")
}

func TestReleasesMatch_RepackMismatchEpisode(t *testing.T) {
	src := parse("Show.S01E01.REPACK.1080p.WEB-DL.x264-GROUP")
	cand := parse("Show.S01E01.1080p.WEB-DL.x264-GROUP")
	require.False(t, ReleasesMatch(src, cand, MatchOptions{}), "REPACK vs non-REPACK episode should not match")
}

func TestReleasesMatch_RepackMatchEpisode(t *testing.T) {
	src := parse("Show.S01E01.REPACK.1080p.WEB-DL.x264-GROUP")
	cand := parse("Show.S01E01.REPACK.1080p.WEB-DL.x264-GROUP")
	require.True(t, ReleasesMatch(src, cand, MatchOptions{}), "matching REPACK episodes should match")
}

func TestReleasesMatch_RepackExemptSeasonPack(t *testing.T) {
	src := parse("Show.S01.1080p.WEB-DL.x264-GROUP")
	cand := parse("Show.S01.REPACK.1080p.WEB-DL.x264-GROUP")
	require.True(t, ReleasesMatch(src, cand, MatchOptions{}), "REPACK mismatch should be exempt for season packs")
}

func TestReleasesMatch_ProperMismatchEpisode(t *testing.T) {
	src := parse("Show.S01E01.PROPER.1080p.WEB-DL.x264-GROUP")
	cand := parse("Show.S01E01.1080p.WEB-DL.x264-GROUP")
	require.False(t, ReleasesMatch(src, cand, MatchOptions{}), "PROPER vs non-PROPER episode should not match")
}

// --- Anime fields (Site, Sum) ---

func TestReleasesMatch_DifferentAnimeSite(t *testing.T) {
	src := &rls.Release{Title: "Show", Series: 1, Episode: 1, Resolution: "1080p", Codec: []string{"HEVC"}, Site: "SubsPlease"}
	cand := &rls.Release{Title: "Show", Series: 1, Episode: 1, Resolution: "1080p", Codec: []string{"HEVC"}, Site: "Erai-raws"}
	require.False(t, ReleasesMatch(src, cand, MatchOptions{}), "different anime sites should not match")
}

func TestReleasesMatch_AnimeSiteMissing(t *testing.T) {
	src := &rls.Release{Title: "Show", Series: 1, Episode: 1, Resolution: "1080p", Codec: []string{"HEVC"}, Site: "SubsPlease"}
	cand := &rls.Release{Title: "Show", Series: 1, Episode: 1, Resolution: "1080p", Codec: []string{"HEVC"}}
	require.True(t, ReleasesMatch(src, cand, MatchOptions{}), "missing site on candidate should be permissive")
}

func TestReleasesMatch_DifferentCRC32(t *testing.T) {
	src := &rls.Release{Title: "Show", Series: 1, Episode: 1, Resolution: "1080p", Codec: []string{"HEVC"}, Sum: "32ECE75A"}
	cand := &rls.Release{Title: "Show", Series: 1, Episode: 1, Resolution: "1080p", Codec: []string{"HEVC"}, Sum: "AABBCCDD"}
	require.False(t, ReleasesMatch(src, cand, MatchOptions{}), "different CRC32 checksums mean different files")
}

func TestReleasesMatch_SameCRC32(t *testing.T) {
	src := &rls.Release{Title: "Show", Series: 1, Episode: 1, Resolution: "1080p", Codec: []string{"HEVC"}, Sum: "32ECE75A"}
	cand := &rls.Release{Title: "Show", Series: 1, Episode: 1, Resolution: "1080p", Codec: []string{"HEVC"}, Sum: "32ECE75A"}
	require.True(t, ReleasesMatch(src, cand, MatchOptions{}), "matching CRC32 should match")
}

// --- Cross-seed-only checks (Artist, Date, Type, Disc, Platform, Arch, FindIndividualEpisodes) ---

func TestReleasesMatch_ArtistMustMatch(t *testing.T) {
	adamBeyer := rls.Release{
		Type: rls.Music, Artist: "Adam Beyer", Title: "Dance Department",
		Year: 2025, Month: 10, Day: 4, Source: "CABLE", Group: "TALiON",
	}
	arminVanBuuren := rls.Release{
		Type: rls.Music, Artist: "Armin van Buuren", Title: "Dance Department",
		Year: 2025, Month: 11, Day: 29, Source: "CABLE", Group: "TALiON",
	}
	sameArtist := rls.Release{
		Type: rls.Music, Artist: "Adam Beyer", Title: "Dance Department",
		Year: 2025, Month: 10, Day: 4, Source: "CABLE", Group: "TALiON",
	}

	require.False(t, ReleasesMatch(&adamBeyer, &arminVanBuuren, MatchOptions{}),
		"different artists with same title should NOT match")
	require.True(t, ReleasesMatch(&adamBeyer, &sameArtist, MatchOptions{}),
		"same artist with same title should match")
}

func TestReleasesMatch_DateBasedReleasesRequireExactDate(t *testing.T) {
	oct4 := rls.Release{
		Type: rls.Music, Artist: "Artist", Title: "Show",
		Year: 2025, Month: 10, Day: 4, Group: "GROUP",
	}
	nov29 := rls.Release{
		Type: rls.Music, Artist: "Artist", Title: "Show",
		Year: 2025, Month: 11, Day: 29, Group: "GROUP",
	}
	sameDate := rls.Release{
		Type: rls.Music, Artist: "Artist", Title: "Show",
		Year: 2025, Month: 10, Day: 4, Group: "GROUP",
	}

	require.False(t, ReleasesMatch(&oct4, &nov29, MatchOptions{}),
		"same year but different month/day should NOT match")
	require.True(t, ReleasesMatch(&oct4, &sameDate, MatchOptions{}),
		"same year/month/day should match")
}

func TestReleasesMatch_NonTVRequiresCompatibleType(t *testing.T) {
	movie := rls.Release{Type: rls.Movie, Title: "Shared Title", Year: 2025}
	music := rls.Release{Type: rls.Music, Title: "Shared Title", Year: 2025}
	unknown := rls.Release{Type: rls.Unknown, Title: "Shared Title", Year: 2025}

	require.False(t, ReleasesMatch(&movie, &music, MatchOptions{}),
		"movie and music with same title/year should not match")
	require.True(t, ReleasesMatch(&movie, &unknown, MatchOptions{}),
		"unknown type should not block matching when other metadata agrees")
	require.True(t, ReleasesMatch(&unknown, &music, MatchOptions{}),
		"unknown type should not block matching when other metadata agrees")
}

func TestReleasesMatch_DiscMustMatch(t *testing.T) {
	disc1 := &rls.Release{Title: "Album", Year: 2025, Group: "GROUP", Disc: "1"}
	disc2 := &rls.Release{Title: "Album", Year: 2025, Group: "GROUP", Disc: "2"}
	noDisc := &rls.Release{Title: "Album", Year: 2025, Group: "GROUP"}

	require.False(t, ReleasesMatch(disc1, disc2, MatchOptions{}), "Disc1 vs Disc2 should not match")
	require.True(t, ReleasesMatch(disc1, noDisc, MatchOptions{}), "disc vs no disc should be permissive")
}

func TestReleasesMatch_PlatformMustMatch(t *testing.T) {
	win := &rls.Release{Title: "App", Year: 2025, Group: "GROUP", Platform: "Windows"}
	mac := &rls.Release{Title: "App", Year: 2025, Group: "GROUP", Platform: "macOS"}
	noPlatform := &rls.Release{Title: "App", Year: 2025, Group: "GROUP"}

	require.False(t, ReleasesMatch(win, mac, MatchOptions{}), "Windows vs macOS should not match")
	require.True(t, ReleasesMatch(win, noPlatform, MatchOptions{}), "platform vs no platform should be permissive")
}

func TestReleasesMatch_ArchMustMatch(t *testing.T) {
	x64 := &rls.Release{Title: "App", Year: 2025, Group: "GROUP", Arch: "x64"}
	x86 := &rls.Release{Title: "App", Year: 2025, Group: "GROUP", Arch: "x86"}
	noArch := &rls.Release{Title: "App", Year: 2025, Group: "GROUP"}

	require.False(t, ReleasesMatch(x64, x86, MatchOptions{}), "x64 vs x86 should not match")
	require.True(t, ReleasesMatch(x64, noArch, MatchOptions{}), "arch vs no arch should be permissive")
}

func TestReleasesMatch_FindIndividualEpisodes(t *testing.T) {
	seasonPack := parse("Show.S01.1080p.BluRay.x264-GROUP")
	episode := parse("Show.S01E01.1080p.BluRay.x264-GROUP")

	// Without FindIndividualEpisodes: season pack should NOT match episode
	require.False(t, ReleasesMatch(seasonPack, episode, MatchOptions{FindIndividualEpisodes: false}),
		"strict mode: season pack vs episode should not match")

	// With FindIndividualEpisodes: season pack SHOULD match episode
	require.True(t, ReleasesMatch(seasonPack, episode, MatchOptions{FindIndividualEpisodes: true}),
		"flexible mode: season pack should match episode")

	// Even in flexible mode, different episodes should not match each other
	episode2 := parse("Show.S01E02.1080p.BluRay.x264-GROUP")
	require.False(t, ReleasesMatch(episode, episode2, MatchOptions{FindIndividualEpisodes: true}),
		"flexible mode: different episodes should still not match")
}
