// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package models

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/autobrr/qui/internal/dbinterface"
)

// ArrSeedItemStatus defines the status of an Arr Seed item.
type ArrSeedItemStatus string

const (
	ArrSeedItemStatusPending  ArrSeedItemStatus = "pending"  //nolint:goconst
	ArrSeedItemStatusSearched ArrSeedItemStatus = "searched"
	ArrSeedItemStatusMatched  ArrSeedItemStatus = "matched"
	ArrSeedItemStatusSeeded   ArrSeedItemStatus = "seeded"
	ArrSeedItemStatusNoMatch  ArrSeedItemStatus = "no_match" //nolint:goconst
	ArrSeedItemStatusError    ArrSeedItemStatus = "error"    //nolint:goconst
)

// ArrSeedRunStatus defines the status of an Arr Seed run.
type ArrSeedRunStatus string

const (
	ArrSeedRunStatusRunning   ArrSeedRunStatus = "running"
	ArrSeedRunStatusCompleted ArrSeedRunStatus = "completed"
	ArrSeedRunStatusFailed    ArrSeedRunStatus = "failed"    //nolint:goconst
	ArrSeedRunStatusCancelled ArrSeedRunStatus = "cancelled" //nolint:goconst
)

// ArrSeedSettings represents global Arr Seed settings.
// Shared settings (startPaused, sizeTolerance, tags) are read from CrossSeedAutomationSettings.
type ArrSeedSettings struct {
	ID                        int       `json:"id"`
	Enabled                   bool      `json:"enabled"`
	SearchDelaySeconds        int       `json:"searchDelaySeconds"`
	MaxItemsPerRun            int       `json:"maxItemsPerRun"`
	EnableSeasonPackHighScore bool      `json:"enableSeasonPackHighScore"`
	EnableSeasonPack          bool      `json:"enableSeasonPack"`
	EnableEpisode             bool      `json:"enableEpisode"`
	EnableSeasonPackUpgrade   bool      `json:"enableSeasonPackUpgrade"`
	CreatedAt                 time.Time `json:"createdAt"`
	UpdatedAt                 time.Time `json:"updatedAt"`
}

// ArrSeedInstanceConfig represents a per-ARR-instance cross-seed configuration.
type ArrSeedInstanceConfig struct {
	ID                    int        `json:"id"`
	ArrInstanceID         int        `json:"arrInstanceId"`
	ArrInstanceName       string     `json:"arrInstanceName,omitempty"`
	ArrInstanceType       string     `json:"arrInstanceType,omitempty"`
	Enabled               bool       `json:"enabled"`
	TargetQbitInstanceID  int        `json:"targetQbitInstanceId"`
	TargetQbitInstanceName string    `json:"targetQbitInstanceName,omitempty"`
	Category              string     `json:"category"`
	ArrDockerPath         string     `json:"arrDockerPath"`
	HostDataPath          string     `json:"hostDataPath"`
	TorrentSavePath       string     `json:"torrentSavePath"`
	ScanIntervalMinutes   int        `json:"scanIntervalMinutes"`
	UnmonitorAfterSeed    bool       `json:"unmonitorAfterSeed"`
	TagAfterSeed          string     `json:"tagAfterSeed"`
	LastScanAt            *time.Time `json:"lastScanAt,omitempty"`
	CreatedAt             time.Time  `json:"createdAt"`
	UpdatedAt             time.Time  `json:"updatedAt"`
}

// ArrSeedRun represents a scan run history entry.
type ArrSeedRun struct {
	ID            int64            `json:"id"`
	ConfigID      int              `json:"configId"`
	Status        ArrSeedRunStatus `json:"status"`
	TriggeredBy   string           `json:"triggeredBy"`
	ItemsScanned  int              `json:"itemsScanned"`
	ItemsSearched int              `json:"itemsSearched"`
	MatchesFound  int              `json:"matchesFound"`
	TorrentsAdded int              `json:"torrentsAdded"`
	ErrorMessage  string           `json:"errorMessage,omitempty"`
	StartedAt     time.Time        `json:"startedAt"`
	CompletedAt   *time.Time       `json:"completedAt,omitempty"`
}

