// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package releasematch

import (
	"slices"
	"strings"

	"github.com/moistari/rls"
)

// variantOverrides lists release tags that must match on both releases for
// matching to be considered safe. This lets us plug RLS parsing gaps
// (e.g. IMAX vs HYBRID) without forking the parser.
type variantOverrides struct {
	collection []string
	other      []string
	edition    []string
	cut        []string
}

var (
	// strictVariantOverrides contains tags that must ALWAYS match exactly.
	// These represent different video masters (IMAX, HYBRID) that cannot be cross-seeded.
	strictVariantOverrides = newVariantOverrides(
		[]string{"IMAX"},
		[]string{"HYBRID"},
		nil,
		nil,
	)

	// nonPackVariantOverrides contains tags that must match for non-pack content.
	// Season packs are exempt because a pack might contain a REPACK of just one episode.
	nonPackVariantOverrides = newVariantOverrides(
		nil,
		[]string{
			"REPACK", "REPACK2", "REPACK3", "REPACK4", "REPACK5",
			"REPACK6", "REPACK7", "REPACK8", "REPACK9", "REPACK10",
			"PROPER",
		},
		nil,
		nil,
	)
)

func newVariantOverrides(collection, other, edition, cut []string) variantOverrides {
	return variantOverrides{
		collection: normalizeVariantSlice(collection),
		other:      normalizeVariantSlice(other),
		edition:    normalizeVariantSlice(edition),
		cut:        normalizeVariantSlice(cut),
	}
}

func normalizeVariantSlice(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, v := range values {
		if nv := upper(v); nv != "" {
			normalized = append(normalized, nv)
		}
	}
	return normalized
}

func (o variantOverrides) releaseVariants(r *rls.Release) map[string]struct{} {
	variants := make(map[string]struct{})

	addVariant := func(name string) {
		if name == "" {
			return
		}
		variants[name] = struct{}{}
	}

	for _, candidate := range o.collection {
		if upper(r.Collection) == candidate {
			addVariant(candidate)
		}
	}

	addListVariants := func(values []string, strictValues []string) {
		if len(values) == 0 || len(strictValues) == 0 {
			return
		}
		for _, entry := range values {
			nv := upper(entry)
			if nv == "" {
				continue
			}
			for _, candidate := range strictValues {
				if variantValueMatches(nv, candidate) {
					addVariant(candidate)
				}
			}
		}
	}

	addListVariants(r.Other, o.other)
	addListVariants(r.Edition, o.edition)
	addListVariants(r.Cut, o.cut)

	return variants
}

func variantValueMatches(value, target string) bool {
	if value == "" || target == "" {
		return false
	}
	if value == target {
		return true
	}
	tokens := variantTokens(value)
	return slices.Contains(tokens, target)
}

func variantTokens(value string) []string {
	split := func(r rune) bool {
		switch r {
		case '.', '-', '_', ' ', '/', '+', '[', ']', '(', ')':
			return true
		default:
			return false
		}
	}
	tokens := strings.FieldsFunc(value, split)
	if len(tokens) == 0 && value != "" {
		return []string{value}
	}
	return tokens
}

func (o variantOverrides) variantsCompatible(source, candidate *rls.Release) bool {
	sourceVariants := o.releaseVariants(source)
	if len(sourceVariants) == 0 {
		return true
	}
	candidateVariants := o.releaseVariants(candidate)
	for key := range sourceVariants {
		if _, ok := candidateVariants[key]; !ok {
			return false
		}
	}
	return true
}

// findMismatch returns the first variant in source that is missing from candidate.
// Returns empty string if all variants match.
func (o variantOverrides) findMismatch(source, candidate *rls.Release) string {
	sourceVariants := o.releaseVariants(source)
	if len(sourceVariants) == 0 {
		return ""
	}
	candidateVariants := o.releaseVariants(candidate)
	for key := range sourceVariants {
		if _, ok := candidateVariants[key]; !ok {
			return key
		}
	}
	return ""
}

// isSeasonPack returns true if the release is a season pack (has series but no episode).
func isSeasonPack(r *rls.Release) bool {
	return r.Series > 0 && r.Episode == 0
}

// CheckVariantsCompatible validates variant compatibility between source and candidate.
// For always-strict variants (IMAX, HYBRID), mismatches are never allowed.
// For non-pack variants (REPACK, PROPER), mismatches are allowed if either release is a season pack.
// Returns (compatible, mismatchReason) where mismatchReason is empty if compatible.
func CheckVariantsCompatible(source, candidate *rls.Release) (bool, string) {
	// Always-strict variants must match regardless of content type
	if mismatch := strictVariantOverrides.findMismatch(source, candidate); mismatch != "" {
		return false, mismatch
	}
	if mismatch := strictVariantOverrides.findMismatch(candidate, source); mismatch != "" {
		return false, mismatch
	}

	// Non-pack variants are skipped for season packs
	// A season pack might contain a REPACK of just one episode
	if isSeasonPack(source) || isSeasonPack(candidate) {
		return true, ""
	}

	// For non-pack content, REPACK/PROPER must match
	if mismatch := nonPackVariantOverrides.findMismatch(source, candidate); mismatch != "" {
		return false, mismatch
	}
	if mismatch := nonPackVariantOverrides.findMismatch(candidate, source); mismatch != "" {
		return false, mismatch
	}

	return true, ""
}
