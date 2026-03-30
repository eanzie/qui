// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package models_test

import (
	"context"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/autobrr/qui/internal/database"
	"github.com/autobrr/qui/internal/models"
)

var arrInstanceSeq atomic.Int64

func setupArrSeedTestDB(t *testing.T) *database.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "arrseed.db")
	db, err := database.New(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return db
}

// createTestArrInstance inserts an arr_instance so configs can reference it via FK.
func createTestArrInstance(t *testing.T, db *database.DB) int {
	t.Helper()
	ctx := context.Background()
	store, err := models.NewArrInstanceStore(db, []byte("01234567890123456789012345678901"))
	require.NoError(t, err)
	seq := arrInstanceSeq.Add(1)
	name := fmt.Sprintf("test-sonarr-%d", seq)
	baseURL := fmt.Sprintf("http://localhost:%d", 8900+seq)
	instance, err := store.Create(ctx, models.ArrInstanceTypeSonarr, name, baseURL, "testkey", nil, nil, true, 1, 15)
	require.NoError(t, err)
	return instance.ID
}

// createTestConfig is a shorthand for creating a config with sensible defaults.
func createTestConfig(t *testing.T, store *models.ArrSeedStore, arrInstanceID int, enabled bool) *models.ArrSeedInstanceConfig {
	t.Helper()
	ctx := context.Background()
	cfg, err := store.CreateConfig(ctx, &models.ArrSeedInstanceConfig{
		ArrInstanceID:       arrInstanceID,
		Enabled:             enabled,
		TargetQbitInstanceID: 0,
		Category:            "tv",
		ArrDockerPath:       "/data",
		HostDataPath:        "/host/data",
		TorrentSavePath:     "/torrents",
		ScanIntervalMinutes: 30,
		UnmonitorAfterSeed:  false,
		TagAfterSeed:        "cross-seeded",
	})
	require.NoError(t, err)
	return cfg
}

// --- Settings Tests ---

func TestArrSeedStore_GetSettings_ReturnsNilWhenEmpty(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	ctx := context.Background()

	settings, err := store.GetSettings(ctx)
	require.NoError(t, err)
	assert.Nil(t, settings)
}

