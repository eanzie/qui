/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useCallback, useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import {
  AlertTriangle,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Clock,
  Info,
  Loader2,
  Pencil,
  Play,
  Plus,
  Radar,
  RotateCcw,
  Settings2,
  Square,
  Trash2,
  XCircle,
} from "lucide-react"

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { FieldHelp } from "@/components/ui/field-help"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { useDateTimeFormatters } from "@/hooks/useDateTimeFormatters"
import {
  useArrSeedConfigs,
  useArrSeedItems,
  useArrSeedRuns,
  useArrSeedScanStatus,
  useArrSeedSettings,
  useCancelArrSeedScan,
  useCreateArrSeedConfig,
  useDeleteArrSeedConfig,
  useResetArrSeedItems,
  useStopArrSeedScan,
  useTriggerArrSeedScan,
  useUpdateArrSeedConfig,
  useUpdateArrSeedSettings,
} from "@/hooks/useArrSeed"
import { api } from "@/lib/api"
import type { ArrInstance } from "@/types/arr"
import type { ArrSeedConfigCreate, ArrSeedInstanceConfig, ArrSeedProgress } from "@/types/arrseed"
import type { Instance } from "@/types"
import { useQuery, useQueryClient } from "@tanstack/react-query"

interface ArrSeedTabProps {
  instances: Instance[]
}

