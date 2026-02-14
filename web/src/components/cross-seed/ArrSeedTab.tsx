/*
 * Copyright (c) 2025-2026, s0up and the autobrr contributors.
 * SPDX-License-Identifier: GPL-2.0-or-later
 */

import { useCallback, useState } from "react"
import { toast } from "sonner"
import {
  AlertTriangle,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Clock,
  Info,
  Loader2,
  Pause,
  Pencil,
  Play,
  Plus,
  Radar,
  RotateCcw,
  Settings2,
  Trash2,
  XCircle,
} from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
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
  useTriggerArrSeedScan,
  useUpdateArrSeedConfig,
  useUpdateArrSeedSettings,
} from "@/hooks/useArrSeed"
import { api } from "@/lib/api"
import type { ArrInstance } from "@/types/arr"
import type { ArrSeedConfigCreate, ArrSeedInstanceConfig, ArrSeedProgress } from "@/types/arrseed"
import type { Instance } from "@/types"
import { useQuery } from "@tanstack/react-query"

interface ArrSeedTabProps {
  instances: Instance[]
}

export function ArrSeedTab({ instances }: ArrSeedTabProps) {
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
        onSuccess: () => toast.success(enabled ? "Arr Scan enabled" : "Arr Scan disabled"),
        onError: () => toast.error("Failed to update settings"),
      })
    },
    [updateSettings]
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
                Arr Scan
              </CardTitle>
              <CardDescription>
                Scan your Sonarr and Radarr libraries to find cross-seed matches on your indexers.
              </CardDescription>
            </div>
            <div className="flex items-center gap-4">
              <Button variant="outline" size="sm" onClick={() => setShowSettingsDialog(true)}>
                <Settings2 className="size-4 mr-2" />
                Settings
              </Button>
              <Label htmlFor="arrseed-enabled" className="flex items-center gap-2">
                <Switch
                  id="arrseed-enabled"
                  checked={settings?.enabled ?? false}
                  onCheckedChange={handleToggleEnabled}
                  disabled={updateSettings.isPending}
                />
                {settings?.enabled ? "Enabled" : "Disabled"}
              </Label>
            </div>
          </div>
        </CardHeader>
      </Card>

      {/* Warning */}
      <Alert className="border-destructive/20 bg-destructive/10 text-destructive">
        <AlertTriangle className="h-4 w-4 !text-destructive" />
        <AlertTitle>Run sparingly</AlertTitle>
        <AlertDescription>
          Each run scans your full Sonarr/Radarr library and queries indexers for every item. Use reasonable max items per run and search delay values to stay within indexer rate limits.
        </AlertDescription>
      </Alert>

      {/* Instance Configs */}
      <Card>
        <CardHeader>
          <div className="flex items-center justify-between">
            <div>
              <CardTitle className="text-base">Instance Configurations</CardTitle>
              <CardDescription>Configure cross-seeding for each Sonarr/Radarr instance</CardDescription>
            </div>
            <Button size="sm" onClick={() => setShowAddDialog(true)}>
              <Plus className="h-4 w-4 mr-1" />
              Add Config
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
              <p>No configurations yet. Add one to start cross-seeding from your Arr libraries.</p>
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
          toast.success("Settings saved")
          onOpenChange(false)
        },
        onError: () => toast.error("Failed to save settings"),
      }
    )
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Arr Scan Settings</DialogTitle>
          <DialogDescription>
            Configure global settings for Arr Scan.
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="space-y-2">
              <Label>Search Delay (seconds)</Label>
              <Input
                type="number"
                value={searchDelay}
                onChange={(e) => setSearchDelay(e.target.value)}
                min={0}
              />
              <p className="text-xs text-muted-foreground">Delay between indexer searches to avoid rate limiting</p>
            </div>
            <div className="space-y-2">
              <Label>Max Items Per Run</Label>
              <Input
                type="number"
                value={maxItems}
                onChange={(e) => setMaxItems(e.target.value)}
                min={0}
              />
              <p className="text-xs text-muted-foreground">0 = unlimited</p>
            </div>
          </div>
          <p className="text-xs text-muted-foreground">
            Size tolerance, start paused, and tags are configured in Cross Seed Rules and shared across all cross-seed modes.
          </p>
          <div className="space-y-2">
            <Label className="text-sm font-medium">Priority Tiers</Label>
            <p className="text-xs text-muted-foreground">Enable/disable which content types to cross-seed. Higher tiers are processed first.</p>
            <div className="space-y-2">
              <div className="flex items-center gap-2">
                <Switch
                  checked={settings?.enableSeasonPackHighScore ?? true}
                  onCheckedChange={(checked) =>
                    updateSettings.mutate({ enableSeasonPackHighScore: checked })
                  }
                />
                <Label className="text-sm">Season Pack (High Score)</Label>
                <span className="text-xs text-muted-foreground">Season packs with custom format score above upgrade-until</span>
              </div>
              <div className="flex items-center gap-2">
                <Switch
                  checked={settings?.enableSeasonPack ?? true}
                  onCheckedChange={(checked) =>
                    updateSettings.mutate({ enableSeasonPack: checked })
                  }
                />
                <Label className="text-sm">Season Pack</Label>
                <span className="text-xs text-muted-foreground">Full season downloads (S01 without episode number)</span>
              </div>
              <div className="flex items-center gap-2">
                <Switch
                  checked={settings?.enableEpisode ?? true}
                  onCheckedChange={(checked) =>
                    updateSettings.mutate({ enableEpisode: checked })
                  }
                />
                <Label className="text-sm">Episode</Label>
                <span className="text-xs text-muted-foreground">Individual episode files</span>
              </div>
            </div>
          </div>
          <div className="space-y-2">
            <Label className="text-sm font-medium">Season Pack Upgrade</Label>
            <div className="flex items-center gap-2">
              <Switch
                checked={settings?.enableSeasonPackUpgrade ?? false}
                onCheckedChange={(checked) =>
                  updateSettings.mutate({ enableSeasonPackUpgrade: checked })
                }
              />
              <Label className="text-sm">Enable Season Pack Upgrade</Label>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Info className="h-3.5 w-3.5 text-muted-foreground" />
                </TooltipTrigger>
                <TooltipContent className="max-w-xs">
                  When enabled, partial season packs will be injected and qBittorrent will download missing episodes as upgrades. After download completes, Sonarr will import the new files.
                </TooltipContent>
              </Tooltip>
            </div>
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleSave} disabled={updateSettings.isPending}>
            Save
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
  const triggerScan = useTriggerArrSeedScan()
  const cancelScan = useCancelArrSeedScan()
  const deleteConfig = useDeleteArrSeedConfig()
  const updateConfig = useUpdateArrSeedConfig(config.id)
  const { data: scanStatus } = useArrSeedScanStatus(config.id)
  const [showDelete, setShowDelete] = useState(false)
  const [showEdit, setShowEdit] = useState(false)

  const isRunning = scanStatus?.status === "running"

  const handleTriggerScan = () => {
    triggerScan.mutate(config.id, {
      onSuccess: () => toast.success("Scan started"),
      onError: (err) => toast.error(`Failed to start scan: ${err.message}`),
    })
  }

  const handleCancelScan = () => {
    cancelScan.mutate(config.id, {
      onSuccess: () => toast.success("Scan cancelled"),
      onError: () => toast.error("Failed to cancel scan"),
    })
  }

  const handleDelete = () => {
    deleteConfig.mutate(config.id, {
      onSuccess: () => {
        toast.success("Config deleted")
        setShowDelete(false)
      },
      onError: () => toast.error("Failed to delete config"),
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
                  <span className="font-medium">{config.arrInstanceName || `Arr #${config.arrInstanceId}`}</span>
                  <Badge variant="outline" className="text-xs">
                    {config.arrInstanceType || "unknown"}
                  </Badge>
                  {!config.enabled && (
                    <Badge variant="secondary" className="text-xs">disabled</Badge>
                  )}
                </div>
                <div className="text-sm text-muted-foreground">
                  Target: {config.targetQbitInstanceName || `qBit #${config.targetQbitInstanceId}`}
                  {config.category && ` | Category: ${config.category}`}
                </div>
              </div>
            </div>
            <div className="flex items-center gap-2">
              {isRunning && <ScanProgressBadge progress={scanStatus!} />}
              <Switch
                checked={config.enabled}
                onCheckedChange={handleToggleEnabled}
              />
              {isRunning ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button size="icon" variant="ghost" onClick={handleCancelScan}>
                      <Pause className="h-4 w-4" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>Cancel Scan</TooltipContent>
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
                  <TooltipContent>Start Scan</TooltipContent>
                </Tooltip>
              )}
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button size="icon" variant="ghost" onClick={() => setShowEdit(true)}>
                    <Pencil className="h-4 w-4" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>Edit</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger asChild>
                  <Button size="icon" variant="ghost" onClick={() => setShowDelete(true)}>
                    <Trash2 className="h-4 w-4 text-destructive" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent>Delete</TooltipContent>
              </Tooltip>
            </div>
          </div>

          {config.lastScanAt && (
            <div className="mt-1 text-xs text-muted-foreground flex items-center gap-1 ml-7">
              <Clock className="h-3 w-3" />
              Last scan: {formatDate(new Date(config.lastScanAt))}
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
            <AlertDialogTitle>Delete Configuration</AlertDialogTitle>
            <AlertDialogDescription>
              This will delete the configuration for &quot;{config.arrInstanceName}&quot; and all associated scan history and items.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction onClick={handleDelete} className="bg-destructive text-destructive-foreground">
              Delete
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
  return (
    <Badge variant="outline" className="text-xs gap-1">
      <Loader2 className="h-3 w-3 animate-spin" />
      {progress.phase}: {progress.itemsProcessed}/{progress.itemsTotal}
      {progress.torrentsAdded > 0 && ` (+${progress.torrentsAdded})`}
    </Badge>
  )
}

function ConfigDetails({ config }: { config: ArrSeedInstanceConfig }) {
  return (
    <div className="grid grid-cols-1 md:grid-cols-2 gap-3 text-sm">
      <div>
        <span className="text-muted-foreground">Arr Docker Path:</span>{" "}
        <code className="text-xs bg-muted px-1 rounded">{config.arrDockerPath || "(not set)"}</code>
      </div>
      <div>
        <span className="text-muted-foreground">Host Data Path:</span>{" "}
        <code className="text-xs bg-muted px-1 rounded">{config.hostDataPath || "(not set)"}</code>
      </div>
      <div>
        <span className="text-muted-foreground">Torrent Save Path:</span>{" "}
        <code className="text-xs bg-muted px-1 rounded">{config.torrentSavePath || "(not set)"}</code>
      </div>
      <div>
        <span className="text-muted-foreground">Scan Interval:</span>{" "}
        {config.scanIntervalMinutes} minutes
      </div>
      <div>
        <span className="text-muted-foreground">Unmonitor After Seed:</span>{" "}
        {config.unmonitorAfterSeed ? "Yes" : "No"}
      </div>
      <div>
        <span className="text-muted-foreground">Tag After Seed:</span>{" "}
        <code className="text-xs bg-muted px-1 rounded">{config.tagAfterSeed || "(none)"}</code>
      </div>
    </div>
  )
}

function ConfigHistory({ configId, formatDate }: { configId: number; formatDate: (d: Date) => string }) {
  const { data: runs } = useArrSeedRuns(configId)
  const { data: items } = useArrSeedItems(configId)
  const resetItems = useResetArrSeedItems()

  const handleReset = () => {
    resetItems.mutate(configId, {
      onSuccess: () => toast.success("Items reset for re-scanning"),
      onError: () => toast.error("Failed to reset items"),
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
          <span className="text-sm text-muted-foreground">Items:</span>
          {statusCounts.seeded && (
            <Badge variant="default" className="gap-1">
              <CheckCircle2 className="h-3 w-3" />
              {statusCounts.seeded} seeded
            </Badge>
          )}
          {statusCounts.pending && (
            <Badge variant="secondary">{statusCounts.pending} pending</Badge>
          )}
          {statusCounts.no_match && (
            <Badge variant="outline">{statusCounts.no_match} no match</Badge>
          )}
          {statusCounts.error && (
            <Badge variant="destructive" className="gap-1">
              <XCircle className="h-3 w-3" />
              {statusCounts.error} errors
            </Badge>
          )}
          <Button size="sm" variant="ghost" onClick={handleReset} className="h-6 text-xs">
            <RotateCcw className="h-3 w-3 mr-1" />
            Reset
          </Button>
        </div>
      )}

      {/* Recent runs */}
      {runs && runs.length > 0 && (
        <div>
          <p className="text-sm font-medium mb-1">Recent Runs</p>
          <div className="space-y-1">
            {runs.slice(0, 5).map((run) => (
              <div key={run.id} className="flex items-center gap-2 text-xs">
                <RunStatusIcon status={run.status} />
                <span className="text-muted-foreground">{formatDate(new Date(run.startedAt))}</span>
                <span>
                  {run.itemsScanned} scanned, {run.matchesFound} matched, {run.torrentsAdded} added
                </span>
                <Badge variant="outline" className="text-xs">{run.triggeredBy}</Badge>
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
  const createConfig = useCreateArrSeedConfig()

  const [form, setForm] = useState<Partial<ArrSeedConfigCreate>>({
    enabled: true,
    scanIntervalMinutes: 1440,
  })

  const handleCreate = () => {
    if (!form.arrInstanceId || !form.targetQbitInstanceId) {
      toast.error("Please select both an Arr instance and a target qBittorrent instance")
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
          toast.success("Config created")
          onOpenChange(false)
          setForm({ enabled: true, scanIntervalMinutes: 1440 })
        },
        onError: () => toast.error("Failed to create config"),
      }
    )
  }

  const enabledArrInstances = arrInstances.filter((i) => i.enabled)

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Add Arr Scan Configuration</DialogTitle>
          <DialogDescription>
            Configure cross-seeding for a Sonarr or Radarr instance
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>Arr Instance</Label>
            <Select
              value={form.arrInstanceId ? String(form.arrInstanceId) : ""}
              onValueChange={(v) => setForm({ ...form, arrInstanceId: parseInt(v) })}
            >
              <SelectTrigger>
                <SelectValue placeholder="Select an Arr instance" />
              </SelectTrigger>
              <SelectContent>
                {enabledArrInstances.map((inst) => (
                  <SelectItem key={inst.id} value={String(inst.id)}>
                    {inst.name} ({inst.type})
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="space-y-2">
            <Label>Target qBittorrent Instance</Label>
            <Select
              value={form.targetQbitInstanceId ? String(form.targetQbitInstanceId) : ""}
              onValueChange={(v) => setForm({ ...form, targetQbitInstanceId: parseInt(v) })}
            >
              <SelectTrigger>
                <SelectValue placeholder="Select a qBittorrent instance" />
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
            <Label>Category</Label>
            <Input
              value={form.category ?? ""}
              onChange={(e) => setForm({ ...form, category: e.target.value })}
              placeholder="e.g., TV.cross or MOVIES.cross"
            />
          </div>

          <div className="space-y-2">
            <Label>
              Arr Docker Path
              <Tooltip>
                <TooltipTrigger asChild>
                  <Info className="h-3.5 w-3.5 ml-1 inline text-muted-foreground" />
                </TooltipTrigger>
                <TooltipContent>
                  The path prefix as seen inside the Arr container (e.g., /mnt/media/tv)
                </TooltipContent>
              </Tooltip>
            </Label>
            <Input
              value={form.arrDockerPath ?? ""}
              onChange={(e) => setForm({ ...form, arrDockerPath: e.target.value })}
              placeholder="/mnt/media/tv"
            />
          </div>

          <div className="space-y-2">
            <Label>
              Host Data Path
              <Tooltip>
                <TooltipTrigger asChild>
                  <Info className="h-3.5 w-3.5 ml-1 inline text-muted-foreground" />
                </TooltipTrigger>
                <TooltipContent>
                  The matching path on the host system (e.g., /mnt/media/tv/data)
                </TooltipContent>
              </Tooltip>
            </Label>
            <Input
              value={form.hostDataPath ?? ""}
              onChange={(e) => setForm({ ...form, hostDataPath: e.target.value })}
              placeholder="/mnt/media/tv/data"
            />
          </div>

          <div className="space-y-2">
            <Label>
              Torrent Save Path
              <Tooltip>
                <TooltipTrigger asChild>
                  <Info className="h-3.5 w-3.5 ml-1 inline text-muted-foreground" />
                </TooltipTrigger>
                <TooltipContent>
                  Where hardlinks and torrent data should be saved (e.g., /mnt/media/tv/torrents)
                </TooltipContent>
              </Tooltip>
            </Label>
            <Input
              value={form.torrentSavePath ?? ""}
              onChange={(e) => setForm({ ...form, torrentSavePath: e.target.value })}
              placeholder="/mnt/media/tv/torrents"
            />
          </div>

          <div className="space-y-2">
            <Label>Scan Interval (minutes)</Label>
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
            <Label>Unmonitor after seed</Label>
            <Tooltip>
              <TooltipTrigger asChild>
                <Info className="h-3.5 w-3.5 text-muted-foreground" />
              </TooltipTrigger>
              <TooltipContent>
                Unmonitor the series/movie in Sonarr/Radarr after a successful cross-seed
              </TooltipContent>
            </Tooltip>
          </div>

          <div className="space-y-2">
            <Label>
              Tag after seed
              <Tooltip>
                <TooltipTrigger asChild>
                  <Info className="h-3.5 w-3.5 ml-1 inline text-muted-foreground" />
                </TooltipTrigger>
                <TooltipContent>
                  Add this tag to the series/movie in Sonarr/Radarr after a successful cross-seed
                </TooltipContent>
              </Tooltip>
            </Label>
            <Input
              value={form.tagAfterSeed ?? ""}
              onChange={(e) => setForm({ ...form, tagAfterSeed: e.target.value })}
              placeholder="e.g., cross-seeded"
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleCreate} disabled={createConfig.isPending}>
            {createConfig.isPending && <Loader2 className="h-4 w-4 animate-spin mr-1" />}
            Create
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
          toast.success("Config updated")
          onOpenChange(false)
        },
        onError: () => toast.error("Failed to update config"),
      }
    )
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>Edit Configuration</DialogTitle>
          <DialogDescription>
            Update settings for {config.arrInstanceName || `Arr #${config.arrInstanceId}`}
          </DialogDescription>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label>Target qBittorrent Instance</Label>
            <Select
              value={String(form.targetQbitInstanceId)}
              onValueChange={(v) => setForm({ ...form, targetQbitInstanceId: parseInt(v) })}
            >
              <SelectTrigger>
                <SelectValue placeholder="Select a qBittorrent instance" />
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
            <Label>Category</Label>
            <Input
              value={form.category}
              onChange={(e) => setForm({ ...form, category: e.target.value })}
              placeholder="e.g., TV.cross or MOVIES.cross"
            />
          </div>

          <div className="space-y-2">
            <Label>
              Arr Docker Path
              <Tooltip>
                <TooltipTrigger asChild>
                  <Info className="h-3.5 w-3.5 ml-1 inline text-muted-foreground" />
                </TooltipTrigger>
                <TooltipContent>
                  The path prefix as seen inside the Arr container (e.g., /mnt/media/tv)
                </TooltipContent>
              </Tooltip>
            </Label>
            <Input
              value={form.arrDockerPath}
              onChange={(e) => setForm({ ...form, arrDockerPath: e.target.value })}
              placeholder="/mnt/media/tv"
            />
          </div>

          <div className="space-y-2">
            <Label>
              Host Data Path
              <Tooltip>
                <TooltipTrigger asChild>
                  <Info className="h-3.5 w-3.5 ml-1 inline text-muted-foreground" />
                </TooltipTrigger>
                <TooltipContent>
                  The matching path on the host system (e.g., /mnt/media/tv/data)
                </TooltipContent>
              </Tooltip>
            </Label>
            <Input
              value={form.hostDataPath}
              onChange={(e) => setForm({ ...form, hostDataPath: e.target.value })}
              placeholder="/mnt/media/tv/data"
            />
          </div>

          <div className="space-y-2">
            <Label>
              Torrent Save Path
              <Tooltip>
                <TooltipTrigger asChild>
                  <Info className="h-3.5 w-3.5 ml-1 inline text-muted-foreground" />
                </TooltipTrigger>
                <TooltipContent>
                  Where hardlinks and torrent data should be saved (e.g., /mnt/media/tv/torrents)
                </TooltipContent>
              </Tooltip>
            </Label>
            <Input
              value={form.torrentSavePath}
              onChange={(e) => setForm({ ...form, torrentSavePath: e.target.value })}
              placeholder="/mnt/media/tv/torrents"
            />
          </div>

          <div className="space-y-2">
            <Label>Scan Interval (minutes)</Label>
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
            <Label>Unmonitor after seed</Label>
            <Tooltip>
              <TooltipTrigger asChild>
                <Info className="h-3.5 w-3.5 text-muted-foreground" />
              </TooltipTrigger>
              <TooltipContent>
                Unmonitor the series/movie in Sonarr/Radarr after a successful cross-seed
              </TooltipContent>
            </Tooltip>
          </div>

          <div className="space-y-2">
            <Label>
              Tag after seed
              <Tooltip>
                <TooltipTrigger asChild>
                  <Info className="h-3.5 w-3.5 ml-1 inline text-muted-foreground" />
                </TooltipTrigger>
                <TooltipContent>
                  Add this tag to the series/movie in Sonarr/Radarr after a successful cross-seed
                </TooltipContent>
              </Tooltip>
            </Label>
            <Input
              value={form.tagAfterSeed}
              onChange={(e) => setForm({ ...form, tagAfterSeed: e.target.value })}
              placeholder="e.g., cross-seeded"
            />
          </div>
        </div>
        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleSave} disabled={updateConfig.isPending}>
            {updateConfig.isPending && <Loader2 className="h-4 w-4 animate-spin mr-1" />}
            Save
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
