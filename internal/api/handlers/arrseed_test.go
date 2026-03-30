// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/qui/internal/database"
	"github.com/autobrr/qui/internal/models"
)

func setupArrSeedHandlerTest(t *testing.T) (*ArrSeedHandler, *models.ArrSeedStore, *database.DB) {
	t.Helper()

	db, err := database.New(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	store := models.NewArrSeedStore(db)
	handler := &ArrSeedHandler{store: store, service: nil}

	return handler, store, db
}

func createTestArrInstance(t *testing.T, db *database.DB) int {
	t.Helper()

	arrStore, err := models.NewArrInstanceStore(db, []byte("01234567890123456789012345678901"))
	require.NoError(t, err)

	instance, err := arrStore.Create(
		t.Context(),
		models.ArrInstanceTypeSonarr,
		"test-sonarr",
		"http://localhost:8989",
		"testapikey1234567890",
		nil, nil,
		true, 0, 30,
	)
	require.NoError(t, err)

	return instance.ID
}

func createTestQbitInstance(t *testing.T, db *database.DB) int {
	t.Helper()

	instanceStore, err := models.NewInstanceStore(db, []byte("01234567890123456789012345678901"))
	require.NoError(t, err)

	instance, err := instanceStore.Create(
		t.Context(),
		"test-qbit",
		"http://localhost:8080",
		"admin", "password",
		nil, nil,
		false, nil,
	)
	require.NoError(t, err)

	return instance.ID
}

func withChiParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// --- GetSettings ---

func TestGetSettings_DefaultsWhenEmpty(t *testing.T) {
	handler, _, _ := setupArrSeedHandlerTest(t)

	r := httptest.NewRequest(http.MethodGet, "/api/arr-seed/settings", nil)
	w := httptest.NewRecorder()

	handler.GetSettings(w, r)

	require.Equal(t, http.StatusOK, w.Code)

	var settings models.ArrSeedSettings
	require.NoError(t, json.NewDecoder(w.Body).Decode(&settings))
	assert.Equal(t, 5, settings.SearchDelaySeconds)
	assert.False(t, settings.Enabled)
}

func TestGetSettings_ReturnsStoredSettings(t *testing.T) {
	handler, store, _ := setupArrSeedHandlerTest(t)

	_, err := store.UpdateSettings(t.Context(), &models.ArrSeedSettings{
		Enabled:            true,
		SearchDelaySeconds: 10,
		MaxItemsPerRun:     50,
	})
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodGet, "/api/arr-seed/settings", nil)
	w := httptest.NewRecorder()

	handler.GetSettings(w, r)

	require.Equal(t, http.StatusOK, w.Code)

	var settings models.ArrSeedSettings
	require.NoError(t, json.NewDecoder(w.Body).Decode(&settings))
	assert.True(t, settings.Enabled)
	assert.Equal(t, 10, settings.SearchDelaySeconds)
	assert.Equal(t, 50, settings.MaxItemsPerRun)
}

// --- UpdateSettings ---

