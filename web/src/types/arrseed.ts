/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

export interface ArrSeedSettings {
  id: number
  enabled: boolean
  searchDelaySeconds: number
  maxItemsPerRun: number
  enableSeasonPackHighScore: boolean
  enableSeasonPack: boolean
  enableEpisode: boolean
  enableSeasonPackUpgrade: boolean
  createdAt: string
  updatedAt: string
}

export interface ArrSeedSettingsUpdate {
  enabled?: boolean
  searchDelaySeconds?: number
  maxItemsPerRun?: number
  enableSeasonPackHighScore?: boolean
  enableSeasonPack?: boolean
  enableEpisode?: boolean
  enableSeasonPackUpgrade?: boolean
}

export interface ArrSeedInstanceConfig {
  id: number
  arrInstanceId: number
  arrInstanceName?: string
  arrInstanceType?: string
  enabled: boolean
  targetQbitInstanceId: number
  targetQbitInstanceName?: string
  category: string
  arrDockerPath: string
  hostDataPath: string
  torrentSavePath: string
  scanIntervalMinutes: number
  unmonitorAfterSeed: boolean
  tagAfterSeed: string
  lastScanAt?: string
  createdAt: string
  updatedAt: string
}

export interface ArrSeedConfigCreate {
  arrInstanceId: number
  enabled: boolean
  targetQbitInstanceId: number
  category: string
  arrDockerPath: string
  hostDataPath: string
  torrentSavePath: string
  scanIntervalMinutes: number
  unmonitorAfterSeed?: boolean
  tagAfterSeed?: string
}

export interface ArrSeedConfigUpdate {
  enabled?: boolean
  targetQbitInstanceId?: number
  category?: string
  arrDockerPath?: string
  hostDataPath?: string
  torrentSavePath?: string
  scanIntervalMinutes?: number
  unmonitorAfterSeed?: boolean
  tagAfterSeed?: string
}

export type ArrSeedRunStatus = "running" | "completed" | "failed" | "cancelled"

export interface ArrSeedRun {
  id: number
  configId: number
  status: ArrSeedRunStatus
  triggeredBy: string
  itemsScanned: number
  itemsSearched: number
  matchesFound: number
  torrentsAdded: number
  errorMessage?: string
  startedAt: string
  completedAt?: string
}

export type ArrSeedItemStatus = "pending" | "searched" | "matched" | "seeded" | "no_match" | "error"

export interface ArrSeedItem {
  id: number
  configId: number
  itemType: string
  arrFileId: number
  releaseName: string
  filePath: string
  fileSize: number
  priority: number
  status: ArrSeedItemStatus
  torrentHash?: string
  indexerName?: string
  errorMessage?: string
  lastSearchedAt?: string
  createdAt: string
  updatedAt: string
}

export interface ArrSeedProgress {
  runId: number
  configId: number
  status: string
  phase: string
  itemsTotal: number
  itemsProcessed: number
  matchesFound: number
  torrentsAdded: number
  currentItem?: string
}
