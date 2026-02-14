// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package releasematch

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeSource(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// WEB-DL variants normalize to WEBDL
		{"WEB-DL uppercase", "WEB-DL", "WEBDL"},
		{"web-dl lowercase", "web-dl", "WEBDL"},
		{"WEBDL no hyphen", "WEBDL", "WEBDL"},

		// WEBRip variants normalize to WEBRIP
		{"WEBRIP uppercase", "WEBRIP", "WEBRIP"},
		{"WEBRip mixed", "WEBRip", "WEBRIP"},
		{"webrip lowercase", "webrip", "WEBRIP"},

		// Plain WEB stays as WEB (ambiguous)
		{"WEB uppercase", "WEB", "WEB"},
		{"web lowercase", "web", "WEB"},

		// Non-web sources pass through uppercased
		{"BluRay passthrough", "BluRay", "BLURAY"},
		{"HDTV passthrough", "HDTV", "HDTV"},
		{"DVDRip passthrough", "DVDRip", "DVDRIP"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeSource(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestSourcesCompatible(t *testing.T) {
	tests := []struct {
		name       string
		source     string
		candidate  string
		compatible bool
	}{
		// Empty sources are always compatible
		{"both empty", "", "", true},
		{"source empty", "", "WEBDL", true},
		{"candidate empty", "WEBDL", "", true},

		// Identical sources are compatible
		{"both WEBDL", "WEBDL", "WEBDL", true},
		{"both WEBRIP", "WEBRIP", "WEBRIP", true},
		{"both WEB", "WEB", "WEB", true},
		{"both BLURAY", "BLURAY", "BLURAY", true},

		// WEB is ambiguous - matches both WEBDL and WEBRIP
		{"WEB matches WEBDL", "WEB", "WEBDL", true},
		{"WEBDL matches WEB", "WEBDL", "WEB", true},
		{"WEB matches WEBRIP", "WEB", "WEBRIP", true},
		{"WEBRIP matches WEB", "WEBRIP", "WEB", true},

		// WEBDL and WEBRIP are explicitly different
		{"WEBDL vs WEBRIP", "WEBDL", "WEBRIP", false},
		{"WEBRIP vs WEBDL", "WEBRIP", "WEBDL", false},

		// Other sources must match exactly
		{"BLURAY vs HDTV", "BLURAY", "HDTV", false},
		{"WEBDL vs BLURAY", "WEBDL", "BLURAY", false},
		{"WEB does not match BLURAY", "WEB", "BLURAY", false},
		{"BLURAY does not match WEB", "BLURAY", "WEB", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SourcesCompatible(tt.source, tt.candidate)
			require.Equal(t, tt.compatible, result)
		})
	}
}

func TestNormalizeVideoCodec(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// AVC/H.264 aliases
		{"x264 lowercase", "x264", "AVC"},
		{"X264 uppercase", "X264", "AVC"},
		{"H.264 with dot", "H.264", "AVC"},
		{"h.264 lowercase", "h.264", "AVC"},
		{"H264 no dot", "H264", "AVC"},
		{"AVC direct", "AVC", "AVC"},
		{"avc lowercase", "avc", "AVC"},

		// HEVC/H.265 aliases
		{"x265 lowercase", "x265", "HEVC"},
		{"X265 uppercase", "X265", "HEVC"},
		{"H.265 with dot", "H.265", "HEVC"},
		{"h.265 lowercase", "h.265", "HEVC"},
		{"H265 no dot", "H265", "HEVC"},
		{"HEVC direct", "HEVC", "HEVC"},
		{"hevc lowercase", "hevc", "HEVC"},

		// Non-aliased codecs pass through uppercased
		{"VP9 passthrough", "VP9", "VP9"},
		{"AV1 passthrough", "AV1", "AV1"},
		{"XViD passthrough", "XViD", "XVID"},
		{"DivX passthrough", "DivX", "DIVX"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeVideoCodec(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestJoinNormalizedCodecSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{"empty slice", []string{}, ""},
		{"single x264", []string{"x264"}, "AVC"},
		{"single H.264", []string{"H.264"}, "AVC"},
		{"x265 alone", []string{"x265"}, "HEVC"},
		{"H.265 alone", []string{"H.265"}, "HEVC"},
		{"multiple codecs sorted", []string{"HEVC", "AVC"}, "AVC HEVC"},
		{"dedupe hevc aliases", []string{"HEVC", "x265"}, "HEVC"},
		{"dedupe avc aliases", []string{"x264", "H.264"}, "AVC"},
		{"passthrough codec", []string{"VP9"}, "VP9"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := JoinNormalizedCodecSlice(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestJoinNormalizedSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{"empty slice", []string{}, ""},
		{"single element", []string{"HDR"}, "HDR"},
		{"multiple sorted", []string{"DV", "HDR10"}, "DV HDR10"},
		{"case normalization", []string{"hdr", "DV"}, "DV HDR"},
		{"trimming", []string{" HDR ", " DV "}, "DV HDR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := JoinNormalizedSlice(tt.input)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestSDFallbackAllowed(t *testing.T) {
	tests := []struct {
		name     string
		a, b     string
		expected bool
	}{
		{"empty vs 480P", "", "480P", true},
		{"480P vs empty", "480P", "", true},
		{"empty vs 576P", "", "576P", true},
		{"576P vs empty", "576P", "", true},
		{"empty vs SD", "", "SD", true},
		{"SD vs empty", "SD", "", true},
		{"empty vs 720P", "", "720P", false},
		{"empty vs 1080P", "", "1080P", false},
		{"both empty", "", "", false},
		{"480P vs 576P", "480P", "576P", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SDFallbackAllowed(tt.a, tt.b)
			require.Equal(t, tt.expected, result)
		})
	}
}
