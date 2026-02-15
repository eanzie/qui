// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package crossseed

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/moistari/rls"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/qui/internal/models"
	"github.com/autobrr/qui/internal/services/arr"
	"github.com/autobrr/qui/internal/services/jackett"
	"github.com/autobrr/qui/pkg/releasematch"
)

// ArrInstanceProvider loads and decrypts ARR instance data.
type ArrInstanceProvider interface {
	Get(ctx context.Context, id int) (*models.ArrInstance, error)
	GetDecryptedAPIKey(instance *models.ArrInstance) (string, error)
	GetDecryptedBasicPassword(instance *models.ArrInstance) (string, error)
}

// ArrSeedRunner manages ARR library scanning and cross-seeding as a feature
// within the crossseed Service.
type ArrSeedRunner struct {
	svc              *Service
	store            *models.ArrSeedStore
	arrInstanceStore ArrInstanceProvider

	scanner          *arrSeedScanner
	searcher         *arrSeedSearcher
	injector         *arrSeedInjector
	postSeedExecutor *arrSeedPostSeedExecutor

	// Per-config mutex to prevent overlapping scans.
	configMu map[int]*sync.Mutex
	mu       sync.Mutex

	// In-memory cancel handles keyed by runID.
	cancelFuncs map[int64]context.CancelFunc
	cancelMu    sync.Mutex

	// Graceful stop flags keyed by runID.
	stopFlags map[int64]struct{}

	// In-memory progress tracking.
	progress *arrSeedProgressTracker

	// Scheduler control.
	schedulerCtx    context.Context
	schedulerCancel context.CancelFunc
	schedulerWg     sync.WaitGroup
}

// InitArrSeed initializes the ArrSeed runner within the crossseed Service.
func (s *Service) InitArrSeed(
	store *models.ArrSeedStore,
	arrInstanceStore ArrInstanceProvider,
) {
	r := &ArrSeedRunner{
		svc:              s,
		store:            store,
		arrInstanceStore: arrInstanceStore,
		scanner:          &arrSeedScanner{},
		searcher:         &arrSeedSearcher{jackettService: s.jackettService},
		injector:         &arrSeedInjector{svc: s},
		postSeedExecutor: &arrSeedPostSeedExecutor{},
		configMu:         make(map[int]*sync.Mutex),
		cancelFuncs:      make(map[int64]context.CancelFunc),
		stopFlags:        make(map[int64]struct{}),
		progress:         newArrSeedProgressTracker(),
	}
	s.arrSeedRunner = r
}

// StartArrSeed starts the ArrSeed scheduler loop.
func (s *Service) StartArrSeed(ctx context.Context) error {
	if s.arrSeedRunner == nil {
		return nil
	}
	return s.arrSeedRunner.start(ctx)
}

// StopArrSeed stops the ArrSeed scheduler.
func (s *Service) StopArrSeed() {
	if s.arrSeedRunner == nil {
		return
	}
	s.arrSeedRunner.stop()
}

// ArrSeedStartManualScan starts a manual scan for a config.
func (s *Service) ArrSeedStartManualScan(ctx context.Context, configID int) (int64, error) {
	if s.arrSeedRunner == nil {
		return 0, errors.New("arrseed not initialized")
	}
	return s.arrSeedRunner.startManualScan(ctx, configID)
}

// ArrSeedStopScan gracefully stops a scan (finishes current item, then stops).
func (s *Service) ArrSeedStopScan(ctx context.Context, configID int) error {
	if s.arrSeedRunner == nil {
		return errors.New("arrseed not initialized")
	}
	return s.arrSeedRunner.stopScan(ctx, configID)
}

// ArrSeedCancelScan immediately kills a running scan.
func (s *Service) ArrSeedCancelScan(ctx context.Context, configID int) error {
	if s.arrSeedRunner == nil {
		return errors.New("arrseed not initialized")
	}
	return s.arrSeedRunner.cancelScan(ctx, configID)
}

// ArrSeedGetScanProgress returns the live progress for a config.
func (s *Service) ArrSeedGetScanProgress(configID int) *ArrSeedScanProgress {
	if s.arrSeedRunner == nil {
		return nil
	}
	return s.arrSeedRunner.progress.getByConfigID(configID)
}