// ArrSeedItem represents a per-media-item tracking entry.
type ArrSeedItem struct {
	ID             int64             `json:"id"`
	ConfigID       int               `json:"configId"`
	ItemType       string            `json:"itemType"`
	ArrFileID      int               `json:"arrFileId"`
	ReleaseName    string            `json:"releaseName"`
	FilePath       string            `json:"filePath"`
	FileSize       int64             `json:"fileSize"`
	Priority       int               `json:"priority"`
	Status         ArrSeedItemStatus `json:"status"`
	TorrentHash    string            `json:"torrentHash,omitempty"`
	IndexerName    string            `json:"indexerName,omitempty"`
	ErrorMessage   string            `json:"errorMessage,omitempty"`
	LastSearchedAt *time.Time        `json:"lastSearchedAt,omitempty"`
	CreatedAt      time.Time         `json:"createdAt"`
	UpdatedAt      time.Time         `json:"updatedAt"`
}

// ArrSeedStore handles database operations for Arr Seed.
type ArrSeedStore struct {
	db dbinterface.Querier
}

// NewArrSeedStore creates a new ArrSeedStore.
func NewArrSeedStore(db dbinterface.Querier) *ArrSeedStore {
	return &ArrSeedStore{db: db}
}

// --- Settings Operations ---

// GetSettings retrieves the global Arr Seed settings.
func (s *ArrSeedStore) GetSettings(ctx context.Context) (*ArrSeedSettings, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, enabled, search_delay_seconds, max_items_per_run,
		       enable_season_pack_high_score, enable_season_pack,
		       enable_episode, enable_season_pack_upgrade,
		       created_at, updated_at
		FROM arr_seed_settings WHERE id = 1
	`)

	var settings ArrSeedSettings

	err := row.Scan(
		&settings.ID,
		&settings.Enabled,
		&settings.SearchDelaySeconds,
		&settings.MaxItemsPerRun,
		&settings.EnableSeasonPackHighScore,
		&settings.EnableSeasonPack,
		&settings.EnableEpisode,
		&settings.EnableSeasonPackUpgrade,
		&settings.CreatedAt,
		&settings.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan settings: %w", err)
	}

	return &settings, nil
}

// UpdateSettings updates the global Arr Seed settings (upsert).
func (s *ArrSeedStore) UpdateSettings(ctx context.Context, settings *ArrSeedSettings) (*ArrSeedSettings, error) {
	if settings == nil {
		return nil, errors.New("settings is nil")
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO arr_seed_settings (
			id, enabled, search_delay_seconds, max_items_per_run,
			enable_season_pack_high_score, enable_season_pack, enable_episode,
			enable_season_pack_upgrade
		) VALUES (1, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			enabled = excluded.enabled,
			search_delay_seconds = excluded.search_delay_seconds,
			max_items_per_run = excluded.max_items_per_run,
			enable_season_pack_high_score = excluded.enable_season_pack_high_score,
			enable_season_pack = excluded.enable_season_pack,
			enable_episode = excluded.enable_episode,
			enable_season_pack_upgrade = excluded.enable_season_pack_upgrade,
			updated_at = CURRENT_TIMESTAMP
	`,
		boolToInt(settings.Enabled),
		settings.SearchDelaySeconds,
		settings.MaxItemsPerRun,
		boolToInt(settings.EnableSeasonPackHighScore),
		boolToInt(settings.EnableSeasonPack),
		boolToInt(settings.EnableEpisode),
		boolToInt(settings.EnableSeasonPackUpgrade),
	)
	if err != nil {
		return nil, fmt.Errorf("update settings: %w", err)
	}

	return s.GetSettings(ctx)
}

// --- Instance Config Operations ---

var ErrArrSeedConfigNotFound = errors.New("arr seed config not found")