func TestUpdateSettings_ValidPayload(t *testing.T) {
	handler, _, _ := setupArrSeedHandlerTest(t)

	body := `{"enabled": true, "searchDelaySeconds": 15, "maxItemsPerRun": 100}`
	r := httptest.NewRequest(http.MethodPut, "/api/arr-seed/settings", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.UpdateSettings(w, r)

	require.Equal(t, http.StatusOK, w.Code)

	var settings models.ArrSeedSettings
	require.NoError(t, json.NewDecoder(w.Body).Decode(&settings))
	assert.True(t, settings.Enabled)
	assert.Equal(t, 15, settings.SearchDelaySeconds)
	assert.Equal(t, 100, settings.MaxItemsPerRun)
}

func TestUpdateSettings_InvalidJSON(t *testing.T) {
	handler, _, _ := setupArrSeedHandlerTest(t)

	r := httptest.NewRequest(http.MethodPut, "/api/arr-seed/settings", strings.NewReader(`{invalid`))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.UpdateSettings(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid request body")
}

func TestUpdateSettings_SeasonPackUpgradeForcesEpisodeFalse(t *testing.T) {
	handler, _, _ := setupArrSeedHandlerTest(t)

	body := `{"enableEpisode": true, "enableSeasonPackUpgrade": true}`
	r := httptest.NewRequest(http.MethodPut, "/api/arr-seed/settings", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.UpdateSettings(w, r)

	require.Equal(t, http.StatusOK, w.Code)

	var settings models.ArrSeedSettings
	require.NoError(t, json.NewDecoder(w.Body).Decode(&settings))
	assert.True(t, settings.EnableSeasonPackUpgrade)
	assert.False(t, settings.EnableEpisode, "enableEpisode must be false when enableSeasonPackUpgrade is true")
}

// --- ListConfigs ---

func TestListConfigs_EmptyList(t *testing.T) {
	handler, _, _ := setupArrSeedHandlerTest(t)

	r := httptest.NewRequest(http.MethodGet, "/api/arr-seed/configs", nil)
	w := httptest.NewRecorder()

	handler.ListConfigs(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "[]\n", w.Body.String())
}

func TestListConfigs_WithConfigs(t *testing.T) {
	handler, store, db := setupArrSeedHandlerTest(t)

	arrID := createTestArrInstance(t, db)
	qbitID := createTestQbitInstance(t, db)

	_, err := store.CreateConfig(t.Context(), &models.ArrSeedInstanceConfig{
		ArrInstanceID:        arrID,
		Enabled:              true,
		TargetQbitInstanceID: qbitID,
		Category:             "tv",
		ScanIntervalMinutes:  1440,
	})
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodGet, "/api/arr-seed/configs", nil)
	w := httptest.NewRecorder()

	handler.ListConfigs(w, r)

	require.Equal(t, http.StatusOK, w.Code)

	var configs []models.ArrSeedInstanceConfig
	require.NoError(t, json.NewDecoder(w.Body).Decode(&configs))
	assert.Len(t, configs, 1)
	assert.Equal(t, "tv", configs[0].Category)
}

// --- CreateConfig ---

func TestCreateConfig_ValidPayload(t *testing.T) {
	handler, _, db := setupArrSeedHandlerTest(t)

	arrID := createTestArrInstance(t, db)
	qbitID := createTestQbitInstance(t, db)

	body := `{"arrInstanceId": ` + itoa(arrID) + `, "targetQbitInstanceId": ` + itoa(qbitID) + `, "enabled": true, "category": "movies", "scanIntervalMinutes": 720}`
	r := httptest.NewRequest(http.MethodPost, "/api/arr-seed/configs", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CreateConfig(w, r)

	require.Equal(t, http.StatusCreated, w.Code)

	var cfg models.ArrSeedInstanceConfig
	require.NoError(t, json.NewDecoder(w.Body).Decode(&cfg))
	assert.Equal(t, "movies", cfg.Category)
	assert.Equal(t, 720, cfg.ScanIntervalMinutes)
	assert.True(t, cfg.Enabled)
}

func TestCreateConfig_MissingArrInstanceID(t *testing.T) {
	handler, _, db := setupArrSeedHandlerTest(t)

	qbitID := createTestQbitInstance(t, db)

	body := `{"targetQbitInstanceId": ` + itoa(qbitID) + `}`
	r := httptest.NewRequest(http.MethodPost, "/api/arr-seed/configs", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CreateConfig(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "arrInstanceId is required")
}

func TestCreateConfig_MissingTargetQbitInstanceID(t *testing.T) {
	handler, _, db := setupArrSeedHandlerTest(t)

	arrID := createTestArrInstance(t, db)

	body := `{"arrInstanceId": ` + itoa(arrID) + `}`
	r := httptest.NewRequest(http.MethodPost, "/api/arr-seed/configs", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CreateConfig(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "targetQbitInstanceId is required")
}

func TestCreateConfig_DefaultScanInterval(t *testing.T) {
	handler, _, db := setupArrSeedHandlerTest(t)

	arrID := createTestArrInstance(t, db)
	qbitID := createTestQbitInstance(t, db)

	body := `{"arrInstanceId": ` + itoa(arrID) + `, "targetQbitInstanceId": ` + itoa(qbitID) + `, "scanIntervalMinutes": 0}`
	r := httptest.NewRequest(http.MethodPost, "/api/arr-seed/configs", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.CreateConfig(w, r)

	require.Equal(t, http.StatusCreated, w.Code)

	var cfg models.ArrSeedInstanceConfig
	require.NoError(t, json.NewDecoder(w.Body).Decode(&cfg))
	assert.Equal(t, 1440, cfg.ScanIntervalMinutes)
}

// --- GetConfig ---

func TestGetConfig_Existing(t *testing.T) {
	handler, store, db := setupArrSeedHandlerTest(t)

	arrID := createTestArrInstance(t, db)
	qbitID := createTestQbitInstance(t, db)

	created, err := store.CreateConfig(t.Context(), &models.ArrSeedInstanceConfig{
		ArrInstanceID:        arrID,
		TargetQbitInstanceID: qbitID,
		Category:             "tv",
		ScanIntervalMinutes:  1440,
	})
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodGet, "/api/arr-seed/configs/1", nil)
	r = withChiParam(r, "configID", itoa(created.ID))
	w := httptest.NewRecorder()

	handler.GetConfig(w, r)

	require.Equal(t, http.StatusOK, w.Code)

	var cfg models.ArrSeedInstanceConfig
	require.NoError(t, json.NewDecoder(w.Body).Decode(&cfg))
	assert.Equal(t, created.ID, cfg.ID)
	assert.Equal(t, "tv", cfg.Category)
}

func TestGetConfig_NotFound(t *testing.T) {
	handler, _, _ := setupArrSeedHandlerTest(t)

	r := httptest.NewRequest(http.MethodGet, "/api/arr-seed/configs/99999", nil)
	r = withChiParam(r, "configID", "99999")
	w := httptest.NewRecorder()

	handler.GetConfig(w, r)

	require.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "Config not found")
}

func TestGetConfig_InvalidID(t *testing.T) {
	handler, _, _ := setupArrSeedHandlerTest(t)

	r := httptest.NewRequest(http.MethodGet, "/api/arr-seed/configs/abc", nil)
	r = withChiParam(r, "configID", "abc")
	w := httptest.NewRecorder()

	handler.GetConfig(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "Invalid config ID")
}

// --- UpdateConfig ---

func TestUpdateConfig_PartialUpdate(t *testing.T) {
	handler, store, db := setupArrSeedHandlerTest(t)

	arrID := createTestArrInstance(t, db)
	qbitID := createTestQbitInstance(t, db)

	created, err := store.CreateConfig(t.Context(), &models.ArrSeedInstanceConfig{
		ArrInstanceID:        arrID,
		TargetQbitInstanceID: qbitID,
		Category:             "tv",
		ScanIntervalMinutes:  1440,
	})
	require.NoError(t, err)

	body := `{"category": "movies", "enabled": true}`
	r := httptest.NewRequest(http.MethodPatch, "/api/arr-seed/configs/1", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParam(r, "configID", itoa(created.ID))
	w := httptest.NewRecorder()

	handler.UpdateConfig(w, r)

	require.Equal(t, http.StatusOK, w.Code)

	var cfg models.ArrSeedInstanceConfig
	require.NoError(t, json.NewDecoder(w.Body).Decode(&cfg))
	assert.Equal(t, "movies", cfg.Category)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, 1440, cfg.ScanIntervalMinutes, "unchanged fields should be preserved")
}

func TestUpdateConfig_NotFound(t *testing.T) {
	handler, _, _ := setupArrSeedHandlerTest(t)

	body := `{"category": "movies"}`
	r := httptest.NewRequest(http.MethodPatch, "/api/arr-seed/configs/99999", strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = withChiParam(r, "configID", "99999")
	w := httptest.NewRecorder()

	handler.UpdateConfig(w, r)

	require.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "Config not found")
}

// --- DeleteConfig ---

func TestDeleteConfig_Existing(t *testing.T) {
	handler, store, db := setupArrSeedHandlerTest(t)

	arrID := createTestArrInstance(t, db)
	qbitID := createTestQbitInstance(t, db)

	created, err := store.CreateConfig(t.Context(), &models.ArrSeedInstanceConfig{
		ArrInstanceID:        arrID,
		TargetQbitInstanceID: qbitID,
		ScanIntervalMinutes:  1440,
	})
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodDelete, "/api/arr-seed/configs/1", nil)
	r = withChiParam(r, "configID", itoa(created.ID))
	w := httptest.NewRecorder()

	handler.DeleteConfig(w, r)

	require.Equal(t, http.StatusNoContent, w.Code)
}

func TestDeleteConfig_NotFound(t *testing.T) {
	handler, _, _ := setupArrSeedHandlerTest(t)

	r := httptest.NewRequest(http.MethodDelete, "/api/arr-seed/configs/99999", nil)
	r = withChiParam(r, "configID", "99999")
	w := httptest.NewRecorder()

	handler.DeleteConfig(w, r)

	require.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "Config not found")
}

// --- ListRuns ---

func TestListRuns_Empty(t *testing.T) {
	handler, store, db := setupArrSeedHandlerTest(t)

	arrID := createTestArrInstance(t, db)
	qbitID := createTestQbitInstance(t, db)

	created, err := store.CreateConfig(t.Context(), &models.ArrSeedInstanceConfig{
		ArrInstanceID:        arrID,
		TargetQbitInstanceID: qbitID,
		ScanIntervalMinutes:  1440,
	})
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodGet, "/api/arr-seed/configs/1/runs", nil)
	r = withChiParam(r, "configID", itoa(created.ID))
	w := httptest.NewRecorder()

	handler.ListRuns(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "[]\n", w.Body.String())
}

// --- ListItems ---

func TestListItems_Empty(t *testing.T) {
	handler, store, db := setupArrSeedHandlerTest(t)

	arrID := createTestArrInstance(t, db)
	qbitID := createTestQbitInstance(t, db)

	created, err := store.CreateConfig(t.Context(), &models.ArrSeedInstanceConfig{
		ArrInstanceID:        arrID,
		TargetQbitInstanceID: qbitID,
		ScanIntervalMinutes:  1440,
	})
	require.NoError(t, err)

	r := httptest.NewRequest(http.MethodGet, "/api/arr-seed/configs/1/items", nil)
	r = withChiParam(r, "configID", itoa(created.ID))
	w := httptest.NewRecorder()

	handler.ListItems(w, r)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "[]\n", w.Body.String())
}