func TestArrSeedStore_UpdateSettings_CreatesOnFirstCall(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	ctx := context.Background()

	result, err := store.UpdateSettings(ctx, &models.ArrSeedSettings{
		Enabled:            true,
		SearchDelaySeconds: 10,
		MaxItemsPerRun:     50,
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 1, result.ID)
	assert.True(t, result.Enabled)
	assert.Equal(t, 10, result.SearchDelaySeconds)
	assert.Equal(t, 50, result.MaxItemsPerRun)
}

func TestArrSeedStore_UpdateSettings_UpdatesExisting(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	ctx := context.Background()

	_, err := store.UpdateSettings(ctx, &models.ArrSeedSettings{
		Enabled:            true,
		SearchDelaySeconds: 10,
		MaxItemsPerRun:     50,
	})
	require.NoError(t, err)

	result, err := store.UpdateSettings(ctx, &models.ArrSeedSettings{
		Enabled:            false,
		SearchDelaySeconds: 20,
		MaxItemsPerRun:     100,
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 1, result.ID)
	assert.False(t, result.Enabled)
	assert.Equal(t, 20, result.SearchDelaySeconds)
	assert.Equal(t, 100, result.MaxItemsPerRun)
}

func TestArrSeedStore_UpdateSettings_NilReturnsError(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	ctx := context.Background()

	_, err := store.UpdateSettings(ctx, nil)
	require.EqualError(t, err, "settings is nil")
}

func TestArrSeedStore_UpdateSettings_RoundTrip(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	ctx := context.Background()

	input := &models.ArrSeedSettings{
		Enabled:                 true,
		SearchDelaySeconds:      30,
		MaxItemsPerRun:          200,
		EnableHighScoreOnly:     true,
		EnableEpisode:           true,
		EnableSeasonPackUpgrade: true,
	}

	_, err := store.UpdateSettings(ctx, input)
	require.NoError(t, err)

	got, err := store.GetSettings(ctx)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.True(t, got.Enabled)
	assert.Equal(t, 30, got.SearchDelaySeconds)
	assert.Equal(t, 200, got.MaxItemsPerRun)
	assert.True(t, got.EnableHighScoreOnly)
	assert.True(t, got.EnableEpisode)
	assert.True(t, got.EnableSeasonPackUpgrade)
	assert.False(t, got.CreatedAt.IsZero())
	assert.False(t, got.UpdatedAt.IsZero())
}

// --- Config Tests ---

func TestArrSeedStore_CreateConfig_PersistsAllFields(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID := createTestArrInstance(t, db)
	ctx := context.Background()

	cfg, err := store.CreateConfig(ctx, &models.ArrSeedInstanceConfig{
		ArrInstanceID:       arrID,
		Enabled:             true,
		TargetQbitInstanceID: 0,
		Category:            "movies",
		ArrDockerPath:       "/docker/data",
		HostDataPath:        "/host/data",
		TorrentSavePath:     "/torrents/save",
		ScanIntervalMinutes: 60,
		UnmonitorAfterSeed:  true,
		TagAfterSeed:        "seeded",
	})
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Greater(t, cfg.ID, 0)
	assert.Equal(t, arrID, cfg.ArrInstanceID)
	assert.True(t, cfg.Enabled)
	assert.Equal(t, 0, cfg.TargetQbitInstanceID)
	assert.Equal(t, "movies", cfg.Category)
	assert.Equal(t, "/docker/data", cfg.ArrDockerPath)
	assert.Equal(t, "/host/data", cfg.HostDataPath)
	assert.Equal(t, "/torrents/save", cfg.TorrentSavePath)
	assert.Equal(t, 60, cfg.ScanIntervalMinutes)
	assert.True(t, cfg.UnmonitorAfterSeed)
	assert.Equal(t, "seeded", cfg.TagAfterSeed)
	// Joined name from arr_instances
	assert.Contains(t, cfg.ArrInstanceName, "test-sonarr")
	assert.Equal(t, "sonarr", cfg.ArrInstanceType)
	assert.Nil(t, cfg.LastScanAt)
	assert.False(t, cfg.CreatedAt.IsZero())
	assert.False(t, cfg.UpdatedAt.IsZero())
}

func TestArrSeedStore_CreateConfig_NilReturnsError(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	ctx := context.Background()

	_, err := store.CreateConfig(ctx, nil)
	require.EqualError(t, err, "config is nil")
}

func TestArrSeedStore_GetConfig_NotFound(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	ctx := context.Background()

	_, err := store.GetConfig(ctx, 9999)
	require.ErrorIs(t, err, models.ErrArrSeedConfigNotFound)
}

func TestArrSeedStore_ListConfigs_EmptyWhenNone(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	ctx := context.Background()

	configs, err := store.ListConfigs(ctx)
	require.NoError(t, err)
	assert.Empty(t, configs)
}

func TestArrSeedStore_ListConfigs_OrderedByID(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID1 := createTestArrInstance(t, db)
	arrID2 := createTestArrInstance(t, db)
	ctx := context.Background()

	cfg1 := createTestConfig(t, store, arrID1, true)
	cfg2 := createTestConfig(t, store, arrID2, false)

	configs, err := store.ListConfigs(ctx)
	require.NoError(t, err)
	require.Len(t, configs, 2)
	assert.Equal(t, cfg1.ID, configs[0].ID)
	assert.Equal(t, cfg2.ID, configs[1].ID)
}

func TestArrSeedStore_ListEnabledConfigs(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID1 := createTestArrInstance(t, db)
	arrID2 := createTestArrInstance(t, db)
	ctx := context.Background()

	enabledCfg := createTestConfig(t, store, arrID1, true)
	_ = createTestConfig(t, store, arrID2, false) // disabled

	configs, err := store.ListEnabledConfigs(ctx)
	require.NoError(t, err)
	require.Len(t, configs, 1)
	assert.Equal(t, enabledCfg.ID, configs[0].ID)
	assert.True(t, configs[0].Enabled)
}

func TestArrSeedStore_UpdateConfig_NilParamsReturnsCurrentConfig(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID := createTestArrInstance(t, db)
	ctx := context.Background()

	original := createTestConfig(t, store, arrID, true)

	result, err := store.UpdateConfig(ctx, original.ID, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, original.ID, result.ID)
	assert.Equal(t, original.Category, result.Category)
}

func TestArrSeedStore_UpdateConfig_PartialUpdate(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID := createTestArrInstance(t, db)
	ctx := context.Background()

	original := createTestConfig(t, store, arrID, true)

	newCategory := "movies"
	newInterval := 120
	result, err := store.UpdateConfig(ctx, original.ID, &models.ArrSeedConfigUpdateParams{
		Category:            &newCategory,
		ScanIntervalMinutes: &newInterval,
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Updated fields
	assert.Equal(t, "movies", result.Category)
	assert.Equal(t, 120, result.ScanIntervalMinutes)

	// Unchanged fields
	assert.Equal(t, original.ArrInstanceID, result.ArrInstanceID)
	assert.True(t, result.Enabled)
	assert.Equal(t, original.ArrDockerPath, result.ArrDockerPath)
	assert.Equal(t, original.HostDataPath, result.HostDataPath)
	assert.Equal(t, original.TorrentSavePath, result.TorrentSavePath)
	assert.Equal(t, original.UnmonitorAfterSeed, result.UnmonitorAfterSeed)
	assert.Equal(t, original.TagAfterSeed, result.TagAfterSeed)
}

func TestArrSeedStore_UpdateConfig_NotFound(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	ctx := context.Background()

	enabled := true
	_, err := store.UpdateConfig(ctx, 9999, &models.ArrSeedConfigUpdateParams{
		Enabled: &enabled,
	})
	require.ErrorIs(t, err, models.ErrArrSeedConfigNotFound)
}

func TestArrSeedStore_DeleteConfig(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID := createTestArrInstance(t, db)
	ctx := context.Background()

	cfg := createTestConfig(t, store, arrID, true)

	err := store.DeleteConfig(ctx, cfg.ID)
	require.NoError(t, err)

	_, err = store.GetConfig(ctx, cfg.ID)
	require.ErrorIs(t, err, models.ErrArrSeedConfigNotFound)
}

func TestArrSeedStore_DeleteConfig_NotFound(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	ctx := context.Background()

	err := store.DeleteConfig(ctx, 9999)
	require.ErrorIs(t, err, models.ErrArrSeedConfigNotFound)
}

func TestArrSeedStore_UpdateConfigLastScan(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID := createTestArrInstance(t, db)
	ctx := context.Background()

	cfg := createTestConfig(t, store, arrID, true)
	require.Nil(t, cfg.LastScanAt)

	err := store.UpdateConfigLastScan(ctx, cfg.ID)
	require.NoError(t, err)

	updated, err := store.GetConfig(ctx, cfg.ID)
	require.NoError(t, err)
	assert.NotNil(t, updated.LastScanAt)
}

// --- Run Tests ---

func TestArrSeedStore_CreateRunIfNoActive(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID := createTestArrInstance(t, db)
	cfg := createTestConfig(t, store, arrID, true)
	ctx := context.Background()

	runID, err := store.CreateRunIfNoActive(ctx, cfg.ID, "manual")
	require.NoError(t, err)
	assert.Greater(t, runID, int64(0))
}

func TestArrSeedStore_CreateRunIfNoActive_BlocksWhenActive(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID := createTestArrInstance(t, db)
	cfg := createTestConfig(t, store, arrID, true)
	ctx := context.Background()

	_, err := store.CreateRunIfNoActive(ctx, cfg.ID, "manual")
	require.NoError(t, err)

	_, err = store.CreateRunIfNoActive(ctx, cfg.ID, "manual")
	require.ErrorIs(t, err, models.ErrArrSeedRunAlreadyActive)
}

func TestArrSeedStore_CreateRunIfNoActive_AllowsAfterCompletion(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID := createTestArrInstance(t, db)
	cfg := createTestConfig(t, store, arrID, true)
	ctx := context.Background()

	runID, err := store.CreateRunIfNoActive(ctx, cfg.ID, "manual")
	require.NoError(t, err)

	err = store.UpdateRunCompleted(ctx, runID, 10, 5, 3, 2)
	require.NoError(t, err)

	newRunID, err := store.CreateRunIfNoActive(ctx, cfg.ID, "scheduled")
	require.NoError(t, err)
	assert.NotEqual(t, runID, newRunID)
}

func TestArrSeedStore_GetActiveRun_NilWhenNone(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID := createTestArrInstance(t, db)
	cfg := createTestConfig(t, store, arrID, true)
	ctx := context.Background()

	run, err := store.GetActiveRun(ctx, cfg.ID)
	require.NoError(t, err)
	assert.Nil(t, run)
}

func TestArrSeedStore_GetActiveRun_ReturnsActiveRun(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID := createTestArrInstance(t, db)
	cfg := createTestConfig(t, store, arrID, true)
	ctx := context.Background()

	runID, err := store.CreateRunIfNoActive(ctx, cfg.ID, "manual")
	require.NoError(t, err)

	run, err := store.GetActiveRun(ctx, cfg.ID)
	require.NoError(t, err)
	require.NotNil(t, run)

	assert.Equal(t, runID, run.ID)
	assert.Equal(t, cfg.ID, run.ConfigID)
	assert.Equal(t, models.ArrSeedRunStatusRunning, run.Status)
	assert.Equal(t, "manual", run.TriggeredBy)
	assert.Nil(t, run.CompletedAt)
}

func TestArrSeedStore_ListRuns_DefaultsLimitTo20(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID := createTestArrInstance(t, db)
	cfg := createTestConfig(t, store, arrID, true)
	ctx := context.Background()

	// Create and complete 25 runs
	for i := 0; i < 25; i++ {
		runID, err := store.CreateRunIfNoActive(ctx, cfg.ID, "auto")
		require.NoError(t, err)
		err = store.UpdateRunCompleted(ctx, runID, 1, 1, 0, 0)
		require.NoError(t, err)
	}

	runs, err := store.ListRuns(ctx, cfg.ID, 0)
	require.NoError(t, err)
	assert.Len(t, runs, 20)
}

func TestArrSeedStore_ListRuns_DescendingOrder(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID := createTestArrInstance(t, db)
	cfg := createTestConfig(t, store, arrID, true)
	ctx := context.Background()

	var runIDs []int64
	for i := 0; i < 3; i++ {
		runID, err := store.CreateRunIfNoActive(ctx, cfg.ID, "auto")
		require.NoError(t, err)
		runIDs = append(runIDs, runID)
		err = store.UpdateRunCompleted(ctx, runID, 1, 1, 0, 0)
		require.NoError(t, err)
	}

	runs, err := store.ListRuns(ctx, cfg.ID, 10)
	require.NoError(t, err)
	require.Len(t, runs, 3)

	// Verify descending order by started_at; for same timestamps, IDs are used
	// as a tiebreaker implicitly. Just verify all runs are present and the
	// general ordering constraint holds: each run's started_at >= the next.
	gotIDs := make(map[int64]bool)
	for _, run := range runs {
		gotIDs[run.ID] = true
	}
	for _, id := range runIDs {
		assert.True(t, gotIDs[id], "expected run ID %d to be in results", id)
	}
	for i := 0; i < len(runs)-1; i++ {
		assert.True(t, !runs[i].StartedAt.Before(runs[i+1].StartedAt),
			"runs should be ordered by started_at DESC")
	}
}

func TestArrSeedStore_UpdateRunCompleted(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID := createTestArrInstance(t, db)
	cfg := createTestConfig(t, store, arrID, true)
	ctx := context.Background()

	runID, err := store.CreateRunIfNoActive(ctx, cfg.ID, "manual")
	require.NoError(t, err)

	err = store.UpdateRunCompleted(ctx, runID, 100, 50, 10, 5)
	require.NoError(t, err)

	runs, err := store.ListRuns(ctx, cfg.ID, 1)
	require.NoError(t, err)
	require.Len(t, runs, 1)

	run := runs[0]
	assert.Equal(t, models.ArrSeedRunStatusCompleted, run.Status)
	assert.Equal(t, 100, run.ItemsScanned)
	assert.Equal(t, 50, run.ItemsSearched)
	assert.Equal(t, 10, run.MatchesFound)
	assert.Equal(t, 5, run.TorrentsAdded)
	assert.NotNil(t, run.CompletedAt)
}

func TestArrSeedStore_UpdateRunFailed(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID := createTestArrInstance(t, db)
	cfg := createTestConfig(t, store, arrID, true)
	ctx := context.Background()

	runID, err := store.CreateRunIfNoActive(ctx, cfg.ID, "manual")
	require.NoError(t, err)

	err = store.UpdateRunFailed(ctx, runID, "connection refused")
	require.NoError(t, err)

	runs, err := store.ListRuns(ctx, cfg.ID, 1)
	require.NoError(t, err)
	require.Len(t, runs, 1)

	run := runs[0]
	assert.Equal(t, models.ArrSeedRunStatusFailed, run.Status)
	assert.Equal(t, "connection refused", run.ErrorMessage)
	assert.NotNil(t, run.CompletedAt)
}

func TestArrSeedStore_UpdateRunCancelled(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID := createTestArrInstance(t, db)
	cfg := createTestConfig(t, store, arrID, true)
	ctx := context.Background()

	runID, err := store.CreateRunIfNoActive(ctx, cfg.ID, "manual")
	require.NoError(t, err)

	err = store.UpdateRunCancelled(ctx, runID)
	require.NoError(t, err)

	runs, err := store.ListRuns(ctx, cfg.ID, 1)
	require.NoError(t, err)
	require.Len(t, runs, 1)

	assert.Equal(t, models.ArrSeedRunStatusCancelled, runs[0].Status)
	assert.NotNil(t, runs[0].CompletedAt)
}

func TestArrSeedStore_UpdateRunStats(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID := createTestArrInstance(t, db)
	cfg := createTestConfig(t, store, arrID, true)
	ctx := context.Background()

	runID, err := store.CreateRunIfNoActive(ctx, cfg.ID, "manual")
	require.NoError(t, err)

	err = store.UpdateRunStats(ctx, runID, 50, 25, 3, 1)
	require.NoError(t, err)

	run, err := store.GetActiveRun(ctx, cfg.ID)
	require.NoError(t, err)
	require.NotNil(t, run)

	assert.Equal(t, models.ArrSeedRunStatusRunning, run.Status)
	assert.Equal(t, 50, run.ItemsScanned)
	assert.Equal(t, 25, run.ItemsSearched)
	assert.Equal(t, 3, run.MatchesFound)
	assert.Equal(t, 1, run.TorrentsAdded)
	// Stats update should not set completed_at
	assert.Nil(t, run.CompletedAt)
}

func TestArrSeedStore_MarkActiveRunsFailed(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID1 := createTestArrInstance(t, db)
	arrID2 := createTestArrInstance(t, db)
	ctx := context.Background()

	cfg1 := createTestConfig(t, store, arrID1, true)
	cfg2 := createTestConfig(t, store, arrID2, true)

	_, err := store.CreateRunIfNoActive(ctx, cfg1.ID, "manual")
	require.NoError(t, err)
	_, err = store.CreateRunIfNoActive(ctx, cfg2.ID, "manual")
	require.NoError(t, err)

	affected, err := store.MarkActiveRunsFailed(ctx, "server shutdown")
	require.NoError(t, err)
	assert.Equal(t, int64(2), affected)

	// Both should now be failed
	run1, err := store.GetActiveRun(ctx, cfg1.ID)
	require.NoError(t, err)
	assert.Nil(t, run1) // no longer active

	run2, err := store.GetActiveRun(ctx, cfg2.ID)
	require.NoError(t, err)
	assert.Nil(t, run2)

	// Verify the runs have the failed status via list
	runs, err := store.ListRuns(ctx, cfg1.ID, 1)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, models.ArrSeedRunStatusFailed, runs[0].Status)
	assert.Equal(t, "server shutdown", runs[0].ErrorMessage)
}

// --- Item Tests ---

func TestArrSeedStore_UpsertItem_InsertsNew(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID := createTestArrInstance(t, db)
	cfg := createTestConfig(t, store, arrID, true)
	ctx := context.Background()

	err := store.UpsertItem(ctx, &models.ArrSeedItem{
		ConfigID:    cfg.ID,
		ItemType:    "episode",
		ArrFileID:   101,
		ReleaseName: "Show.S01E01.720p",
		FilePath:    "/data/show/s01e01.mkv",
		FileSize:    1024000,
		Priority:    1,
		Status:      models.ArrSeedItemStatusPending,
	})
	require.NoError(t, err)

	items, err := store.ListItems(ctx, cfg.ID, nil, 10, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)

	item := items[0]
	assert.Equal(t, cfg.ID, item.ConfigID)
	assert.Equal(t, "episode", item.ItemType)
	assert.Equal(t, 101, item.ArrFileID)
	assert.Equal(t, "Show.S01E01.720p", item.ReleaseName)
	assert.Equal(t, "/data/show/s01e01.mkv", item.FilePath)
	assert.Equal(t, int64(1024000), item.FileSize)
	assert.Equal(t, 1, item.Priority)
	assert.Equal(t, models.ArrSeedItemStatusPending, item.Status)
}

func TestArrSeedStore_UpsertItem_NilReturnsError(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	ctx := context.Background()

	err := store.UpsertItem(ctx, nil)
	require.EqualError(t, err, "item is nil")
}

func TestArrSeedStore_UpsertItem_ConflictPreservesStatus(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID := createTestArrInstance(t, db)
	cfg := createTestConfig(t, store, arrID, true)
	ctx := context.Background()

	// Insert with pending status
	err := store.UpsertItem(ctx, &models.ArrSeedItem{
		ConfigID:    cfg.ID,
		ItemType:    "episode",
		ArrFileID:   200,
		ReleaseName: "Show.S01E02.720p",
		FilePath:    "/data/show/s01e02.mkv",
		FileSize:    2048000,
		Priority:    1,
		Status:      models.ArrSeedItemStatusPending,
	})
	require.NoError(t, err)

	// Mark item as seeded
	items, err := store.ListItems(ctx, cfg.ID, nil, 10, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)
	err = store.UpdateItemStatus(ctx, items[0].ID, models.ArrSeedItemStatusSeeded, "abc123", "indexer1", "")
	require.NoError(t, err)

	// Upsert again with a new status -- the ON CONFLICT should preserve existing status
	err = store.UpsertItem(ctx, &models.ArrSeedItem{
		ConfigID:    cfg.ID,
		ItemType:    "episode",
		ArrFileID:   200,
		ReleaseName: "Show.S01E02.720p.REPACK",
		FilePath:    "/data/show/s01e02.repack.mkv",
		FileSize:    2100000,
		Priority:    2,
		Status:      models.ArrSeedItemStatusPending,
	})
	require.NoError(t, err)

	items, err = store.ListItems(ctx, cfg.ID, nil, 10, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)

	item := items[0]
	// Status preserved from original
	assert.Equal(t, models.ArrSeedItemStatusSeeded, item.Status)
	// Other fields updated
	assert.Equal(t, "Show.S01E02.720p.REPACK", item.ReleaseName)
	assert.Equal(t, "/data/show/s01e02.repack.mkv", item.FilePath)
	assert.Equal(t, int64(2100000), item.FileSize)
	assert.Equal(t, 2, item.Priority)
}

func TestArrSeedStore_UpdateItemStatus(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID := createTestArrInstance(t, db)
	cfg := createTestConfig(t, store, arrID, true)
	ctx := context.Background()

	err := store.UpsertItem(ctx, &models.ArrSeedItem{
		ConfigID:    cfg.ID,
		ItemType:    "movie",
		ArrFileID:   300,
		ReleaseName: "Movie.2024.1080p",
		FilePath:    "/data/movie.mkv",
		FileSize:    5000000,
		Priority:    1,
		Status:      models.ArrSeedItemStatusPending,
	})
	require.NoError(t, err)

	items, err := store.ListItems(ctx, cfg.ID, nil, 10, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)

	err = store.UpdateItemStatus(ctx, items[0].ID, models.ArrSeedItemStatusMatched, "deadbeef", "torrentleech", "")
	require.NoError(t, err)

	items, err = store.ListItems(ctx, cfg.ID, nil, 10, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)

	item := items[0]
	assert.Equal(t, models.ArrSeedItemStatusMatched, item.Status)
	assert.Equal(t, "deadbeef", item.TorrentHash)
	assert.Equal(t, "torrentleech", item.IndexerName)
	assert.Empty(t, item.ErrorMessage)
	assert.NotNil(t, item.LastSearchedAt)
}

func TestArrSeedStore_UpdateItemStatus_NullForEmptyStrings(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID := createTestArrInstance(t, db)
	cfg := createTestConfig(t, store, arrID, true)
	ctx := context.Background()

	err := store.UpsertItem(ctx, &models.ArrSeedItem{
		ConfigID:    cfg.ID,
		ItemType:    "movie",
		ArrFileID:   400,
		ReleaseName: "Movie.2023.720p",
		FilePath:    "/data/movie2.mkv",
		FileSize:    3000000,
		Priority:    1,
		Status:      models.ArrSeedItemStatusPending,
	})
	require.NoError(t, err)

	items, err := store.ListItems(ctx, cfg.ID, nil, 10, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)

	// First set some values
	err = store.UpdateItemStatus(ctx, items[0].ID, models.ArrSeedItemStatusMatched, "hash123", "indexer", "some error")
	require.NoError(t, err)

	// Now set them back to empty strings (should store as NULL)
	err = store.UpdateItemStatus(ctx, items[0].ID, models.ArrSeedItemStatusNoMatch, "", "", "")
	require.NoError(t, err)

	items, err = store.ListItems(ctx, cfg.ID, nil, 10, 0)
	require.NoError(t, err)
	require.Len(t, items, 1)

	item := items[0]
	assert.Equal(t, models.ArrSeedItemStatusNoMatch, item.Status)
	assert.Empty(t, item.TorrentHash)
	assert.Empty(t, item.IndexerName)
	assert.Empty(t, item.ErrorMessage)
}

func TestArrSeedStore_ListItems_AllWithNilStatus(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID := createTestArrInstance(t, db)
	cfg := createTestConfig(t, store, arrID, true)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		err := store.UpsertItem(ctx, &models.ArrSeedItem{
			ConfigID:    cfg.ID,
			ItemType:    "episode",
			ArrFileID:   500 + i,
			ReleaseName: "Show.S01E0" + string(rune('1'+i)) + ".720p",
			FilePath:    "/data/ep" + string(rune('1'+i)) + ".mkv",
			FileSize:    1000000,
			Priority:    1,
			Status:      models.ArrSeedItemStatusPending,
		})
		require.NoError(t, err)
	}

	items, err := store.ListItems(ctx, cfg.ID, nil, 100, 0)
	require.NoError(t, err)
	assert.Len(t, items, 3)
}

func TestArrSeedStore_ListItems_WithStatusFilter(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID := createTestArrInstance(t, db)
	cfg := createTestConfig(t, store, arrID, true)
	ctx := context.Background()

	// Insert two items
	err := store.UpsertItem(ctx, &models.ArrSeedItem{
		ConfigID: cfg.ID, ItemType: "episode", ArrFileID: 600,
		ReleaseName: "Ep1", FilePath: "/ep1", FileSize: 100, Priority: 1,
		Status: models.ArrSeedItemStatusPending,
	})
	require.NoError(t, err)

	err = store.UpsertItem(ctx, &models.ArrSeedItem{
		ConfigID: cfg.ID, ItemType: "episode", ArrFileID: 601,
		ReleaseName: "Ep2", FilePath: "/ep2", FileSize: 100, Priority: 1,
		Status: models.ArrSeedItemStatusPending,
	})
	require.NoError(t, err)

	// Mark one as seeded
	allItems, err := store.ListItems(ctx, cfg.ID, nil, 10, 0)
	require.NoError(t, err)
	require.Len(t, allItems, 2)

	err = store.UpdateItemStatus(ctx, allItems[0].ID, models.ArrSeedItemStatusSeeded, "", "", "")
	require.NoError(t, err)

	// Filter by pending only
	pendingStatus := models.ArrSeedItemStatusPending
	items, err := store.ListItems(ctx, cfg.ID, &pendingStatus, 10, 0)
	require.NoError(t, err)
	assert.Len(t, items, 1)
	assert.Equal(t, models.ArrSeedItemStatusPending, items[0].Status)
}

func TestArrSeedStore_ListItems_DefaultsLimitTo100(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID := createTestArrInstance(t, db)
	cfg := createTestConfig(t, store, arrID, true)
	ctx := context.Background()

	// Insert 105 items
	for i := 0; i < 105; i++ {
		err := store.UpsertItem(ctx, &models.ArrSeedItem{
			ConfigID: cfg.ID, ItemType: "episode", ArrFileID: 700 + i,
			ReleaseName: "Ep", FilePath: "/ep", FileSize: 100, Priority: 1,
			Status: models.ArrSeedItemStatusPending,
		})
		require.NoError(t, err)
	}

	items, err := store.ListItems(ctx, cfg.ID, nil, 0, 0)
	require.NoError(t, err)
	assert.Len(t, items, 100)
}

func TestArrSeedStore_GetPendingItems_FiltersCorrectStatuses(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID := createTestArrInstance(t, db)
	cfg := createTestConfig(t, store, arrID, true)
	ctx := context.Background()

	statuses := []models.ArrSeedItemStatus{
		models.ArrSeedItemStatusPending,
		models.ArrSeedItemStatusSearched,
		models.ArrSeedItemStatusMatched,
		models.ArrSeedItemStatusSeeded,
		models.ArrSeedItemStatusNoMatch,
		models.ArrSeedItemStatusError,
	}

	for i, status := range statuses {
		err := store.UpsertItem(ctx, &models.ArrSeedItem{
			ConfigID: cfg.ID, ItemType: "episode", ArrFileID: 800 + i,
			ReleaseName: "Ep", FilePath: "/ep", FileSize: 100, Priority: 1,
			Status: status,
		})
		require.NoError(t, err)
	}

	pending, err := store.GetPendingItems(ctx, cfg.ID)
	require.NoError(t, err)

	// Should include pending, no_match, error (3 out of 6)
	assert.Len(t, pending, 3)

	statusSet := make(map[models.ArrSeedItemStatus]bool)
	for _, item := range pending {
		statusSet[item.Status] = true
	}
	assert.True(t, statusSet[models.ArrSeedItemStatusPending])
	assert.True(t, statusSet[models.ArrSeedItemStatusNoMatch])
	assert.True(t, statusSet[models.ArrSeedItemStatusError])
	assert.False(t, statusSet[models.ArrSeedItemStatusSearched])
	assert.False(t, statusSet[models.ArrSeedItemStatusMatched])
	assert.False(t, statusSet[models.ArrSeedItemStatusSeeded])
}

func TestArrSeedStore_GetPendingItems_Ordering(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID := createTestArrInstance(t, db)
	cfg := createTestConfig(t, store, arrID, true)
	ctx := context.Background()

	// Item with priority 2, no last_searched_at
	err := store.UpsertItem(ctx, &models.ArrSeedItem{
		ConfigID: cfg.ID, ItemType: "episode", ArrFileID: 901,
		ReleaseName: "LowPri", FilePath: "/lp", FileSize: 100, Priority: 2,
		Status: models.ArrSeedItemStatusPending,
	})
	require.NoError(t, err)

	// Item with priority 1, no last_searched_at
	err = store.UpsertItem(ctx, &models.ArrSeedItem{
		ConfigID: cfg.ID, ItemType: "episode", ArrFileID: 902,
		ReleaseName: "HighPri", FilePath: "/hp", FileSize: 100, Priority: 1,
		Status: models.ArrSeedItemStatusPending,
	})
	require.NoError(t, err)

	// Item with priority 1, has last_searched_at (searched before)
	err = store.UpsertItem(ctx, &models.ArrSeedItem{
		ConfigID: cfg.ID, ItemType: "episode", ArrFileID: 903,
		ReleaseName: "HighPriSearched", FilePath: "/hps", FileSize: 100, Priority: 1,
		Status: models.ArrSeedItemStatusPending,
	})
	require.NoError(t, err)

	// Set last_searched_at on the third item by updating its status
	allItems, err := store.ListItems(ctx, cfg.ID, nil, 10, 0)
	require.NoError(t, err)
	for _, item := range allItems {
		if item.ReleaseName == "HighPriSearched" {
			err = store.UpdateItemStatus(ctx, item.ID, models.ArrSeedItemStatusPending, "", "", "")
			require.NoError(t, err)
		}
	}

	pending, err := store.GetPendingItems(ctx, cfg.ID)
	require.NoError(t, err)
	require.Len(t, pending, 3)

	// Order: priority ASC, last_searched_at ASC NULLS FIRST, id ASC
	// Priority 1 items come first (HighPri with NULL last_searched, then HighPriSearched with a timestamp)
	// Then priority 2 (LowPri)
	assert.Equal(t, "HighPri", pending[0].ReleaseName)
	assert.Equal(t, "HighPriSearched", pending[1].ReleaseName)
	assert.Equal(t, "LowPri", pending[2].ReleaseName)
}

func TestArrSeedStore_DeleteItemsForConfig(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID := createTestArrInstance(t, db)
	cfg := createTestConfig(t, store, arrID, true)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		err := store.UpsertItem(ctx, &models.ArrSeedItem{
			ConfigID: cfg.ID, ItemType: "episode", ArrFileID: 1000 + i,
			ReleaseName: "Ep", FilePath: "/ep", FileSize: 100, Priority: 1,
			Status: models.ArrSeedItemStatusPending,
		})
		require.NoError(t, err)
	}

	err := store.DeleteItemsForConfig(ctx, cfg.ID)
	require.NoError(t, err)

	items, err := store.ListItems(ctx, cfg.ID, nil, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, items)
}

func TestArrSeedStore_DeleteItemsByIDs(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	arrID := createTestArrInstance(t, db)
	cfg := createTestConfig(t, store, arrID, true)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		err := store.UpsertItem(ctx, &models.ArrSeedItem{
			ConfigID: cfg.ID, ItemType: "episode", ArrFileID: 1100 + i,
			ReleaseName: "Ep", FilePath: "/ep", FileSize: 100, Priority: 1,
			Status: models.ArrSeedItemStatusPending,
		})
		require.NoError(t, err)
	}

	items, err := store.ListItems(ctx, cfg.ID, nil, 10, 0)
	require.NoError(t, err)
	require.Len(t, items, 3)

	// Delete the first two
	err = store.DeleteItemsByIDs(ctx, []int64{items[0].ID, items[1].ID})
	require.NoError(t, err)

	remaining, err := store.ListItems(ctx, cfg.ID, nil, 10, 0)
	require.NoError(t, err)
	assert.Len(t, remaining, 1)
	assert.Equal(t, items[2].ID, remaining[0].ID)
}

func TestArrSeedStore_DeleteItemsByIDs_EmptySliceIsNoOp(t *testing.T) {
	db := setupArrSeedTestDB(t)
	store := models.NewArrSeedStore(db)
	ctx := context.Background()

	err := store.DeleteItemsByIDs(ctx, []int64{})
	require.NoError(t, err)
}