// CreateConfig creates a new Arr Seed instance config.
func (s *ArrSeedStore) CreateConfig(ctx context.Context, cfg *ArrSeedInstanceConfig) (*ArrSeedInstanceConfig, error) {
	if cfg == nil {
		return nil, errors.New("config is nil")
	}

	res, err := s.db.ExecContext(ctx, `
		INSERT INTO arr_seed_instance_configs
			(arr_instance_id, enabled, target_qbit_instance_id, category,
			 arr_docker_path, host_data_path, torrent_save_path, scan_interval_minutes,
			 unmonitor_after_seed, tag_after_seed)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, cfg.ArrInstanceID, boolToInt(cfg.Enabled), cfg.TargetQbitInstanceID, cfg.Category,
		cfg.ArrDockerPath, cfg.HostDataPath, cfg.TorrentSavePath, cfg.ScanIntervalMinutes,
		boolToInt(cfg.UnmonitorAfterSeed), cfg.TagAfterSeed)
	if err != nil {
		return nil, fmt.Errorf("insert config: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("get last insert id: %w", err)
	}

	return s.GetConfig(ctx, int(id))
}

// GetConfig retrieves a config by ID, joining ARR and qBit instance names.
func (s *ArrSeedStore) GetConfig(ctx context.Context, id int) (*ArrSeedInstanceConfig, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT c.id, c.arr_instance_id, c.enabled, c.target_qbit_instance_id, c.category,
		       c.arr_docker_path, c.host_data_path, c.torrent_save_path,
		       c.scan_interval_minutes, c.unmonitor_after_seed, c.tag_after_seed,
		       c.last_scan_at, c.created_at, c.updated_at,
		       COALESCE(a.name, ''), COALESCE(a.type, ''),
		       COALESCE(i.name, '')
		FROM arr_seed_instance_configs c
		LEFT JOIN arr_instances_view a ON a.id = c.arr_instance_id
		LEFT JOIN instances_view i ON i.id = c.target_qbit_instance_id
		WHERE c.id = ?
	`, id)

	return s.scanConfig(row)
}

type configScanner interface {
	Scan(dest ...any) error
}

func (s *ArrSeedStore) scanConfig(scanner configScanner) (*ArrSeedInstanceConfig, error) {
	var cfg ArrSeedInstanceConfig
	var lastScanAt sql.NullTime

	err := scanner.Scan(
		&cfg.ID,
		&cfg.ArrInstanceID,
		&cfg.Enabled,
		&cfg.TargetQbitInstanceID,
		&cfg.Category,
		&cfg.ArrDockerPath,
		&cfg.HostDataPath,
		&cfg.TorrentSavePath,
		&cfg.ScanIntervalMinutes,
		&cfg.UnmonitorAfterSeed,
		&cfg.TagAfterSeed,
		&lastScanAt,
		&cfg.CreatedAt,
		&cfg.UpdatedAt,
		&cfg.ArrInstanceName,
		&cfg.ArrInstanceType,
		&cfg.TargetQbitInstanceName,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrArrSeedConfigNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan config columns: %w", err)
	}

	if lastScanAt.Valid {
		cfg.LastScanAt = &lastScanAt.Time
	}

	return &cfg, nil
}

// ListConfigs retrieves all Arr Seed instance configs.
func (s *ArrSeedStore) ListConfigs(ctx context.Context) ([]*ArrSeedInstanceConfig, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.arr_instance_id, c.enabled, c.target_qbit_instance_id, c.category,
		       c.arr_docker_path, c.host_data_path, c.torrent_save_path,
		       c.scan_interval_minutes, c.unmonitor_after_seed, c.tag_after_seed,
		       c.last_scan_at, c.created_at, c.updated_at,
		       COALESCE(a.name, ''), COALESCE(a.type, ''),
		       COALESCE(i.name, '')
		FROM arr_seed_instance_configs c
		LEFT JOIN arr_instances_view a ON a.id = c.arr_instance_id
		LEFT JOIN instances_view i ON i.id = c.target_qbit_instance_id
		ORDER BY c.id
	`)
	if err != nil {
		return nil, fmt.Errorf("query configs: %w", err)
	}
	defer rows.Close()

	var configs []*ArrSeedInstanceConfig
	for rows.Next() {
		cfg, err := s.scanConfig(rows)
		if err != nil {
			return nil, err
		}
		configs = append(configs, cfg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate configs: %w", err)
	}
	return configs, nil
}

// ListEnabledConfigs returns all enabled configs.
func (s *ArrSeedStore) ListEnabledConfigs(ctx context.Context) ([]*ArrSeedInstanceConfig, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.arr_instance_id, c.enabled, c.target_qbit_instance_id, c.category,
		       c.arr_docker_path, c.host_data_path, c.torrent_save_path,
		       c.scan_interval_minutes, c.unmonitor_after_seed, c.tag_after_seed,
		       c.last_scan_at, c.created_at, c.updated_at,
		       COALESCE(a.name, ''), COALESCE(a.type, ''),
		       COALESCE(i.name, '')
		FROM arr_seed_instance_configs c
		LEFT JOIN arr_instances_view a ON a.id = c.arr_instance_id
		LEFT JOIN instances_view i ON i.id = c.target_qbit_instance_id
		WHERE c.enabled = 1
		ORDER BY c.id
	`)
	if err != nil {
		return nil, fmt.Errorf("query enabled configs: %w", err)
	}
	defer rows.Close()

	var configs []*ArrSeedInstanceConfig
	for rows.Next() {
		cfg, err := s.scanConfig(rows)
		if err != nil {
			return nil, err
		}
		configs = append(configs, cfg)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate configs: %w", err)
	}
	return configs, nil
}