func TestListItems_WithStatusFilter(t *testing.T) {
	handler, store, db := setupArrSeedHandlerTest(t)

	arrID := createTestArrInstance(t, db)
	qbitID := createTestQbitInstance(t, db)

	created, err := store.CreateConfig(t.Context(), &models.ArrSeedInstanceConfig{
		ArrInstanceID:        arrID,
		TargetQbitInstanceID: qbitID,
		ScanIntervalMinutes:  1440,
	})
	require.NoError(t, err)

	ctx := t.Context()
	require.NoError(t, store.UpsertItem(ctx, &models.ArrSeedItem{
		ConfigID:    created.ID,
		ItemType:    "movie",
		ArrFileID:   1,
		ReleaseName: "Test.Movie.2024",
		FilePath:    "/data/movies/test.mkv",
		FileSize:    1000,
		Priority:    1,
		Status:      models.ArrSeedItemStatusPending,
	}))
	require.NoError(t, store.UpsertItem(ctx, &models.ArrSeedItem{
		ConfigID:    created.ID,
		ItemType:    "movie",
		ArrFileID:   2,
		ReleaseName: "Another.Movie.2024",
		FilePath:    "/data/movies/another.mkv",
		FileSize:    2000,
		Priority:    1,
		Status:      models.ArrSeedItemStatusSeeded,
	}))

	r := httptest.NewRequest(http.MethodGet, "/api/arr-seed/configs/1/items?status=pending", nil)
	r = withChiParam(r, "configID", itoa(created.ID))
	w := httptest.NewRecorder()

	handler.ListItems(w, r)

	require.Equal(t, http.StatusOK, w.Code)

	var items []models.ArrSeedItem
	require.NoError(t, json.NewDecoder(w.Body).Decode(&items))
	assert.Len(t, items, 1)
	assert.Equal(t, models.ArrSeedItemStatusPending, items[0].Status)
}