export function ArrSeedTab({ instances }: ArrSeedTabProps) {
  const { t } = useTranslation("crossseed")
  const { formatDate } = useDateTimeFormatters()
  const { data: settings, isLoading: settingsLoading } = useArrSeedSettings()
  const { data: configs, isLoading: configsLoading } = useArrSeedConfigs()
  const updateSettings = useUpdateArrSeedSettings()

  const { data: arrInstances } = useQuery({
    queryKey: ["arr", "instances"],
    queryFn: () => api.listArrInstances(),
    staleTime: 30_000,
  })

  const [showSettingsDialog, setShowSettingsDialog] = useState(false)
  const [showAddDialog, setShowAddDialog] = useState(false)
  const [expandedConfig, setExpandedConfig] = useState<number | null>(null)

  const handleToggleEnabled = useCallback(
    (enabled: boolean) => {
      updateSettings.mutate({ enabled }, {
        onSuccess: () => toast.success(enabled ? t("arrScan.toast.enabled") : t("arrScan.toast.disabled")),
        onError: () => toast.error(t("arrScan.toast.settingsUpdateFailed")),
      })
    },
    [updateSettings, t]
  )

  if (settingsLoading || configsLoading) {
    return (
      <div className="flex items-center justify-center py-12">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="flex items-center gap-2">
                <Radar className="size-5" />
                {t("arrScan.title")}
              </CardTitle>
              <CardDescription>
                {t("arrScan.description")}
              </CardDescription>
            </div>
            <div className="flex items-center gap-4">
              <Button variant="outline" size="sm" onClick={() => setShowSettingsDialog(true)}>
                <Settings2 className="size-4 mr-2" />
                {t("arrScan.settings")}
              </Button>
              <Label htmlFor="arrseed-enabled" className="flex items-center gap-2">
                <Switch
                  id="arrseed-enabled"
                  checked={settings?.enabled ?? false}
                  onCheckedChange={handleToggleEnabled}
                  disabled={updateSettings.isPending}
                />
                {settings?.enabled ? t("arrScan.enabled") : t("arrScan.disabled")}
              </Label>
            </div>
          </div>
        </CardHeader>
      </Card>

      {/* Instance Configs */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="text-base">{t("arrScan.instanceConfigs")}</CardTitle>
              <CardDescription>{t("arrScan.instanceConfigsDescription")}</CardDescription>
            </div>
            <Button size="sm" onClick={() => setShowAddDialog(true)}>
              <Plus className="h-4 w-4 mr-1" />
              {t("arrScan.addConfig")}
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {configs && configs.length > 0 ? (
            <div className="space-y-3">
              {configs.map((config) => (
                <ConfigCard
                  key={config.id}
                  config={config}
                  expanded={expandedConfig === config.id}
                  onToggleExpand={() =>
                    setExpandedConfig(expandedConfig === config.id ? null : config.id)
                  }
                  formatDate={formatDate}
                  instances={instances}
                />
              ))}
            </div>
          ) : (
            <div className="text-center py-8 text-muted-foreground">
              <Info className="h-8 w-8 mx-auto mb-2 opacity-50" />
              <p>{t("arrScan.noConfigs")}</p>
            </div>
          )}
        </CardContent>
      </Card>

      {/* Add Config Dialog */}
      <AddConfigDialog
        open={showAddDialog}
        onOpenChange={setShowAddDialog}
        arrInstances={arrInstances ?? []}
        instances={instances}
      />

      {/* Settings Dialog */}
      <SettingsDialog
        open={showSettingsDialog}
        onOpenChange={setShowSettingsDialog}
        settings={settings}
      />
    </div>
  )
}

// --- Settings Dialog ---

function SettingsDialog({
  open,
  onOpenChange,
  settings,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  settings: ReturnType<typeof useArrSeedSettings>["data"]
}) {
  const { t } = useTranslation("crossseed")
  const updateSettings = useUpdateArrSeedSettings()
  const [searchDelay, setSearchDelay] = useState(String(settings?.searchDelaySeconds ?? 5))
  const [maxItems, setMaxItems] = useState(String(settings?.maxItemsPerRun ?? 0))

  const handleSave = () => {
    updateSettings.mutate(
      {
        searchDelaySeconds: parseInt(searchDelay) || 5,
        maxItemsPerRun: parseInt(maxItems) || 0,
      },
      {
        onSuccess: () => {
          toast.success(t("arrScan.toast.settingsSaved"))
          onOpenChange(false)
        },
        onError: () => toast.error(t("arrScan.toast.settingsSaveFailed")),
      }
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("arrScan.settingsDialog.title")}</DialogTitle>
          <DialogDescription>
            {t("arrScan.settingsDialog.description")}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label>{t("arrScan.settingsDialog.searchDelayLabel")}</Label>
              <Input
                type="number"
                value={searchDelay}
                onChange={(e) => setSearchDelay(e.target.value)}
                min={0}
              />
              <p className="text-xs text-muted-foreground">{t("arrScan.settingsDialog.searchDelayHelp")}</p>
            </div>
            <div className="space-y-2">
              <Label>{t("arrScan.settingsDialog.maxItemsLabel")}</Label>
              <Input
                type="number"
                value={maxItems}
                onChange={(e) => setMaxItems(e.target.value)}
                min={0}
              />
              <p className="text-xs text-muted-foreground">{t("arrScan.settingsDialog.maxItemsHelp")}</p>
            </div>
          </div>
          <p className="text-xs text-muted-foreground">
            {t("arrScan.settingsDialog.sharedSettingsNote")}
          </p>
          <div className="space-y-2">
            <Label className="text-sm font-medium">{t("arrScan.settingsDialog.qualityGateLabel")}</Label>
            <div className="flex items-center gap-2">
              <Switch
                checked={settings?.enableHighScoreOnly ?? false}
                onCheckedChange={(checked) =>
                  updateSettings.mutate({ enableHighScoreOnly: checked })
                }
              />
              <Label className="text-sm flex items-center gap-1">
                {t("arrScan.settingsDialog.highScoreOnlyLabel")}
                <FieldHelp>{t("arrScan.settingsDialog.highScoreOnlyHelp")}</FieldHelp>
              </Label>
            </div>
          </div>
          <div className="space-y-2">
            <Label className="text-sm font-medium">{t("arrScan.settingsDialog.contentTypesLabel")}</Label>
            <div className="space-y-2">
              <div className="flex items-center gap-2">
                <Switch
                  checked={settings?.enableEpisode ?? false}
                  disabled={settings?.enableSeasonPackUpgrade ?? false}
                  onCheckedChange={(checked) =>
                    updateSettings.mutate({ enableEpisode: checked })
                  }
                />
                <Label className="text-sm flex items-center gap-1">
                  {t("arrScan.settingsDialog.individualEpisodesLabel")}
                  <FieldHelp>
                    {settings?.enableSeasonPackUpgrade
                      ? t("arrScan.settingsDialog.individualEpisodesDisabledHelp")
                      : t("arrScan.settingsDialog.individualEpisodesHelp")}
                  </FieldHelp>
                </Label>
              </div>
            </div>
          </div>
          <div className="space-y-2">
            <Label className="text-sm font-medium">{t("arrScan.settingsDialog.seasonPackUpgradeLabel")}</Label>
            <div className="flex items-center gap-2">
              <Switch
                checked={settings?.enableSeasonPackUpgrade ?? false}
                onCheckedChange={(checked) =>
                  updateSettings.mutate({ enableSeasonPackUpgrade: checked })
                }
              />
              <Label className="text-sm flex items-center gap-1">
                {t("arrScan.settingsDialog.enableSeasonPackUpgradeLabel")}
                <FieldHelp>{t("arrScan.settingsDialog.seasonPackUpgradeHelp")}</FieldHelp>
              </Label>
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("arrScan.settingsDialog.cancel")}
          </Button>
          <Button onClick={handleSave} disabled={updateSettings.isPending}>
            {t("arrScan.settingsDialog.save")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// --- Config Card ---

function ConfigCard({
  config,
  expanded,
  onToggleExpand,
  formatDate,
  instances,
}: {
  config: ArrSeedInstanceConfig
  expanded: boolean
  onToggleExpand: () => void
  formatDate: (d: Date) => string
  instances: Instance[]
}) {
  const { t } = useTranslation("crossseed")
  const queryClient = useQueryClient()
  const triggerScan = useTriggerArrSeedScan()
  const stopScan = useStopArrSeedScan()
  const cancelScan = useCancelArrSeedScan()
  const deleteConfig = useDeleteArrSeedConfig()
  const updateConfig = useUpdateArrSeedConfig(config.id)
  const { data: scanStatus } = useArrSeedScanStatus(config.id)
  const [showDelete, setShowDelete] = useState(false)
  const [showEdit, setShowEdit] = useState(false)

  const isRunning = scanStatus?.status === "running"
  const isStopping = scanStatus?.status === "stopping"

  // Detect scan completion and invalidate related queries
  const prevStatusRef = useRef<string | undefined>(undefined)
  useEffect(() => {
    const currentStatus = scanStatus?.status
    const prevStatus = prevStatusRef.current
    prevStatusRef.current = currentStatus

    const wasActive = prevStatus === "running" || prevStatus === "stopping"
    const isNowDone = currentStatus && currentStatus !== "running" && currentStatus !== "stopping"
    if (wasActive && isNowDone) {
      queryClient.invalidateQueries({ queryKey: ["arr-seed", "runs", config.id] })
      queryClient.invalidateQueries({ queryKey: ["arr-seed", "configs"] })
      queryClient.invalidateQueries({ queryKey: ["arr-seed", "items", config.id] })

      if (currentStatus === "completed") {
        toast.success(t("arrScan.toast.scanCompleted", { count: scanStatus!.torrentsAdded }))
      } else if (currentStatus === "failed") {
        toast.error(t("arrScan.toast.scanFailed"))
      }
    }
  }, [scanStatus?.status, scanStatus?.torrentsAdded, config.id, queryClient, t])

  const handleTriggerScan = () => {
    triggerScan.mutate(config.id, {
      onSuccess: () => toast.success(t("arrScan.toast.scanStarted")),
      onError: (err) => toast.error(t("arrScan.toast.scanStartFailed", { error: err.message })),
    })
  }

  const handleStopScan = () => {
    stopScan.mutate(config.id, {
      onSuccess: () => toast.success(t("arrScan.toast.scanStopping")),
      onError: () => toast.error(t("arrScan.toast.scanStopFailed")),
    })
  }

  const handleKillScan = () => {
    cancelScan.mutate(config.id, {
      onSuccess: () => toast.success(t("arrScan.toast.scanKilled")),
      onError: () => toast.error(t("arrScan.toast.scanKillFailed")),
    })
  }

  const handleDelete = () => {
    deleteConfig.mutate(config.id, {
      onSuccess: () => {
        toast.success(t("arrScan.toast.configDeleted"))
        setShowDelete(false)
      },
      onError: () => toast.error(t("arrScan.toast.configDeleteFailed")),
    })
  }

  const handleToggleEnabled = (enabled: boolean) => {
    updateConfig.mutate({ enabled })
  }

  return (
    <>
      <div className="border rounded-lg">
        <div className="p-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3 cursor-pointer" onClick={onToggleExpand}>
              {expanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
              <div>
                <div className="flex items-center gap-2">
                  <span className="font-medium">{config.arrInstanceName || t("arrScan.config.arrInstanceFallback", { id: config.arrInstanceId })}</span>
                  <Badge variant="outline" className="text-xs">
                    {t(`arrScan.instanceTypeLabels.${config.arrInstanceType || "unknown"}`, config.arrInstanceType || "unknown")}
                  </Badge>
                  {!config.enabled && (
                    <Badge variant="secondary" className="text-xs">{t("arrScan.config.disabledBadge")}</Badge>
                  )}
                </div>
                <div className="text-sm text-muted-foreground">
                  {t("arrScan.config.target", { name: config.targetQbitInstanceName || t("arrScan.config.qbitInstanceFallback", { id: config.targetQbitInstanceId }) })}
                  {config.category && t("arrScan.config.categorySuffix", { category: config.category })}
                </div>
              </div>
            </div>
            <div className="flex items-center gap-2">
              {scanStatus?.status && <ScanProgressBadge progress={scanStatus} />}
              <Switch
                checked={config.enabled}
                onCheckedChange={handleToggleEnabled}
              />
              {isStopping ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button size="icon" variant="ghost" onClick={handleKillScan} disabled={cancelScan.isPending}>
                      <XCircle className="h-4 w-4 text-destructive" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{t("arrScan.config.killTooltip")}</TooltipContent>
                </Tooltip>
              ) : isRunning ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button size="icon" variant="ghost" onClick={handleStopScan} disabled={stopScan.isPending}>
                      <Square className="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{t("arrScan.config.stopTooltip")}</TooltipContent>
                </Tooltip>
              ) : (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      size="icon"
                      variant="ghost"
                      onClick={handleTriggerScan}
                      disabled={triggerScan.isPending}
                    >
                      <Play className="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{t("arrScan.config.startTooltip")}</TooltipContent>
                </Tooltip>
              )}
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button size="icon" variant="ghost" onClick={() => setShowEdit(true)}>
                    <Pencil className="h-4 w-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>{t("arrScan.config.editTooltip")}</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button size="icon" variant="ghost" onClick={() => setShowDelete(true)}>
                    <Trash2 className="h-4 w-4 text-destructive" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>{t("arrScan.config.deleteTooltip")}</TooltipContent>
              </Tooltip>
            </div>
          </div>

          {config.lastScanAt && (
            <div className="mt-1 text-xs text-muted-foreground flex items-center gap-1 ml-7">
              <Clock className="h-3 w-3" />
              {t("arrScan.config.lastScan", { time: formatDate(new Date(config.lastScanAt)) })}
            </div>
          )}
        </div>

        {expanded && (
          <div className="border-t px-4 py-3 space-y-4">
            <ConfigDetails config={config} />
            <ConfigHistory configId={config.id} formatDate={formatDate} />
          </div>
        )}
      </div>

      <AlertDialog open={showDelete} onOpenChange={setShowDelete}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("arrScan.deleteDialog.title")}</AlertDialogTitle>
            <AlertDialogDescription>
              {t("arrScan.deleteDialog.description", { name: config.arrInstanceName ?? "" })}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("arrScan.deleteDialog.cancel")}</AlertDialogCancel>
            <AlertDialogAction onClick={handleDelete} className="bg-destructive text-destructive-foreground">
              {t("arrScan.deleteDialog.delete")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      <EditConfigDialog
        open={showEdit}
        onOpenChange={setShowEdit}
        config={config}
        instances={instances}
      />
    </>
  )
}

function ScanProgressBadge({ progress }: { progress: ArrSeedProgress }) {
  const { t } = useTranslation("crossseed")

  if (progress.status === "completed") {
    return (
      <Badge variant="outline" className="text-xs gap-1 text-green-500">
        <CheckCircle2 className="h-3 w-3" />
        {t("arrScan.progress.completed", { processed: progress.itemsProcessed, added: progress.torrentsAdded })}
      </Badge>
    )
  }
  if (progress.status === "failed") {
    return (
      <Badge variant="outline" className="text-xs gap-1 text-destructive">
        <XCircle className="h-3 w-3" />
        {t("arrScan.runStatusLabels.failed")}
      </Badge>
    )
  }
  if (progress.status === "cancelled") {
    return (
      <Badge variant="outline" className="text-xs gap-1 text-yellow-500">
        <AlertTriangle className="h-3 w-3" />
        {t("arrScan.runStatusLabels.cancelled")}
      </Badge>
    )
  }
  if (progress.status === "stopping") {
    return (
      <Badge variant="outline" className="text-xs gap-1 text-yellow-500">
        <Loader2 className="h-3 w-3 animate-spin" />
        {t("arrScan.progress.stopping", { processed: progress.itemsProcessed, total: progress.itemsTotal })}
      </Badge>
    )
  }
  return (
    <Badge variant="outline" className="text-xs gap-1">
      <Loader2 className="h-3 w-3 animate-spin" />
      {t("arrScan.progress.running", {
        phase: t(`arrScan.phaseLabels.${progress.phase}`, progress.phase),
        processed: progress.itemsProcessed,
        total: progress.itemsTotal,
      })}
      {progress.torrentsAdded > 0 && t("arrScan.progress.addedSuffix", { added: progress.torrentsAdded })}
    </Badge>
  )
}

function ConfigDetails({ config }: { config: ArrSeedInstanceConfig }) {
  const { t } = useTranslation("crossseed")

  return (
    <div className="grid grid-cols-1 md:grid-cols-2 gap-3 text-sm">
      <div>
        <span className="text-muted-foreground">{t("arrScan.details.arrDockerPath")}</span>{" "}
        <code className="text-xs bg-muted px-1 rounded">{config.arrDockerPath || t("arrScan.details.notSet")}</code>
      </div>
      <div>
        <span className="text-muted-foreground">{t("arrScan.details.hostDataPath")}</span>{" "}
        <code className="text-xs bg-muted px-1 rounded">{config.hostDataPath || t("arrScan.details.notSet")}</code>
      </div>
      <div>
        <span className="text-muted-foreground">{t("arrScan.details.torrentSavePath")}</span>{" "}
        <code className="text-xs bg-muted px-1 rounded">{config.torrentSavePath || t("arrScan.details.notSet")}</code>
      </div>
      <div>
        <span className="text-muted-foreground">{t("arrScan.details.scanInterval")}</span>{" "}
        {t("arrScan.details.scanIntervalValue", { minutes: config.scanIntervalMinutes })}
      </div>
      <div>
        <span className="text-muted-foreground">{t("arrScan.details.unmonitorAfterSeed")}</span>{" "}
        {config.unmonitorAfterSeed ? t("arrScan.details.yes") : t("arrScan.details.no")}
      </div>
      <div>
        <span className="text-muted-foreground">{t("arrScan.details.tagAfterSeed")}</span>{" "}
        <code className="text-xs bg-muted px-1 rounded">{config.tagAfterSeed || t("arrScan.details.none")}</code>
      </div>
    </div>
  )
}

function ConfigHistory({ configId, formatDate }: { configId: number; formatDate: (d: Date) => string }) {
  const { t } = useTranslation("crossseed")
  const { data: runs } = useArrSeedRuns(configId)
  const { data: items } = useArrSeedItems(configId)
  const resetItems = useResetArrSeedItems()

  const handleReset = () => {
    resetItems.mutate(configId, {
      onSuccess: () => toast.success(t("arrScan.toast.itemsReset")),
      onError: () => toast.error(t("arrScan.toast.itemsResetFailed")),
    })
  }

  const statusCounts = items?.reduce(
    (acc, item) => {
      acc[item.status] = (acc[item.status] || 0) + 1
      return acc
    },
    {} as Record<string, number>
  )

  return (
    <div className="space-y-3">
      {/* Item status summary */}
      {statusCounts && Object.keys(statusCounts).length > 0 && (
        <div className="flex items-center gap-2 flex-wrap">
          <span className="text-sm text-muted-foreground">{t("arrScan.history.items")}</span>
          {statusCounts.seeded && (
            <Badge variant="default" className="gap-1">
              <CheckCircle2 className="h-3 w-3" />
              {t("arrScan.history.itemStatusCount", { count: statusCounts.seeded, status: t("arrScan.itemStatusLabels.seeded") })}
            </Badge>
          )}
          {statusCounts.pending && (
            <Badge variant="secondary">{t("arrScan.history.itemStatusCount", { count: statusCounts.pending, status: t("arrScan.itemStatusLabels.pending") })}</Badge>
          )}
          {statusCounts.no_match && (
            <Badge variant="outline">{t("arrScan.history.itemStatusCount", { count: statusCounts.no_match, status: t("arrScan.itemStatusLabels.no_match") })}</Badge>
          )}
          {statusCounts.error && (
            <Badge variant="destructive" className="gap-1">
              <XCircle className="h-3 w-3" />
              {t("arrScan.history.itemStatusCount", { count: statusCounts.error, status: t("arrScan.itemStatusLabels.error") })}
            </Badge>
          )}
          <Button size="sm" variant="ghost" onClick={handleReset} className="h-6 text-xs">
            <RotateCcw className="h-3 w-3 mr-1" />
            {t("arrScan.history.reset")}
          </Button>
        </div>
      )}

      {/* Recent runs */}
      {runs && runs.length > 0 && (
        <div>
          <p className="text-sm font-medium mb-1">{t("arrScan.history.recentRuns")}</p>
          <div className="space-y-1">
            {runs.slice(0, 5).map(({ id, status, startedAt, itemsScanned, matchesFound, torrentsAdded, triggeredBy }) => (
              <div key={id} className="flex items-center gap-2 text-xs">
                <RunStatusIcon status={status} />
                <span className="text-muted-foreground">{formatDate(new Date(startedAt))}</span>
                <span>
                  {t("arrScan.history.runSummary", { scanned: itemsScanned, matched: matchesFound, added: torrentsAdded })}
                </span>
                <Badge variant="outline" className="text-xs">{t(`arrScan.triggeredByLabels.${triggeredBy}`, triggeredBy)}</Badge>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

function RunStatusIcon({ status }: { status: string }) {
  switch (status) {
    case "completed":
      return <CheckCircle2 className="h-3 w-3 text-green-500" />
    case "failed":
      return <XCircle className="h-3 w-3 text-destructive" />
    case "cancelled":
      return <AlertTriangle className="h-3 w-3 text-yellow-500" />
    case "running":
      return <Loader2 className="h-3 w-3 animate-spin" />
    default:
      return <Clock className="h-3 w-3" />
  }
}

// --- Add Config Dialog ---

function AddConfigDialog({
  open,
  onOpenChange,
  arrInstances,
  instances,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  arrInstances: ArrInstance[]
  instances: Instance[]
}) {
  const { t } = useTranslation("crossseed")
  const createConfig = useCreateArrSeedConfig()

  const [form, setForm] = useState<Partial<ArrSeedConfigCreate>>({
    enabled: true,
    scanIntervalMinutes: 1440,
  })

  const handleCreate = () => {
    if (!form.arrInstanceId || !form.targetQbitInstanceId) {
      toast.error(t("arrScan.toast.selectInstances"))
      return
    }

    createConfig.mutate(
      {
        arrInstanceId: form.arrInstanceId,
        enabled: form.enabled ?? true,
        targetQbitInstanceId: form.targetQbitInstanceId,
        category: form.category ?? "",
        arrDockerPath: form.arrDockerPath ?? "",
        hostDataPath: form.hostDataPath ?? "",
        torrentSavePath: form.torrentSavePath ?? "",
        scanIntervalMinutes: form.scanIntervalMinutes ?? 1440,
        unmonitorAfterSeed: form.unmonitorAfterSeed ?? false,
        tagAfterSeed: form.tagAfterSeed ?? "",
      },
      {
        onSuccess: () => {
          toast.success(t("arrScan.toast.configCreated"))
          onOpenChange(false)
          setForm({ enabled: true, scanIntervalMinutes: 1440 })
        },
        onError: () => toast.error(t("arrScan.toast.configCreateFailed")),
      }
    )
  }

  const enabledArrInstances = arrInstances.filter((i) => i.enabled)

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("arrScan.configDialog.addTitle")}</DialogTitle>
          <DialogDescription>
            {t("arrScan.configDialog.addDescription")}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>{t("arrScan.configDialog.arrInstanceLabel")}</Label>
            <Select
              value={form.arrInstanceId ? String(form.arrInstanceId) : ""}
              onValueChange={(v) => setForm({ ...form, arrInstanceId: parseInt(v) })}
            >
              <SelectTrigger>
                <SelectValue placeholder={t("arrScan.configDialog.selectArrInstance")} />
              </SelectTrigger>
              <SelectContent>
                {enabledArrInstances.map((inst) => (
                  <SelectItem key={inst.id} value={String(inst.id)}>
                    {t("arrScan.configDialog.arrInstanceOption", {
                      name: inst.name,
                      type: t(`arrScan.instanceTypeLabels.${inst.type}`, inst.type),
                    })}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label>{t("arrScan.configDialog.targetInstanceLabel")}</Label>
            <Select
              value={form.targetQbitInstanceId ? String(form.targetQbitInstanceId) : ""}
              onValueChange={(v) => setForm({ ...form, targetQbitInstanceId: parseInt(v) })}
            >
              <SelectTrigger>
                <SelectValue placeholder={t("arrScan.configDialog.selectTargetInstance")} />
              </SelectTrigger>
              <SelectContent>
                {instances.map((inst) => (
                  <SelectItem key={inst.id} value={String(inst.id)}>
                    {inst.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label>{t("arrScan.configDialog.categoryLabel")}</Label>
            <Input
              value={form.category ?? ""}
              onChange={(e) => setForm({ ...form, category: e.target.value })}
              placeholder={t("arrScan.configDialog.categoryPlaceholder")}
            />
          </div>

          <div className="space-y-2">
            <Label className="flex items-center gap-1">
              {t("arrScan.configDialog.arrDockerPathLabel")}
              <FieldHelp>{t("arrScan.configDialog.arrDockerPathHelp")}</FieldHelp>
            </Label>
            <Input
              value={form.arrDockerPath ?? ""}
              onChange={(e) => setForm({ ...form, arrDockerPath: e.target.value })}
              placeholder={t("arrScan.configDialog.arrDockerPathPlaceholder")}
            />
          </div>

          <div className="space-y-2">
            <Label className="flex items-center gap-1">
              {t("arrScan.configDialog.hostDataPathLabel")}
              <FieldHelp>{t("arrScan.configDialog.hostDataPathHelp")}</FieldHelp>
            </Label>
            <Input
              value={form.hostDataPath ?? ""}
              onChange={(e) => setForm({ ...form, hostDataPath: e.target.value })}
              placeholder={t("arrScan.configDialog.hostDataPathPlaceholder")}
            />
          </div>

          <div className="space-y-2">
            <Label className="flex items-center gap-1">
              {t("arrScan.configDialog.torrentSavePathLabel")}
              <FieldHelp>{t("arrScan.configDialog.torrentSavePathHelp")}</FieldHelp>
            </Label>
            <Input
              value={form.torrentSavePath ?? ""}
              onChange={(e) => setForm({ ...form, torrentSavePath: e.target.value })}
              placeholder={t("arrScan.configDialog.torrentSavePathPlaceholder")}
            />
          </div>

          <div className="space-y-2">
            <Label>{t("arrScan.configDialog.scanIntervalLabel")}</Label>
            <Input
              type="number"
              value={form.scanIntervalMinutes ?? 1440}
              onChange={(e) =>
                setForm({ ...form, scanIntervalMinutes: parseInt(e.target.value) || 1440 })
              }
              min={60}
            />
          </div>

          <div className="flex items-center gap-2">
            <Switch
              checked={form.unmonitorAfterSeed ?? false}
              onCheckedChange={(checked) => setForm({ ...form, unmonitorAfterSeed: checked })}
            />
            <Label className="flex items-center gap-1">
              {t("arrScan.configDialog.unmonitorAfterSeedLabel")}
              <FieldHelp>{t("arrScan.configDialog.unmonitorAfterSeedHelp")}</FieldHelp>
            </Label>
          </div>

          <div className="space-y-2">
            <Label className="flex items-center gap-1">
              {t("arrScan.configDialog.tagAfterSeedLabel")}
              <FieldHelp>{t("arrScan.configDialog.tagAfterSeedHelp")}</FieldHelp>
            </Label>
            <Input
              value={form.tagAfterSeed ?? ""}
              onChange={(e) => setForm({ ...form, tagAfterSeed: e.target.value })}
              placeholder={t("arrScan.configDialog.tagAfterSeedPlaceholder")}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("arrScan.configDialog.cancel")}
          </Button>
          <Button onClick={handleCreate} disabled={createConfig.isPending}>
            {createConfig.isPending && <Loader2 className="h-4 w-4 animate-spin mr-1" />}
            {t("arrScan.configDialog.createButton")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// --- Edit Config Dialog ---

function EditConfigDialog({
  open,
  onOpenChange,
  config,
  instances,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  config: ArrSeedInstanceConfig
  instances: Instance[]
}) {
  const { t } = useTranslation("crossseed")
  const updateConfig = useUpdateArrSeedConfig(config.id)

  const [form, setForm] = useState({
    targetQbitInstanceId: config.targetQbitInstanceId,
    category: config.category,
    arrDockerPath: config.arrDockerPath,
    hostDataPath: config.hostDataPath,
    torrentSavePath: config.torrentSavePath,
    scanIntervalMinutes: config.scanIntervalMinutes,
    unmonitorAfterSeed: config.unmonitorAfterSeed,
    tagAfterSeed: config.tagAfterSeed,
  })

  // Reset form when config changes or dialog opens
  const handleOpenChange = (isOpen: boolean) => {
    if (isOpen) {
      setForm({
        targetQbitInstanceId: config.targetQbitInstanceId,
        category: config.category,
        arrDockerPath: config.arrDockerPath,
        hostDataPath: config.hostDataPath,
        torrentSavePath: config.torrentSavePath,
        scanIntervalMinutes: config.scanIntervalMinutes,
        unmonitorAfterSeed: config.unmonitorAfterSeed,
        tagAfterSeed: config.tagAfterSeed,
      })
    }
    onOpenChange(isOpen)
  }

  const handleSave = () => {
    updateConfig.mutate(
      {
        targetQbitInstanceId: form.targetQbitInstanceId,
        category: form.category,
        arrDockerPath: form.arrDockerPath,
        hostDataPath: form.hostDataPath,
        torrentSavePath: form.torrentSavePath,
        scanIntervalMinutes: form.scanIntervalMinutes,
        unmonitorAfterSeed: form.unmonitorAfterSeed,
        tagAfterSeed: form.tagAfterSeed,
      },
      {
        onSuccess: () => {
          toast.success(t("arrScan.toast.configUpdated"))
          onOpenChange(false)
        },
        onError: () => toast.error(t("arrScan.toast.configUpdateFailed")),
      }
    )
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{t("arrScan.configDialog.editTitle")}</DialogTitle>
          <DialogDescription>
            {t("arrScan.configDialog.editDescription", { name: config.arrInstanceName || t("arrScan.config.arrInstanceFallback", { id: config.arrInstanceId }) })}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>{t("arrScan.configDialog.targetInstanceLabel")}</Label>
            <Select
              value={String(form.targetQbitInstanceId)}
              onValueChange={(v) => setForm({ ...form, targetQbitInstanceId: parseInt(v) })}
            >
              <SelectTrigger>
                <SelectValue placeholder={t("arrScan.configDialog.selectTargetInstance")} />
              </SelectTrigger>
              <SelectContent>
                {instances.map((inst) => (
                  <SelectItem key={inst.id} value={String(inst.id)}>
                    {inst.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label>{t("arrScan.configDialog.categoryLabel")}</Label>
            <Input
              value={form.category}
              onChange={(e) => setForm({ ...form, category: e.target.value })}
              placeholder={t("arrScan.configDialog.categoryPlaceholder")}
            />
          </div>

          <div className="space-y-2">
            <Label className="flex items-center gap-1">
              {t("arrScan.configDialog.arrDockerPathLabel")}
              <FieldHelp>{t("arrScan.configDialog.arrDockerPathHelp")}</FieldHelp>
            </Label>
            <Input
              value={form.arrDockerPath}
              onChange={(e) => setForm({ ...form, arrDockerPath: e.target.value })}
              placeholder={t("arrScan.configDialog.arrDockerPathPlaceholder")}
            />
          </div>

          <div className="space-y-2">
            <Label className="flex items-center gap-1">
              {t("arrScan.configDialog.hostDataPathLabel")}
              <FieldHelp>{t("arrScan.configDialog.hostDataPathHelp")}</FieldHelp>
            </Label>
            <Input
              value={form.hostDataPath}
              onChange={(e) => setForm({ ...form, hostDataPath: e.target.value })}
              placeholder={t("arrScan.configDialog.hostDataPathPlaceholder")}
            />
          </div>

          <div className="space-y-2">
            <Label className="flex items-center gap-1">
              {t("arrScan.configDialog.torrentSavePathLabel")}
              <FieldHelp>{t("arrScan.configDialog.torrentSavePathHelp")}</FieldHelp>
            </Label>
            <Input
              value={form.torrentSavePath}
              onChange={(e) => setForm({ ...form, torrentSavePath: e.target.value })}
              placeholder={t("arrScan.configDialog.torrentSavePathPlaceholder")}
            />
          </div>

          <div className="space-y-2">
            <Label>{t("arrScan.configDialog.scanIntervalLabel")}</Label>
            <Input
              type="number"
              value={form.scanIntervalMinutes}
              onChange={(e) =>
                setForm({ ...form, scanIntervalMinutes: parseInt(e.target.value) || 1440 })
              }
              min={60}
            />
          </div>

          <div className="flex items-center gap-2">
            <Switch
              checked={form.unmonitorAfterSeed}
              onCheckedChange={(checked) => setForm({ ...form, unmonitorAfterSeed: checked })}
            />
            <Label className="flex items-center gap-1">
              {t("arrScan.configDialog.unmonitorAfterSeedLabel")}
              <FieldHelp>{t("arrScan.configDialog.unmonitorAfterSeedHelp")}</FieldHelp>
            </Label>
          </div>

          <div className="space-y-2">
            <Label className="flex items-center gap-1">
              {t("arrScan.configDialog.tagAfterSeedLabel")}
              <FieldHelp>{t("arrScan.configDialog.tagAfterSeedHelp")}</FieldHelp>
            </Label>
            <Input
              value={form.tagAfterSeed}
              onChange={(e) => setForm({ ...form, tagAfterSeed: e.target.value })}
              placeholder={t("arrScan.configDialog.tagAfterSeedPlaceholder")}
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            {t("arrScan.configDialog.cancel")}
          </Button>
          <Button onClick={handleSave} disabled={updateConfig.isPending}>
            {updateConfig.isPending && <Loader2 className="h-4 w-4 animate-spin mr-1" />}
            {t("arrScan.configDialog.saveButton")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
