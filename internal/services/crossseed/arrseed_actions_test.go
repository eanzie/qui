// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package crossseed

import (
	"encoding/json"
	"testing"
)

// --- arrSeedContainsInt ---

func TestArrSeedContainsInt_Found(t *testing.T) {
	if !arrSeedContainsInt([]int{1, 2, 3}, 2) {
		t.Error("expected true for value present in slice")
	}
}

func TestArrSeedContainsInt_NotFound(t *testing.T) {
	if arrSeedContainsInt([]int{1, 2, 3}, 4) {
		t.Error("expected false for value not in slice")
	}
}

func TestArrSeedContainsInt_EmptySlice(t *testing.T) {
	if arrSeedContainsInt([]int{}, 1) {
		t.Error("expected false for empty slice")
	}
}

func TestArrSeedContainsInt_NilSlice(t *testing.T) {
	if arrSeedContainsInt(nil, 1) {
		t.Error("expected false for nil slice")
	}
}

// --- arrSeedExtractTagsFromJSON ---

func TestArrSeedExtractTagsFromJSON_ValidTags(t *testing.T) {
	raw := json.RawMessage(`{"id":1,"title":"Show","tags":[5,10,15]}`)
	tags := arrSeedExtractTagsFromJSON(raw)
	if len(tags) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(tags))
	}
	if tags[0] != 5 || tags[1] != 10 || tags[2] != 15 {
		t.Errorf("unexpected tags: %v", tags)
	}
}

func TestArrSeedExtractTagsFromJSON_EmptyTags(t *testing.T) {
	raw := json.RawMessage(`{"id":1,"tags":[]}`)
	tags := arrSeedExtractTagsFromJSON(raw)
	if len(tags) != 0 {
		t.Fatalf("expected 0 tags, got %d", len(tags))
	}
}

func TestArrSeedExtractTagsFromJSON_NoTagsField(t *testing.T) {
	raw := json.RawMessage(`{"id":1,"title":"Show"}`)
	tags := arrSeedExtractTagsFromJSON(raw)
	if tags != nil {
		t.Fatalf("expected nil for missing tags field, got %v", tags)
	}
}

func TestArrSeedExtractTagsFromJSON_InvalidJSON(t *testing.T) {
	raw := json.RawMessage(`not valid json`)
	tags := arrSeedExtractTagsFromJSON(raw)
	if tags != nil {
		t.Fatalf("expected nil for invalid JSON, got %v", tags)
	}
}

func TestArrSeedExtractTagsFromJSON_NilInput(t *testing.T) {
	tags := arrSeedExtractTagsFromJSON(nil)
	if tags != nil {
		t.Fatalf("expected nil for nil input, got %v", tags)
	}
}

// --- arrSeedSetSeasonMonitored ---

func TestArrSeedSetSeasonMonitored_SetsFalse(t *testing.T) {
	series := map[string]any{
		"seasons": []any{
			map[string]any{"seasonNumber": float64(1), "monitored": true},
			map[string]any{"seasonNumber": float64(2), "monitored": true},
		},
	}

	allUnmonitored := arrSeedSetSeasonMonitored(series, 1, false)

	// Season 2 is still monitored, so allUnmonitored should be false
	if allUnmonitored {
		t.Error("expected allUnmonitored=false when season 2 is still monitored")
	}

	// Verify season 1 was set to false
	seasons := series["seasons"].([]any)
	s1 := seasons[0].(map[string]any)
	if s1["monitored"] != false {
		t.Error("expected season 1 monitored=false")
	}
}

func TestArrSeedSetSeasonMonitored_AllUnmonitored(t *testing.T) {
	series := map[string]any{
		"seasons": []any{
			map[string]any{"seasonNumber": float64(1), "monitored": false},
			map[string]any{"seasonNumber": float64(2), "monitored": true},
		},
	}

	allUnmonitored := arrSeedSetSeasonMonitored(series, 2, false)

	if !allUnmonitored {
		t.Error("expected allUnmonitored=true when all seasons are unmonitored")
	}
}

func TestArrSeedSetSeasonMonitored_NoSeasonsKey(t *testing.T) {
	series := map[string]any{"id": float64(1)}

	result := arrSeedSetSeasonMonitored(series, 1, false)
	if result {
		t.Error("expected false when seasons key is missing")
	}
}

func TestArrSeedSetSeasonMonitored_WrongSeasonsType(t *testing.T) {
	series := map[string]any{"seasons": "not an array"}

	result := arrSeedSetSeasonMonitored(series, 1, false)
	if result {
		t.Error("expected false when seasons is not an array")
	}
}

func TestArrSeedSetSeasonMonitored_SeasonNotFound(t *testing.T) {
	series := map[string]any{
		"seasons": []any{
			map[string]any{"seasonNumber": float64(1), "monitored": true},
		},
	}

	// Trying to unmonitor season 5 which doesn't exist — season 1 stays monitored
	allUnmonitored := arrSeedSetSeasonMonitored(series, 5, false)
	if allUnmonitored {
		t.Error("expected allUnmonitored=false when existing seasons are still monitored")
	}

	// Verify season 1 was not modified
	seasons := series["seasons"].([]any)
	s1 := seasons[0].(map[string]any)
	if s1["monitored"] != true {
		t.Error("season 1 should not have been modified")
	}
}