func TestListItems_WithLimitOffset(t *testing.T) {
	handler, store, db := setupArrSeedHandlerTest(t)

	arrID := createTestArrInstance(t, db)
	qbitID := createTestQbitInstance(t, db)

	created, err := store.CreateConfig(t.Context(), &models.ArrSeedInstanceConfig{
		ArrInstanceID:        arrID,
		TargetQbitInstanceID: qbitID,
		ScanIntervalMinutes:  1440,
	})
	require.NoError(t, err)

	ctx := t.Context()
	for i := 1; i <= 5; i++ {
		require.NoError(t, store.UpsertItem(ctx, &models.ArrSeedItem{
			ConfigID:    created.ID,
			ItemType:    "movie",
			ArrFileID:   i,
			ReleaseName: "Movie." + itoa(i),
			FilePath:    "/data/movies/" + itoa(i) + ".mkv",
			FileSize:    int64(i * 1000),
			Priority:    1,
			Status:      models.ArrSeedItemStatusPending,
		}))
	}

	r := httptest.NewRequest(http.MethodGet, "/api/arr-seed/configs/1/items?limit=2&offset=1", nil)
	r = withChiParam(r, "configID", itoa(created.ID))
	w := httptest.NewRecorder()

	handler.ListItems(w, r)

	require.Equal(t, http.StatusOK, w.Code)

	var items []models.ArrSeedItem
	require.NoError(t, json.NewDecoder(w.Body).Decode(&items))
	assert.Len(t, items, 2)
}