// ArrSeedConfigUpdateParams holds optional fields for updating a config.
type ArrSeedConfigUpdateParams struct {
	Enabled              *bool
	TargetQbitInstanceID *int
	Category             *string
	ArrDockerPath        *string
	HostDataPath         *string
	TorrentSavePath      *string
	ScanIntervalMinutes  *int
	UnmonitorAfterSeed   *bool
	TagAfterSeed         *string
}

// UpdateConfig updates an Arr Seed instance config.
func (s *ArrSeedStore) UpdateConfig(ctx context.Context, id int, params *ArrSeedConfigUpdateParams) (*ArrSeedInstanceConfig, error) {
	if params == nil {
		return s.GetConfig(ctx, id)
	}

	existing, err := s.GetConfig(ctx, id)
	if err != nil {
		return nil, err
	}

	if params.Enabled != nil {
		existing.Enabled = *params.Enabled
	}
	if params.TargetQbitInstanceID != nil {
		existing.TargetQbitInstanceID = *params.TargetQbitInstanceID
	}
	if params.Category != nil {
		existing.Category = *params.Category
	}
	if params.ArrDockerPath != nil {
		existing.ArrDockerPath = *params.ArrDockerPath
	}
	if params.HostDataPath != nil {
		existing.HostDataPath = *params.HostDataPath
	}
	if params.TorrentSavePath != nil {
		existing.TorrentSavePath = *params.TorrentSavePath
	}
	if params.ScanIntervalMinutes != nil {
		existing.ScanIntervalMinutes = *params.ScanIntervalMinutes
	}
	if params.UnmonitorAfterSeed != nil {
		existing.UnmonitorAfterSeed = *params.UnmonitorAfterSeed
	}
	if params.TagAfterSeed != nil {
		existing.TagAfterSeed = *params.TagAfterSeed
	}
	_, err = s.db.ExecContext(ctx, `
		UPDATE arr_seed_instance_configs
		SET enabled = ?, target_qbit_instance_id = ?, category = ?,
		    arr_docker_path = ?, host_data_path = ?, torrent_save_path = ?,
		    scan_interval_minutes = ?, unmonitor_after_seed = ?, tag_after_seed = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, boolToInt(existing.Enabled), existing.TargetQbitInstanceID, existing.Category,
		existing.ArrDockerPath, existing.HostDataPath, existing.TorrentSavePath,
		existing.ScanIntervalMinutes, boolToInt(existing.UnmonitorAfterSeed), existing.TagAfterSeed, id)
	if err != nil {
		return nil, fmt.Errorf("update config: %w", err)
	}

	return s.GetConfig(ctx, id)
}

// DeleteConfig deletes an Arr Seed instance config.
func (s *ArrSeedStore) DeleteConfig(ctx context.Context, id int) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM arr_seed_instance_configs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete config: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("get rows affected: %w", err)
	}
	if rows == 0 {
		return ErrArrSeedConfigNotFound
	}
	return nil
}

// UpdateConfigLastScan updates the last scan timestamp.
func (s *ArrSeedStore) UpdateConfigLastScan(ctx context.Context, id int) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE arr_seed_instance_configs SET last_scan_at = CURRENT_TIMESTAMP WHERE id = ?
	`, id)
	return err
}

// --- Run Operations ---

var ErrArrSeedRunAlreadyActive = errors.New("an active arr seed run already exists for this config")

