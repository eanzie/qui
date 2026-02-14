-- Priority tier toggles on global settings
ALTER TABLE arr_seed_settings ADD COLUMN enable_season_pack_high_score BOOLEAN NOT NULL DEFAULT 1;
ALTER TABLE arr_seed_settings ADD COLUMN enable_season_pack BOOLEAN NOT NULL DEFAULT 1;
ALTER TABLE arr_seed_settings ADD COLUMN enable_episode BOOLEAN NOT NULL DEFAULT 1;

-- Post-seed action fields on per-instance configs
ALTER TABLE arr_seed_instance_configs ADD COLUMN unmonitor_after_seed BOOLEAN NOT NULL DEFAULT 0;
ALTER TABLE arr_seed_instance_configs ADD COLUMN tag_after_seed TEXT NOT NULL DEFAULT '';

-- Priority column on items for sorting
ALTER TABLE arr_seed_items ADD COLUMN priority INTEGER NOT NULL DEFAULT 3;