// --- ResetItems ---

func TestResetItems_ClearsItems(t *testing.T) {
	handler, store, db := setupArrSeedHandlerTest(t)

	arrID := createTestArrInstance(t, db)
	qbitID := createTestQbitInstance(t, db)

	created, err := store.CreateConfig(t.Context(), &models.ArrSeedInstanceConfig{
		ArrInstanceID:        arrID,
		TargetQbitInstanceID: qbitID,
		ScanIntervalMinutes:  1440,
	})
	require.NoError(t, err)

	ctx := t.Context()
	require.NoError(t, store.UpsertItem(ctx, &models.ArrSeedItem{
		ConfigID:    created.ID,
		ItemType:    "movie",
		ArrFileID:   1,
		ReleaseName: "Test.Movie",
		FilePath:    "/data/test.mkv",
		FileSize:    1000,
		Priority:    1,
		Status:      models.ArrSeedItemStatusPending,
	}))

	r := httptest.NewRequest(http.MethodPost, "/api/arr-seed/configs/1/items/reset", nil)
	r = withChiParam(r, "configID", itoa(created.ID))
	w := httptest.NewRecorder()

	handler.ResetItems(w, r)

	require.Equal(t, http.StatusNoContent, w.Code)

	// Verify items were cleared
	items, err := store.ListItems(ctx, created.ID, nil, 100, 0)
	require.NoError(t, err)
	assert.Empty(t, items)
}

func itoa(i int) string {
	return strconv.Itoa(i)
}
