---
sidebar_position: 5
title: Arr Scan
description: Scan Sonarr and Radarr libraries to find cross-seed opportunities.
---

# Arr Scan

Arr Scan queries your Sonarr and Radarr instances to build a list of media items, then searches your configured indexers for cross-seed matches. Unlike Library Scan (which works from qBittorrent's torrent list) or Dir Scan (which reads files on disk), Arr Scan uses the *arr API to discover content and resolve release names.

Configure it in **Cross-Seed > Arr Scan**.

## Requirements

- At least one Sonarr or Radarr instance configured in **Settings > Integrations**.
- At least one qBittorrent instance configured as the injection target.
- Prowlarr or Jackett configured with at least one enabled indexer.

## How It Works

1. **Scan**: Queries the Sonarr/Radarr API for all series/movies and their episode/movie files.
2. **Resolve release names**: Uses the `sceneName` field from each file. If `sceneName` is empty, falls back to the series/movie history API to find the original source title.
3. **Classify**: Each item is assigned a priority based on its type and custom format score relative to the quality profile cutoff.
4. **Filter**: Items are filtered based on your settings (content type toggles, quality gate).
5. **Search**: Queries indexers for each item using release name and external IDs (IMDb, TMDb, TVDb).
6. **Inject**: Matched torrents are added to the target qBittorrent instance.

### Season Pack Detection

Arr Scan groups episode files by release name. If all episodes in a release share the same name and that name is a season pack (e.g., `Show.S01.1080p.BluRay-GROUP`), they are treated as a single season pack item.

Additionally, if all episodes in a season share the same release group but were grabbed individually, Arr Scan synthesizes a virtual season pack name (e.g., converting `Show.S01E05.1080p.WEB-DL-NTb` to `Show.S01.1080p.WEB-DL-NTb`) and searches for the full season pack. This can find cross-seed opportunities that individual episode searches would miss.

## Instance Configuration

Each Arr Scan configuration links one Sonarr/Radarr instance to one qBittorrent target. You can create multiple configurations for different instances.

Click **Add Config** to create a new configuration:

| Setting | Description |
|---------|-------------|
| Arr Instance | The Sonarr or Radarr instance to scan. |
| Target qBittorrent Instance | Where matched torrents are injected. |
| Category | Category applied to injected torrents (e.g., `TV.cross`). |
| Arr Docker Path | The path prefix as seen inside the Arr container (e.g., `/mnt/media/tv`). Used for path mapping and to filter content by directory. |
| Host Data Path | The equivalent path on the host/qui system (e.g., `/data/media/tv`). |
| Torrent Save Path | Where torrent data should be saved on the target instance. |
| Scan Interval | How often to run automatic scans (minimum 60 minutes, default 1440 = 24 hours). |
| Unmonitor After Seed | Unmonitor the series/movie in Sonarr/Radarr after a successful cross-seed. |
| Tag After Seed | Add this tag to the series/movie in Sonarr/Radarr after a successful cross-seed. |

### Path Mapping

When Sonarr/Radarr and qui see different mount points (common in Docker setups), configure **Arr Docker Path** and **Host Data Path** to map between them.

**Example:**
- Sonarr sees files at `/mnt/media/tv/Show/S01E01.mkv`
- qui sees the same file at `/data/media/tv/Show/S01E01.mkv`

Set:
- **Arr Docker Path**: `/mnt/media/tv`
- **Host Data Path**: `/data/media/tv`

If the Arr Docker Path is set, only content under that path is scanned. Content stored outside the configured path is skipped. This is useful when an Arr instance manages content across multiple root folders but you only want to cross-seed from one.

## Settings

Open **Arr Scan > Settings** to configure global behavior:

### Search Settings

| Setting | Description | Default |
|---------|-------------|---------|
| Search Delay (seconds) | Delay between indexer searches to avoid rate limiting. | 5 |
| Max Items Per Run | Limit how many items are processed per scan. 0 = unlimited. | 0 |

### Quality Gate

| Setting | Description | Default |
|---------|-------------|---------|
| High Score Only | Only process items with a custom format score at or above the quality profile's cutoff score. | Off |

When **High Score Only** is enabled, qui fetches quality profiles from both Sonarr and Radarr and compares each item's custom format score against its profile's cutoff. Items below the cutoff are skipped.

This is useful for focusing cross-seed efforts on your highest-quality releases.

:::note
For Radarr, custom format scores are fetched from the dedicated `/api/v3/moviefile` endpoint rather than the movie list endpoint, because Radarr only calculates format scores when files are queried individually.
:::

### Content Types

| Setting | Description | Default |
|---------|-------------|---------|
| Individual Episodes | Process individual episode files from seasons with mixed release groups. | Off |
| Season Pack Upgrade | Inject partial season packs so qBittorrent downloads missing episodes as upgrades. | Off |

**Content type behavior:**
- **Movies** are always processed regardless of settings.
- **Season packs** are always processed regardless of settings.
- **Individual episodes** are only processed when **Individual Episodes** is enabled.

When **Season Pack Upgrade** is enabled, **Individual Episodes** is automatically disabled (they are mutually exclusive).

### Priority Classification

Each item receives a priority that determines processing order:

| Priority | Level | Criteria |
|----------|-------|----------|
| 1 | High Score | Any item type with custom format score at or above the quality profile cutoff |
| 2 | Normal | Movies and season packs without high score |
| 3 | Episode | Individual episodes without high score |

Higher-priority items (lower number) are processed first during each scan run.

## Operational Behavior

### Incremental scanning

Arr Scan tracks which items have been processed. On subsequent runs, only new or pending items are searched. Use the **Reset** button on a configuration to clear all tracked items and force a full re-scan.

### Concurrent scans

Only one scan runs per configuration at a time. Triggering a scan while one is already running has no effect.

### Scan controls

- **Play**: Start a scan.
- **Stop**: Finish the current item, then stop (graceful).
- **Kill** (appears after Stop): Cancel immediately without waiting.

### Item states

Each tracked item progresses through states:

| State | Meaning |
|-------|---------|
| `pending` | Waiting to be searched |
| `seeded` | Successfully cross-seeded |
| `no_match` | Searched but no match found |
| `error` | Search or injection failed |

## Troubleshooting

### No items found during scan

- Verify the Arr instance is reachable and has content.
- If **Arr Docker Path** is set, ensure it matches the root folder path in Sonarr/Radarr. Content outside this path is skipped.
- Check that episode files have a `sceneName` or history entries with source titles. Items without a resolvable release name are skipped.

### All items filtered (0 after filter)

- If **High Score Only** is enabled, verify that your quality profiles have a non-zero cutoff format score and that your media has custom format scores above that cutoff.
- Check that the content types you want to process are enabled (movies and season packs are always on; episodes require **Individual Episodes** to be enabled).

### Custom format scores all showing 0

- For Radarr: This is expected from the movie list API. Arr Scan fetches scores separately from the moviefile endpoint. If scores are still 0, check that custom formats are configured and assigned in your Radarr quality profiles.
- For Sonarr: Scores come directly from the episode file endpoint.

### Path mapping issues

- Ensure **Arr Docker Path** matches exactly how paths appear in the Arr API (check Sonarr/Radarr's root folder settings).
- Ensure **Host Data Path** is where qui can access the same files.
- Path mapping is a simple string replacement: the Arr Docker Path prefix in each file path is replaced with the Host Data Path.