// CreateRunIfNoActive atomically creates a run if none is active.
func (s *ArrSeedStore) CreateRunIfNoActive(ctx context.Context, configID int, triggeredBy string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO arr_seed_runs (config_id, status, triggered_by)
		SELECT ?, 'running', ?
		WHERE NOT EXISTS (
			SELECT 1 FROM arr_seed_runs
			WHERE config_id = ? AND status = 'running'
		)
	`, configID, triggeredBy, configID)
	if err != nil {
		return 0, fmt.Errorf("insert run: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("get rows affected: %w", err)
	}
	if rows == 0 {
		return 0, ErrArrSeedRunAlreadyActive
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("get last insert id: %w", err)
	}
	return id, nil
}

// GetActiveRun returns the active run for a config, if any.
func (s *ArrSeedStore) GetActiveRun(ctx context.Context, configID int) (*ArrSeedRun, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, config_id, status, triggered_by, items_scanned, items_searched,
		       matches_found, torrents_added, error_message, started_at, completed_at
		FROM arr_seed_runs
		WHERE config_id = ? AND status = 'running'
		ORDER BY started_at DESC LIMIT 1
	`, configID)

	run, err := scanArrSeedRun(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return run, err
}

func scanArrSeedRun(scanner configScanner) (*ArrSeedRun, error) {
	var run ArrSeedRun
	var errorMessage sql.NullString
	var completedAt sql.NullTime

	if err := scanner.Scan(
		&run.ID,
		&run.ConfigID,
		&run.Status,
		&run.TriggeredBy,
		&run.ItemsScanned,
		&run.ItemsSearched,
		&run.MatchesFound,
		&run.TorrentsAdded,
		&errorMessage,
		&run.StartedAt,
		&completedAt,
	); err != nil {
		return nil, fmt.Errorf("scan run columns: %w", err)
	}

	if errorMessage.Valid {
		run.ErrorMessage = errorMessage.String
	}
	if completedAt.Valid {
		run.CompletedAt = &completedAt.Time
	}
	return &run, nil
}

// ListRuns lists recent runs for a config.
func (s *ArrSeedStore) ListRuns(ctx context.Context, configID, limit int) ([]*ArrSeedRun, error) {
	if limit <= 0 {
		limit = 20
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, config_id, status, triggered_by, items_scanned, items_searched,
		       matches_found, torrents_added, error_message, started_at, completed_at
		FROM arr_seed_runs
		WHERE config_id = ?
		ORDER BY started_at DESC
		LIMIT ?
	`, configID, limit)
	if err != nil {
		return nil, fmt.Errorf("query runs: %w", err)
	}
	defer rows.Close()

	var runs []*ArrSeedRun
	for rows.Next() {
		run, err := scanArrSeedRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

// UpdateRunCompleted marks a run as completed.
func (s *ArrSeedStore) UpdateRunCompleted(ctx context.Context, runID int64, itemsScanned, itemsSearched, matchesFound, torrentsAdded int) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE arr_seed_runs
		SET status = 'completed', items_scanned = ?, items_searched = ?, matches_found = ?, torrents_added = ?,
		    completed_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, itemsScanned, itemsSearched, matchesFound, torrentsAdded, runID)
	return err
}

// UpdateRunFailed marks a run as failed.
func (s *ArrSeedStore) UpdateRunFailed(ctx context.Context, runID int64, errorMessage string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE arr_seed_runs
		SET status = 'failed', error_message = ?, completed_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, errorMessage, runID)
	return err
}

// UpdateRunCancelled marks a run as cancelled.
func (s *ArrSeedStore) UpdateRunCancelled(ctx context.Context, runID int64) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE arr_seed_runs
		SET status = 'cancelled', completed_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, runID)
	return err
}

// UpdateRunStats updates in-progress run stats.
func (s *ArrSeedStore) UpdateRunStats(ctx context.Context, runID int64, itemsScanned, itemsSearched, matchesFound, torrentsAdded int) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE arr_seed_runs
		SET items_scanned = ?, items_searched = ?, matches_found = ?, torrents_added = ?
		WHERE id = ?
	`, itemsScanned, itemsSearched, matchesFound, torrentsAdded, runID)
	return err
}

// MarkActiveRunsFailed marks any running runs as failed.
func (s *ArrSeedStore) MarkActiveRunsFailed(ctx context.Context, errorMessage string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		UPDATE arr_seed_runs
		SET status = 'failed', error_message = ?, completed_at = CURRENT_TIMESTAMP
		WHERE status = 'running'
	`, errorMessage)
	if err != nil {
		return 0, fmt.Errorf("mark active runs failed: %w", err)
	}
	return res.RowsAffected()
}

// --- Item Operations ---

// UpsertItem inserts or updates an Arr Seed item.
func (s *ArrSeedStore) UpsertItem(ctx context.Context, item *ArrSeedItem) error {
	if item == nil {
		return errors.New("item is nil")
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO arr_seed_items
			(config_id, item_type, arr_file_id, release_name, file_path, file_size, priority, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(config_id, item_type, arr_file_id) DO UPDATE SET
			release_name = excluded.release_name,
			file_path = excluded.file_path,
			file_size = excluded.file_size,
			priority = excluded.priority,
			status = arr_seed_items.status,
			updated_at = CURRENT_TIMESTAMP
	`, item.ConfigID, item.ItemType, item.ArrFileID, item.ReleaseName, item.FilePath, item.FileSize, item.Priority, item.Status)
	if err != nil {
		return fmt.Errorf("upsert item: %w", err)
	}
	return nil
}

