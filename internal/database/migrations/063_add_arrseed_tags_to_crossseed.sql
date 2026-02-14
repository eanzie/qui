-- Add ARR seed tags to cross-seed settings (following source-specific tag pattern)
ALTER TABLE cross_seed_settings ADD COLUMN arr_seed_tags TEXT NOT NULL DEFAULT '["cross-seed"]';
