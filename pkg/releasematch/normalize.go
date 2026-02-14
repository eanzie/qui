// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package releasematch

import (
	"sort"
	"strings"
)

// upper is a shorthand for strings.ToUpper(strings.TrimSpace(s)).
func upper(s string) string {
	return strings.ToUpper(strings.TrimSpace(s))
}

// SDFallbackAllowed returns true when one resolution is empty and the other is
// a known SD resolution, since many SD releases omit the resolution tag.
func SDFallbackAllowed(a, b string) bool {
	isSD := func(res string) bool {
		switch res {
		case "480P", "576P", "SD":
			return true
		default:
			return false
		}
	}
	return (a == "" && isSD(b)) || (b == "" && isSD(a))
}

// SourceAliases maps source names to a canonical form for comparison.
// WEB-DL variants normalize to WEBDL, WEBRip variants to WEBRIP.
// Plain "WEB" stays as "WEB" and is treated as ambiguous (matches both).
var sourceAliases = map[string]string{
	"WEB-DL": "WEBDL",
	"WEBDL":  "WEBDL",
	"WEBRIP": "WEBRIP",
	"WEB":    "WEB",
}

// NormalizeSource converts a source string to its canonical form.
// Returns the original (uppercased) string if no alias mapping exists.
func NormalizeSource(source string) string {
	u := upper(source)
	if canonical, ok := sourceAliases[u]; ok {
		return canonical
	}
	return u
}

// SourcesCompatible checks if two sources are compatible for matching.
// Plain "WEB" is ambiguous and matches both WEBDL and WEBRIP.
// WEBDL and WEBRIP are explicitly different and do not match each other.
func SourcesCompatible(source, candidate string) bool {
	if source == "" || candidate == "" {
		return true
	}
	if source == candidate {
		return true
	}

	isWebSource := func(s string) bool {
		switch s {
		case "WEB", "WEBDL", "WEBRIP":
			return true
		default:
			return false
		}
	}

	if !isWebSource(source) || !isWebSource(candidate) {
		return false
	}

	// At this point both are web sources, but they differ.
	// WEBDL and WEBRIP are explicitly different and do not match each other.
	return source == "WEB" || candidate == "WEB"
}

// VideoCodecAliases maps equivalent video codec names to a canonical form.
// x264, H.264, H264, and AVC all refer to the same underlying codec (AVC/H.264).
// x265, H.265, H265, and HEVC all refer to the same underlying codec (HEVC/H.265).
var videoCodecAliases = map[string]string{
	"X264":  "AVC",
	"H.264": "AVC",
	"H264":  "AVC",
	"AVC":   "AVC",
	"X265":  "HEVC",
	"H.265": "HEVC",
	"H265":  "HEVC",
	"HEVC":  "HEVC",
}

// normalizeVideoCodec converts a video codec string to its canonical form.
// Returns the original (uppercased) string if no alias mapping exists.
func normalizeVideoCodec(codec string) string {
	u := upper(codec)
	if canonical, ok := videoCodecAliases[u]; ok {
		return canonical
	}
	return u
}

// JoinNormalizedCodecSlice converts a codec slice to a normalized string for comparison.
// Applies codec aliasing so that x264, H.264, H264, and AVC are treated as equivalent.
func JoinNormalizedCodecSlice(slice []string) string {
	if len(slice) == 0 {
		return ""
	}
	seen := make(map[string]struct{}, len(slice))
	normalized := make([]string, 0, len(slice))
	for _, codec := range slice {
		n := normalizeVideoCodec(codec)
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		normalized = append(normalized, n)
	}
	sort.Strings(normalized)
	return strings.Join(normalized, " ")
}

// JoinNormalizedSlice converts a string slice to a normalized uppercase string for comparison.
// Uppercases and joins elements to ensure consistent comparison regardless of case or order.
func JoinNormalizedSlice(slice []string) string {
	if len(slice) == 0 {
		return ""
	}
	normalized := make([]string, len(slice))
	for i, s := range slice {
		normalized[i] = upper(s)
	}
	sort.Strings(normalized)
	return strings.Join(normalized, " ")
}