func (r *ArrSeedRunner) start(ctx context.Context) error {
	if _, err := r.store.MarkActiveRunsFailed(ctx, "service restarted"); err != nil {
		log.Error().Err(err).Msg("arrseed: failed to recover stuck runs")
	}

	r.schedulerCtx, r.schedulerCancel = context.WithCancel(ctx)
	r.schedulerWg.Add(1)
	go r.runScheduler()
	log.Info().Msg("arrseed: scheduler started")
	return nil
}

func (r *ArrSeedRunner) stop() {
	if r.schedulerCancel != nil {
		r.schedulerCancel()
	}
	r.schedulerWg.Wait()
	log.Info().Msg("arrseed: scheduler stopped")
}

func (r *ArrSeedRunner) runScheduler() {
	defer r.schedulerWg.Done()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-r.schedulerCtx.Done():
			return
		case <-ticker.C:
			r.checkScheduledScans()
		}
	}
}

func (r *ArrSeedRunner) checkScheduledScans() {
	ctx := r.schedulerCtx

	settings, err := r.store.GetSettings(ctx)
	if err != nil {
		log.Error().Err(err).Msg("arrseed: failed to get settings")
		return
	}
	if settings == nil || !settings.Enabled {
		return
	}

	configs, err := r.store.ListEnabledConfigs(ctx)
	if err != nil {
		log.Error().Err(err).Msg("arrseed: failed to list enabled configs")
		return
	}

	log.Trace().Int("enabledConfigs", len(configs)).Msg("arrseed: scheduler tick")

	// Run due configs sequentially to avoid overwhelming arr/indexer APIs.
	for _, cfg := range configs {
		if ctx.Err() != nil {
			return
		}
		if r.isDueForScan(cfg) {
			log.Info().Int("configID", cfg.ID).Str("arrInstance", cfg.ArrInstanceName).Msg("arrseed: config due for scan, triggering")
			r.triggerScheduledScan(cfg.ID)
		}
	}
}

func (r *ArrSeedRunner) isDueForScan(cfg *models.ArrSeedInstanceConfig) bool {
	if cfg.LastScanAt == nil {
		return true
	}
	nextScan := cfg.LastScanAt.Add(time.Duration(cfg.ScanIntervalMinutes) * time.Minute)
	return time.Now().After(nextScan)
}

func (r *ArrSeedRunner) triggerScheduledScan(configID int) {
	ctx := r.schedulerCtx

	runID, err := r.store.CreateRunIfNoActive(ctx, configID, "schedule")
	if err != nil {
		if !errors.Is(err, models.ErrArrSeedRunAlreadyActive) {
			log.Error().Err(err).Int("configID", configID).Msg("arrseed: failed to create scheduled run")
		}
		return
	}

	// Run synchronously so scheduled scans execute one at a time.
	runCtx, cancel := context.WithCancel(ctx)
	r.cancelMu.Lock()
	r.cancelFuncs[runID] = cancel
	r.cancelMu.Unlock()

	defer func() {
		r.cancelMu.Lock()
		delete(r.cancelFuncs, runID)
		delete(r.stopFlags, runID)
		r.cancelMu.Unlock()
	}()

	r.executeScan(runCtx, configID, runID)
}

func (r *ArrSeedRunner) startRun(parent context.Context, configID int, runID int64) {
	runCtx, cancel := context.WithCancel(parent)
	r.cancelMu.Lock()
	r.cancelFuncs[runID] = cancel
	r.cancelMu.Unlock()

	go func() {
		defer func() {
			r.cancelMu.Lock()
			delete(r.cancelFuncs, runID)
			delete(r.stopFlags, runID)
			r.cancelMu.Unlock()
		}()
		r.executeScan(runCtx, configID, runID)
	}()
}

func (r *ArrSeedRunner) startManualScan(ctx context.Context, configID int) (int64, error) {
	runID, err := r.store.CreateRunIfNoActive(ctx, configID, "manual")
	if err != nil {
		return 0, fmt.Errorf("create run: %w", err)
	}

	r.startRun(context.Background(), configID, runID)
	return runID, nil
}

func (r *ArrSeedRunner) stopScan(ctx context.Context, configID int) error {
	run, err := r.store.GetActiveRun(ctx, configID)
	if err != nil {
		return fmt.Errorf("get active run: %w", err)
	}
	if run == nil {
		return nil
	}

	r.cancelMu.Lock()
	r.stopFlags[run.ID] = struct{}{}
	r.cancelMu.Unlock()

	// Update progress to show stopping state
	if p := r.progress.get(run.ID); p != nil {
		p.Status = "stopping"
		r.progress.set(run.ID, p)
	}

	return nil
}

