/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { api } from "@/lib/api"
import type {
  ArrSeedConfigCreate,
  ArrSeedConfigUpdate,
  ArrSeedInstanceConfig,
  ArrSeedProgress,
  ArrSeedSettings,
  ArrSeedSettingsUpdate,
} from "@/types/arrseed"

export function useArrSeedSettings(options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: ["arr-seed", "settings"],
    queryFn: () => api.getArrSeedSettings(),
    enabled: options?.enabled ?? true,
    staleTime: 30_000,
  })
}

export function useUpdateArrSeedSettings() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (data: ArrSeedSettingsUpdate) => api.updateArrSeedSettings(data),
    onSuccess: (settings: ArrSeedSettings) => {
      queryClient.setQueryData<ArrSeedSettings>(["arr-seed", "settings"], settings)
    },
  })
}

export function useArrSeedConfigs(options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: ["arr-seed", "configs"],
    queryFn: () => api.listArrSeedConfigs(),
    enabled: options?.enabled ?? true,
    staleTime: 30_000,
  })
}

export function useArrSeedConfig(id: number, options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: ["arr-seed", "configs", id],
    queryFn: () => api.getArrSeedConfig(id),
    enabled: (options?.enabled ?? true) && id > 0,
    staleTime: 30_000,
  })
}

export function useCreateArrSeedConfig() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (data: ArrSeedConfigCreate) => api.createArrSeedConfig(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["arr-seed", "configs"] })
    },
  })
}

export function useUpdateArrSeedConfig(id: number) {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (data: ArrSeedConfigUpdate) => api.updateArrSeedConfig(id, data),
    onSuccess: (config: ArrSeedInstanceConfig) => {
      queryClient.setQueryData<ArrSeedInstanceConfig>(["arr-seed", "configs", id], config)
      queryClient.invalidateQueries({ queryKey: ["arr-seed", "configs"] })
    },
  })
}

export function useDeleteArrSeedConfig() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (id: number) => api.deleteArrSeedConfig(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["arr-seed", "configs"] })
    },
  })
}

export function useTriggerArrSeedScan() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (configId: number) => api.triggerArrSeedScan(configId),
    onSuccess: (_data, configId) => {
      queryClient.invalidateQueries({ queryKey: ["arr-seed", "status", configId] })
      queryClient.invalidateQueries({ queryKey: ["arr-seed", "runs", configId] })
    },
  })
}

export function useStopArrSeedScan() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (configId: number) => api.stopArrSeedScan(configId),
    onSuccess: (_data, configId) => {
      queryClient.invalidateQueries({ queryKey: ["arr-seed", "status", configId] })
    },
  })
}

export function useCancelArrSeedScan() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (configId: number) => api.cancelArrSeedScan(configId),
    onSuccess: (_data, configId) => {
      queryClient.setQueryData(["arr-seed", "status", configId], null)
      queryClient.invalidateQueries({ queryKey: ["arr-seed", "runs", configId] })
    },
  })
}

export function useArrSeedScanStatus(configId: number, options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: ["arr-seed", "status", configId],
    queryFn: () => api.getArrSeedScanStatus(configId),
    enabled: (options?.enabled ?? true) && configId > 0,
    refetchInterval: (query) => {
      const data = query.state.data as ArrSeedProgress | null | undefined
      if (data && (data.status === "running" || data.status === "stopping")) {
        return 2000
      }
      return false
    },
  })
}

export function useArrSeedRuns(configId: number, options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: ["arr-seed", "runs", configId],
    queryFn: () => api.listArrSeedRuns(configId),
    enabled: (options?.enabled ?? true) && configId > 0,
    staleTime: 30_000,
  })
}

export function useArrSeedItems(
  configId: number,
  options?: { enabled?: boolean; status?: string }
) {
  return useQuery({
    queryKey: ["arr-seed", "items", configId, options?.status],
    queryFn: () => api.listArrSeedItems(configId, { status: options?.status }),
    enabled: (options?.enabled ?? true) && configId > 0,
    staleTime: 30_000,
  })
}

export function useResetArrSeedItems() {
  const queryClient = useQueryClient()

  return useMutation({
    mutationFn: (configId: number) => api.resetArrSeedItems(configId),
    onSuccess: (_data, configId) => {
      queryClient.invalidateQueries({ queryKey: ["arr-seed", "items", configId] })
    },
  })
}