// UpdateItemStatus updates the status and optional fields of an item.
func (s *ArrSeedStore) UpdateItemStatus(ctx context.Context, id int64, status ArrSeedItemStatus, torrentHash, indexerName, errorMessage string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE arr_seed_items
		SET status = ?, torrent_hash = ?, indexer_name = ?, error_message = ?,
		    last_searched_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`, status, optionalString(torrentHash), optionalString(indexerName), optionalString(errorMessage), id)
	return err
}

// ListItems lists items for a config with optional status filter.
func (s *ArrSeedStore) ListItems(ctx context.Context, configID int, status *ArrSeedItemStatus, limit, offset int) ([]*ArrSeedItem, error) {
	if limit <= 0 {
		limit = 100
	}

	var query string
	var args []any

	if status != nil {
		query = `
			SELECT id, config_id, item_type, arr_file_id, release_name, file_path, file_size,
			       priority, status, torrent_hash, indexer_name, error_message, last_searched_at,
			       created_at, updated_at
			FROM arr_seed_items
			WHERE config_id = ? AND status = ?
			ORDER BY id DESC
			LIMIT ? OFFSET ?
		`
		args = []any{configID, *status, limit, offset}
	} else {
		query = `
			SELECT id, config_id, item_type, arr_file_id, release_name, file_path, file_size,
			       priority, status, torrent_hash, indexer_name, error_message, last_searched_at,
			       created_at, updated_at
			FROM arr_seed_items
			WHERE config_id = ?
			ORDER BY id DESC
			LIMIT ? OFFSET ?
		`
		args = []any{configID, limit, offset}
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query items: %w", err)
	}
	defer rows.Close()

	var items []*ArrSeedItem
	for rows.Next() {
		item, err := scanArrSeedItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func scanArrSeedItem(scanner configScanner) (*ArrSeedItem, error) {
	var item ArrSeedItem
	var torrentHash sql.NullString
	var indexerName sql.NullString
	var errorMessage sql.NullString
	var lastSearchedAt sql.NullTime

	if err := scanner.Scan(
		&item.ID,
		&item.ConfigID,
		&item.ItemType,
		&item.ArrFileID,
		&item.ReleaseName,
		&item.FilePath,
		&item.FileSize,
		&item.Priority,
		&item.Status,
		&torrentHash,
		&indexerName,
		&errorMessage,
		&lastSearchedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan item columns: %w", err)
	}

	if torrentHash.Valid {
		item.TorrentHash = torrentHash.String
	}
	if indexerName.Valid {
		item.IndexerName = indexerName.String
	}
	if errorMessage.Valid {
		item.ErrorMessage = errorMessage.String
	}
	if lastSearchedAt.Valid {
		item.LastSearchedAt = &lastSearchedAt.Time
	}
	return &item, nil
}

// GetPendingItems returns items that haven't been seeded yet for a config.
func (s *ArrSeedStore) GetPendingItems(ctx context.Context, configID int) ([]*ArrSeedItem, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, config_id, item_type, arr_file_id, release_name, file_path, file_size,
		       priority, status, torrent_hash, indexer_name, error_message, last_searched_at,
		       created_at, updated_at
		FROM arr_seed_items
		WHERE config_id = ? AND status IN ('pending', 'no_match', 'error')
		ORDER BY priority ASC, last_searched_at ASC NULLS FIRST, id ASC
	`, configID)
	if err != nil {
		return nil, fmt.Errorf("query pending items: %w", err)
	}
	defer rows.Close()

	var items []*ArrSeedItem
	for rows.Next() {
		item, err := scanArrSeedItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// DeleteItemsForConfig deletes all items for a config (reset).
func (s *ArrSeedStore) DeleteItemsForConfig(ctx context.Context, configID int) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM arr_seed_items WHERE config_id = ?`, configID)
	return err
}