func (r *ArrSeedRunner) cancelScan(ctx context.Context, configID int) error {
	run, err := r.store.GetActiveRun(ctx, configID)
	if err != nil {
		return fmt.Errorf("get active run: %w", err)
	}
	if run == nil {
		return nil
	}

	r.cancelMu.Lock()
	cancel, ok := r.cancelFuncs[run.ID]
	delete(r.stopFlags, run.ID)
	r.cancelMu.Unlock()

	if ok {
		cancel()
	}

	r.progress.clear(run.ID)

	return r.store.UpdateRunCancelled(context.Background(), run.ID)
}

func (r *ArrSeedRunner) isStopped(runID int64) bool {
	r.cancelMu.Lock()
	_, stopped := r.stopFlags[runID]
	r.cancelMu.Unlock()
	return stopped
}

func (r *ArrSeedRunner) getConfigMutex(configID int) *sync.Mutex {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.configMu[configID]
	if !ok {
		m = &sync.Mutex{}
		r.configMu[configID] = m
	}
	return m
}

// executeScan performs the actual scan for a config.
func (r *ArrSeedRunner) executeScan(ctx context.Context, configID int, runID int64) {
	defer r.progress.clear(runID)

	l := log.With().
		Int("configID", configID).
		Int64("runID", runID).
		Logger()

	l.Info().Msg("arrseed: acquiring config lock")
	cfgMu := r.getConfigMutex(configID)
	cfgMu.Lock()
	defer cfgMu.Unlock()

	l.Info().Msg("arrseed: loading config from database")
	config, err := r.store.GetConfig(ctx, configID)
	if err != nil {
		l.Error().Err(err).Msg("arrseed: failed to load config")
		_ = r.store.UpdateRunFailed(context.Background(), runID, fmt.Sprintf("load config: %v", err))
		return
	}
	l.Info().
		Str("arrInstance", config.ArrInstanceName).
		Str("arrType", config.ArrInstanceType).
		Str("category", config.Category).
		Str("arrDockerPath", config.ArrDockerPath).
		Str("hostDataPath", config.HostDataPath).
		Str("torrentSavePath", config.TorrentSavePath).
		Msg("arrseed: config loaded")

	settings, err := r.store.GetSettings(ctx)
	if err != nil || settings == nil {
		l.Error().Err(err).Msg("arrseed: failed to load settings")
		_ = r.store.UpdateRunFailed(context.Background(), runID, "failed to load settings")
		return
	}

	// Load shared cross-seed settings (startPaused, sizeTolerance, tags)
	csSettings, err := r.svc.GetAutomationSettings(ctx)
	if err != nil {
		l.Error().Err(err).Msg("arrseed: failed to load cross-seed settings")
		_ = r.store.UpdateRunFailed(context.Background(), runID, "failed to load cross-seed settings")
		return
	}

	injOpts := &arrSeedInjectionOptions{
		StartPaused:                  csSettings.StartPaused,
		Tags:                         csSettings.ArrSeedTags,
		EnableSeasonPackUpgrade:      settings.EnableSeasonPackUpgrade,
		SizeMismatchTolerancePercent: csSettings.SizeMismatchTolerancePercent,
	}

	l.Info().
		Int("searchDelay", settings.SearchDelaySeconds).
		Int("maxItemsPerRun", settings.MaxItemsPerRun).
		Float64("sizeTolerance", csSettings.SizeMismatchTolerancePercent).
		Bool("startPaused", injOpts.StartPaused).
		Strs("tags", injOpts.Tags).
		Msg("arrseed: settings loaded")

	// Load ARR instance
	l.Info().Int("arrInstanceID", config.ArrInstanceID).Msg("arrseed: loading ARR instance")
	arrInstance, err := r.arrInstanceStore.Get(ctx, config.ArrInstanceID)
	if err != nil {
		l.Error().Err(err).Msg("arrseed: failed to load ARR instance")
		_ = r.store.UpdateRunFailed(context.Background(), runID, fmt.Sprintf("load ARR instance: %v", err))
		return
	}
	l.Info().
		Str("baseURL", arrInstance.BaseURL).
		Str("type", string(arrInstance.Type)).
		Msg("arrseed: ARR instance loaded")

	apiKey, err := r.arrInstanceStore.GetDecryptedAPIKey(arrInstance)
	if err != nil {
		l.Error().Err(err).Msg("arrseed: failed to decrypt API key")
		_ = r.store.UpdateRunFailed(context.Background(), runID, "failed to decrypt API key")
		return
	}
	l.Debug().Msg("arrseed: API key decrypted successfully")

	var basicPass *string
	if arrInstance.BasicPasswordEncrypted != nil && *arrInstance.BasicPasswordEncrypted != "" {
		decryptedPass, passErr := r.arrInstanceStore.GetDecryptedBasicPassword(arrInstance)
		if passErr != nil {
			l.Warn().Err(passErr).Msg("arrseed: failed to decrypt basic auth password")
		} else {
			basicPass = &decryptedPass
			l.Debug().Msg("arrseed: basic auth password decrypted")
		}
	}

	arrClient := arr.NewClient(arrInstance.BaseURL, apiKey, arrInstance.BasicUsername, basicPass, arrInstance.Type, arrInstance.TimeoutSeconds)
	l.Info().Str("type", string(arrInstance.Type)).Str("baseURL", arrInstance.BaseURL).Msg("arrseed: ARR API client created, starting library scan")

	// Fetch quality profiles for priority classification
	var profileCutoffScores map[int]int
	var profileMinScores map[int]int
	profiles, profErr := arrClient.GetQualityProfiles(ctx)
	if profErr != nil {
		l.Warn().Err(profErr).Msg("arrseed: failed to fetch quality profiles, priority scoring will be limited")
	} else {
		profileCutoffScores = make(map[int]int, len(profiles))
		profileMinScores = make(map[int]int, len(profiles))
		for _, p := range profiles {
			profileCutoffScores[p.ID] = p.CutoffFormatScore
			profileMinScores[p.ID] = p.MinFormatScore
			l.Info().
				Int("profileID", p.ID).
				Str("name", p.Name).
				Int("cutoffFormatScore", p.CutoffFormatScore).
				Int("minFormatScore", p.MinFormatScore).
				Bool("upgradeAllowed", p.UpgradeAllowed).
				Msg("arrseed: quality profile loaded")
		}
		l.Info().Int("profiles", len(profileCutoffScores)).Msg("arrseed: quality profiles loaded")
	}

	progress := &ArrSeedScanProgress{
		RunID:    runID,
		ConfigID: configID,
		Status:   "running",
		Phase:    "scanning",
	}
	r.progress.set(runID, progress)

	// Scan the ARR library
	var items []*ArrSeedMediaItem
	switch arrInstance.Type {
	case models.ArrInstanceTypeSonarr:
		l.Info().Msg("arrseed: scanning Sonarr instance for episode files")
		items, err = r.scanner.scanSonarrInstance(ctx, arrClient, config, &l)
	case models.ArrInstanceTypeRadarr:
		l.Info().Msg("arrseed: scanning Radarr instance for movie files")
		items, err = r.scanner.scanRadarrInstance(ctx, arrClient, config, &l)
	default:
		err = fmt.Errorf("unsupported ARR type: %s", arrInstance.Type)
	}

	if err != nil {
		if ctx.Err() != nil {
			l.Info().Msg("arrseed: scan cancelled")
			return
		}
		l.Error().Err(err).Msg("arrseed: failed to scan ARR library")
		_ = r.store.UpdateRunFailed(context.Background(), runID, fmt.Sprintf("scan ARR library: %v", err))
		return
	}

	l.Info().Int("items", len(items)).Msg("arrseed: library scan complete, upserting items to database")

	// Classify priority on scanned items
	priorityCounts := map[int]int{}
	for _, item := range items {
		cutoffScore := 0
		if profileCutoffScores != nil {
			cutoffScore = profileCutoffScores[item.QualityProfileID]
		}
		if profileMinScores != nil {
			item.MinFormatScore = profileMinScores[item.QualityProfileID]
		}
		item.Priority = ArrSeedClassifyPriority(item, cutoffScore)
		priorityCounts[item.Priority]++

		if item.ItemType == "season_pack" {
			l.Info().
				Str("title", item.Title).
				Str("release", item.ReleaseName).
				Int("season", item.SeasonNumber).
				Int("customFormatScore", item.CustomFormatScore).
				Int("cutoffFormatScore", cutoffScore).
				Int("qualityProfileID", item.QualityProfileID).
				Int("priority", item.Priority).
				Int64("size", item.FileSize).
				Msg("arrseed: season pack classified")
		}
	}
	l.Info().
		Int("priority1_highScore", priorityCounts[ArrSeedPriorityHighScore]).
		Int("priority2_normal", priorityCounts[ArrSeedPriorityNormal]).
		Int("priority3_episode", priorityCounts[ArrSeedPriorityEpisode]).
		Msg("arrseed: priority classification complete")

	// Upsert items to DB
	upsertCount := 0
	for _, item := range items {
		if ctx.Err() != nil {
			return
		}
		if upsertErr := r.store.UpsertItem(ctx, &models.ArrSeedItem{
			ConfigID:    item.ConfigID,
			ItemType:    item.ItemType,
			ArrFileID:   item.ArrFileID,
			ReleaseName: item.ReleaseName,
			FilePath:    item.HostFilePath,
			FileSize:    item.FileSize,
			Priority:    item.Priority,
			Status:      models.ArrSeedItemStatusPending,
		}); upsertErr != nil {
			l.Warn().Err(upsertErr).Str("release", item.ReleaseName).Msg("arrseed: failed to upsert item")
		} else {
			upsertCount++
		}
	}
	l.Info().Int("upserted", upsertCount).Int("total", len(items)).Msg("arrseed: items upserted to database")

	// Get pending items
	pendingItems, err := r.store.GetPendingItems(ctx, configID)
	if err != nil {
		l.Error().Err(err).Msg("arrseed: failed to get pending items")
		_ = r.store.UpdateRunFailed(context.Background(), runID, fmt.Sprintf("get pending items: %v", err))
		return
	}
	l.Info().Int("pendingItems", len(pendingItems)).Msg("arrseed: pending items loaded from database")

	// Prune items whose file path is outside the configured host data path.
	// This cleans up stale items from prior scans with different path configs.
	if config.HostDataPath != "" {
		var validItems []*models.ArrSeedItem
		var staleIDs []int64
		for _, item := range pendingItems {
			if arrSeedPathUnder(item.FilePath, config.HostDataPath) {
				validItems = append(validItems, item)
			} else {
				staleIDs = append(staleIDs, item.ID)
			}
		}
		if len(staleIDs) > 0 {
			l.Info().Int("stale", len(staleIDs)).Str("hostDataPath", config.HostDataPath).Msg("arrseed: removing items outside configured path")
			if delErr := r.store.DeleteItemsByIDs(ctx, staleIDs); delErr != nil {
				l.Warn().Err(delErr).Msg("arrseed: failed to delete stale items")
			}
			pendingItems = validItems
		}
	}

	// Filter by type and quality gate
	pendingPriorityCounts := map[int]int{}
	for _, item := range pendingItems {
		pendingPriorityCounts[item.Priority]++
	}
	l.Info().
		Int("priority1_highScore", pendingPriorityCounts[ArrSeedPriorityHighScore]).
		Int("priority2_normal", pendingPriorityCounts[ArrSeedPriorityNormal]).
		Int("priority3_episode", pendingPriorityCounts[ArrSeedPriorityEpisode]).
		Bool("enableHighScoreOnly", settings.EnableHighScoreOnly).
		Bool("enableEpisode", settings.EnableEpisode).
		Msg("arrseed: pending items priority distribution before filter")

	pendingItems = arrSeedFilterPendingItems(pendingItems, settings)
	l.Info().Int("afterFilter", len(pendingItems)).Msg("arrseed: items after priority filtering")

	if settings.MaxItemsPerRun > 0 && len(pendingItems) > settings.MaxItemsPerRun {
		l.Info().Int("limit", settings.MaxItemsPerRun).Int("total", len(pendingItems)).Msg("arrseed: applying max items per run limit")
		pendingItems = pendingItems[:settings.MaxItemsPerRun]
	}

	progress.Phase = "searching"
	progress.ItemsTotal = len(pendingItems)
	r.progress.set(runID, progress)
	l.Info().Int("itemsToProcess", len(pendingItems)).Msg("arrseed: entering search phase")

	var stats struct {
		itemsScanned  int
		itemsSearched int
		matchesFound  int
		torrentsAdded int
	}
	stats.itemsScanned = len(items)

	// Build a map from (item_type, arr_file_id) -> media item for external IDs
	mediaItemMap := make(map[string]*ArrSeedMediaItem, len(items))
	for _, item := range items {
		key := fmt.Sprintf("%s:%d", item.ItemType, item.ArrFileID)
		mediaItemMap[key] = item
	}

	// Process each pending item
	for i, dbItem := range pendingItems {
		if ctx.Err() != nil {
			l.Info().Msg("arrseed: scan cancelled during search phase")
			_ = r.store.UpdateRunCancelled(context.Background(), runID)
			return
		}

		if r.isStopped(runID) {
			l.Info().Int("processed", i).Int("total", len(pendingItems)).Msg("arrseed: scan stopped, finishing current item")
			break
		}

		progress.ItemsProcessed = i + 1
		progress.CurrentItem = dbItem.ReleaseName
		r.progress.set(runID, progress)

		l.Info().
			Int("item", i+1).
			Int("total", len(pendingItems)).
			Str("release", dbItem.ReleaseName).
			Str("type", dbItem.ItemType).
			Int64("size", dbItem.FileSize).
			Str("filePath", dbItem.FilePath).
			Msg("arrseed: processing item")

		mediaKey := fmt.Sprintf("%s:%d", dbItem.ItemType, dbItem.ArrFileID)
		mediaItem := mediaItemMap[mediaKey]
		if mediaItem == nil {
			l.Debug().Str("release", dbItem.ReleaseName).Msg("arrseed: no media item in scan cache, building from DB data")
			mediaItem = &ArrSeedMediaItem{
				ConfigID:     dbItem.ConfigID,
				ItemType:     dbItem.ItemType,
				ArrFileID:    dbItem.ArrFileID,
				ReleaseName:  dbItem.ReleaseName,
				HostFilePath: dbItem.FilePath,
				FileSize:     dbItem.FileSize,
			}
		} else {
			l.Debug().
				Str("imdbID", mediaItem.ExternalIDs.IMDbID).
				Int("tvdbID", mediaItem.ExternalIDs.TVDbID).
				Int("tmdbID", mediaItem.ExternalIDs.TMDbID).
				Msg("arrseed: found media item with external IDs")
		}

		l.Info().Str("release", dbItem.ReleaseName).Msg("arrseed: searching indexers")
		results, searchErr := r.searcher.searchRelease(ctx, mediaItem, &l)
		if searchErr != nil {
			if ctx.Err() != nil {
				return
			}
			l.Warn().Err(searchErr).Str("release", dbItem.ReleaseName).Msg("arrseed: search failed")
			_ = r.store.UpdateItemStatus(ctx, dbItem.ID, models.ArrSeedItemStatusError, "", "", searchErr.Error())
			continue
		}
		stats.itemsSearched++

		if len(results) == 0 {
			l.Info().Str("release", dbItem.ReleaseName).Msg("arrseed: no search results found")
			_ = r.store.UpdateItemStatus(ctx, dbItem.ID, models.ArrSeedItemStatusNoMatch, "", "", "")
			continue
		}

		l.Info().
			Str("release", dbItem.ReleaseName).
			Int("results", len(results)).
			Msg("arrseed: search results found, filtering by release match")

		// Filter search results by release match AND size tolerance.
		sourceRelease := rls.ParseString(dbItem.ReleaseName)
		var matched []jackett.SearchResult
		sizeFiltered := 0
		for _, result := range results {
			candidateRelease := rls.ParseString(result.Title)
			if !releasematch.ReleasesMatch(&sourceRelease, &candidateRelease, releasematch.MatchOptions{}) {
				l.Debug().
					Str("source", dbItem.ReleaseName).
					Str("candidate", result.Title).
					Str("indexer", result.Indexer).
					Msg("arrseed: candidate filtered out by release matching")
				continue
			}
			allowPartial := mediaItem.ItemType == "season_pack" &&
				injOpts.EnableSeasonPackUpgrade &&
				mediaItem.CustomFormatScore >= mediaItem.MinFormatScore
			if !allowPartial &&
				result.Size > 0 && mediaItem.FileSize > 0 &&
				!isSizeWithinTolerance(mediaItem.FileSize, result.Size, injOpts.SizeMismatchTolerancePercent) {
				sizeFiltered++
				l.Debug().
					Str("candidate", result.Title).
					Int64("sourceSize", mediaItem.FileSize).
					Int64("candidateSize", result.Size).
					Msg("arrseed: candidate filtered by size tolerance")
				continue
			}
			matched = append(matched, result)
		}
		if sizeFiltered > 0 {
			l.Debug().Int("sizeFiltered", sizeFiltered).Msg("arrseed: candidates filtered by size tolerance")
		}

		if len(matched) == 0 {
			l.Info().
				Str("release", dbItem.ReleaseName).
				Int("searchResults", len(results)).
				Msg("arrseed: no matching releases after filtering")
			_ = r.store.UpdateItemStatus(ctx, dbItem.ID, models.ArrSeedItemStatusNoMatch, "", "", "no matching releases")
			continue
		}

		l.Info().
			Str("release", dbItem.ReleaseName).
			Int("matched", len(matched)).
			Int("total", len(results)).
			Msg("arrseed: matched releases found, attempting injection")
		stats.matchesFound++

		injected := false
		for j, result := range matched {
			if ctx.Err() != nil {
				return
			}

			l.Info().
				Int("attempt", j+1).
				Int("totalResults", len(matched)).
				Str("title", result.Title).
				Str("indexer", result.Indexer).
				Int64("size", result.Size).
				Str("downloadURL", result.DownloadURL).
				Msg("arrseed: trying inject for matched result")

			injectResult, injectErr := r.injector.tryInject(ctx, mediaItem, &result, config, injOpts, &l)
			if injectErr != nil {
				l.Warn().Err(injectErr).
					Str("title", result.Title).
					Str("indexer", result.Indexer).
					Msg("arrseed: inject attempt failed")
				continue
			}

			if injectResult.Success {
				stats.torrentsAdded++
				progress.MatchesFound = stats.matchesFound
				progress.TorrentsAdded = stats.torrentsAdded
				r.progress.set(runID, progress)

				_ = r.store.UpdateItemStatus(ctx, dbItem.ID, models.ArrSeedItemStatusSeeded, injectResult.TorrentHash, result.Indexer, "")
				injected = true

				l.Info().
					Str("release", dbItem.ReleaseName).
					Str("indexer", result.Indexer).
					Str("hash", injectResult.TorrentHash).
					Bool("partialUpgrade", injectResult.IsPartialUpgrade).
					Msg("arrseed: torrent injected successfully")

				if injectResult.IsPartialUpgrade {
					l.Info().
						Str("hash", injectResult.TorrentHash).
						Int("unmatched", injectResult.UnmatchedCount).
						Msg("arrseed: partial upgrade injected, starting download monitor")
					go r.monitorPartialUpgrade(config.TargetQbitInstanceID, injectResult.TorrentHash, arrClient, mediaItem, config)
				} else {
					r.postSeedExecutor.execute(ctx, arrClient, mediaItem, config, &l)
				}
				break
			} else {
				l.Warn().
					Str("title", result.Title).
					Str("error", injectResult.ErrorMessage).
					Msg("arrseed: inject returned not successful")
			}
		}

		if !injected {
			l.Info().Str("release", dbItem.ReleaseName).Int("triedResults", len(matched)).Msg("arrseed: no injectable results for item")
			_ = r.store.UpdateItemStatus(ctx, dbItem.ID, models.ArrSeedItemStatusNoMatch, "", "", "no injectable results")
		}

		if settings.SearchDelaySeconds > 0 && i < len(pendingItems)-1 {
			l.Debug().Int("delaySeconds", settings.SearchDelaySeconds).Msg("arrseed: applying search delay")
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Duration(settings.SearchDelaySeconds) * time.Second):
			}
		}

		_ = r.store.UpdateRunStats(ctx, runID, stats.itemsScanned, stats.itemsSearched, stats.matchesFound, stats.torrentsAdded)
	}

	_ = r.store.UpdateConfigLastScan(context.Background(), configID)
	_ = r.store.UpdateRunCompleted(context.Background(), runID, stats.itemsScanned, stats.itemsSearched, stats.matchesFound, stats.torrentsAdded)

	l.Info().
		Int("scanned", stats.itemsScanned).
		Int("searched", stats.itemsSearched).
		Int("matches", stats.matchesFound).
		Int("added", stats.torrentsAdded).
		Msg("arrseed: run completed")
}
