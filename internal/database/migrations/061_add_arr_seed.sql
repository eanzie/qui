-- Global settings (singleton, id=1)
CREATE TABLE IF NOT EXISTS arr_seed_settings (
    id INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    enabled BOOLEAN NOT NULL DEFAULT 0,
    search_delay_seconds INTEGER NOT NULL DEFAULT 5,
    max_items_per_run INTEGER NOT NULL DEFAULT 0,
    size_tolerance_percent REAL NOT NULL DEFAULT 0.0,
    start_paused BOOLEAN NOT NULL DEFAULT 1,
    tags TEXT NOT NULL DEFAULT '[]',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Per-ARR-instance cross-seed configuration
CREATE TABLE IF NOT EXISTS arr_seed_instance_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    arr_instance_id INTEGER NOT NULL REFERENCES arr_instances(id) ON DELETE CASCADE,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    target_qbit_instance_id INTEGER NOT NULL,
    category TEXT NOT NULL DEFAULT '',
    arr_docker_path TEXT NOT NULL DEFAULT '',
    host_data_path TEXT NOT NULL DEFAULT '',
    torrent_save_path TEXT NOT NULL DEFAULT '',
    scan_interval_minutes INTEGER NOT NULL DEFAULT 1440,
    last_scan_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(arr_instance_id)
);

-- Scan run history
CREATE TABLE IF NOT EXISTS arr_seed_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_id INTEGER NOT NULL REFERENCES arr_seed_instance_configs(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT 'running',
    triggered_by TEXT NOT NULL DEFAULT 'manual',
    items_scanned INTEGER NOT NULL DEFAULT 0,
    items_searched INTEGER NOT NULL DEFAULT 0,
    matches_found INTEGER NOT NULL DEFAULT 0,
    torrents_added INTEGER NOT NULL DEFAULT 0,
    error_message TEXT,
    started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP
);

-- Per-media-item tracking (dedup + history)
CREATE TABLE IF NOT EXISTS arr_seed_items (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_id INTEGER NOT NULL REFERENCES arr_seed_instance_configs(id) ON DELETE CASCADE,
    item_type TEXT NOT NULL,
    arr_file_id INTEGER NOT NULL,
    release_name TEXT NOT NULL,
    file_path TEXT NOT NULL,
    file_size INTEGER NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending',
    torrent_hash TEXT,
    indexer_name TEXT,
    error_message TEXT,
    last_searched_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(config_id, item_type, arr_file_id)
);

CREATE INDEX IF NOT EXISTS idx_arr_seed_items_config_status
    ON arr_seed_items(config_id, status);
CREATE INDEX IF NOT EXISTS idx_arr_seed_runs_config
    ON arr_seed_runs(config_id);
