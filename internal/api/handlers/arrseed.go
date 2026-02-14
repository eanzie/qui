// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/qui/internal/models"
	"github.com/autobrr/qui/internal/services/crossseed"
)

// ArrSeedHandler handles HTTP requests for Arr Seed.
type ArrSeedHandler struct {
	service *crossseed.Service
	store   *models.ArrSeedStore
}

// NewArrSeedHandler creates a new ArrSeedHandler.
func NewArrSeedHandler(service *crossseed.Service, store *models.ArrSeedStore) *ArrSeedHandler {
	return &ArrSeedHandler{
		service: service,
		store:   store,
	}
}

// --- Settings ---

// GetSettings returns the global Arr Seed settings.
func (h *ArrSeedHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := h.store.GetSettings(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("arrseed: failed to get settings")
		RespondError(w, http.StatusInternalServerError, "Failed to get settings")
		return
	}

	if settings == nil {
		settings = &models.ArrSeedSettings{
			SearchDelaySeconds:        5,
			EnableSeasonPackHighScore: true,
			EnableSeasonPack:          true,
			EnableEpisode:             true,
		}
	}

	RespondJSON(w, http.StatusOK, settings)
}

// ArrSeedSettingsPayload is the request body for updating settings.
type ArrSeedSettingsPayload struct {
	Enabled                   *bool `json:"enabled"`
	SearchDelaySeconds        *int  `json:"searchDelaySeconds"`
	MaxItemsPerRun            *int  `json:"maxItemsPerRun"`
	EnableSeasonPackHighScore *bool `json:"enableSeasonPackHighScore"`
	EnableSeasonPack          *bool `json:"enableSeasonPack"`
	EnableEpisode             *bool `json:"enableEpisode"`
	EnableSeasonPackUpgrade   *bool `json:"enableSeasonPackUpgrade"`
}

// UpdateSettings updates the global Arr Seed settings.
func (h *ArrSeedHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	var payload ArrSeedSettingsPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	settings, err := h.store.GetSettings(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("arrseed: failed to get settings for update")
		RespondError(w, http.StatusInternalServerError, "Failed to get settings")
		return
	}

	if settings == nil {
		settings = &models.ArrSeedSettings{
			SearchDelaySeconds: 5,
		}
	}

	if payload.Enabled != nil {
		settings.Enabled = *payload.Enabled
	}
	if payload.SearchDelaySeconds != nil {
		settings.SearchDelaySeconds = *payload.SearchDelaySeconds
	}
	if payload.MaxItemsPerRun != nil {
		settings.MaxItemsPerRun = *payload.MaxItemsPerRun
	}
	if payload.EnableSeasonPackHighScore != nil {
		settings.EnableSeasonPackHighScore = *payload.EnableSeasonPackHighScore
	}
	if payload.EnableSeasonPack != nil {
		settings.EnableSeasonPack = *payload.EnableSeasonPack
	}
	if payload.EnableEpisode != nil {
		settings.EnableEpisode = *payload.EnableEpisode
	}
	if payload.EnableSeasonPackUpgrade != nil {
		settings.EnableSeasonPackUpgrade = *payload.EnableSeasonPackUpgrade
	}

	updated, err := h.store.UpdateSettings(r.Context(), settings)
	if err != nil {
		log.Error().Err(err).Msg("arrseed: failed to update settings")
		RespondError(w, http.StatusInternalServerError, "Failed to update settings")
		return
	}

	RespondJSON(w, http.StatusOK, updated)
}

// --- Configs ---

// ArrSeedConfigCreatePayload is the request body for creating a config.
type ArrSeedConfigCreatePayload struct {
	ArrInstanceID        int    `json:"arrInstanceId"`
	Enabled              bool   `json:"enabled"`
	TargetQbitInstanceID int    `json:"targetQbitInstanceId"`
	Category             string `json:"category"`
	ArrDockerPath        string `json:"arrDockerPath"`
	HostDataPath         string `json:"hostDataPath"`
	TorrentSavePath      string `json:"torrentSavePath"`
	ScanIntervalMinutes  int    `json:"scanIntervalMinutes"`
	UnmonitorAfterSeed   bool   `json:"unmonitorAfterSeed"`
	TagAfterSeed         string `json:"tagAfterSeed"`
}

// ListConfigs returns all Arr Seed instance configs.
func (h *ArrSeedHandler) ListConfigs(w http.ResponseWriter, r *http.Request) {
	configs, err := h.store.ListConfigs(r.Context())
	if err != nil {
		log.Error().Err(err).Msg("arrseed: failed to list configs")
		RespondError(w, http.StatusInternalServerError, "Failed to list configs")
		return
	}

	if configs == nil {
		configs = []*models.ArrSeedInstanceConfig{}
	}

	RespondJSON(w, http.StatusOK, configs)
}

// CreateConfig creates a new Arr Seed instance config.
func (h *ArrSeedHandler) CreateConfig(w http.ResponseWriter, r *http.Request) {
	var payload ArrSeedConfigCreatePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if payload.ArrInstanceID <= 0 {
		RespondError(w, http.StatusBadRequest, "arrInstanceId is required")
		return
	}
	if payload.TargetQbitInstanceID <= 0 {
		RespondError(w, http.StatusBadRequest, "targetQbitInstanceId is required")
		return
	}

	cfg := &models.ArrSeedInstanceConfig{
		ArrInstanceID:        payload.ArrInstanceID,
		Enabled:              payload.Enabled,
		TargetQbitInstanceID: payload.TargetQbitInstanceID,
		Category:             payload.Category,
		ArrDockerPath:        payload.ArrDockerPath,
		HostDataPath:         payload.HostDataPath,
		TorrentSavePath:      payload.TorrentSavePath,
		ScanIntervalMinutes:  payload.ScanIntervalMinutes,
		UnmonitorAfterSeed:   payload.UnmonitorAfterSeed,
		TagAfterSeed:         payload.TagAfterSeed,
	}

	if cfg.ScanIntervalMinutes <= 0 {
		cfg.ScanIntervalMinutes = 1440
	}

	created, err := h.store.CreateConfig(r.Context(), cfg)
	if err != nil {
		log.Error().Err(err).Msg("arrseed: failed to create config")
		RespondError(w, http.StatusInternalServerError, "Failed to create config")
		return
	}

	RespondJSON(w, http.StatusCreated, created)
}

// GetConfig returns a single config.
func (h *ArrSeedHandler) GetConfig(w http.ResponseWriter, r *http.Request) {
	id, err := parseConfigID(r)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid config ID")
		return
	}

	cfg, err := h.store.GetConfig(r.Context(), id)
	if err != nil {
		if errors.Is(err, models.ErrArrSeedConfigNotFound) {
			RespondError(w, http.StatusNotFound, "Config not found")
			return
		}
		log.Error().Err(err).Int("id", id).Msg("arrseed: failed to get config")
		RespondError(w, http.StatusInternalServerError, "Failed to get config")
		return
	}

	RespondJSON(w, http.StatusOK, cfg)
}

// ArrSeedConfigUpdatePayload is the request body for updating a config.
type ArrSeedConfigUpdatePayload struct {
	Enabled              *bool   `json:"enabled"`
	TargetQbitInstanceID *int    `json:"targetQbitInstanceId"`
	Category             *string `json:"category"`
	ArrDockerPath        *string `json:"arrDockerPath"`
	HostDataPath         *string `json:"hostDataPath"`
	TorrentSavePath      *string `json:"torrentSavePath"`
	ScanIntervalMinutes  *int    `json:"scanIntervalMinutes"`
	UnmonitorAfterSeed   *bool   `json:"unmonitorAfterSeed"`
	TagAfterSeed         *string `json:"tagAfterSeed"`
}

// UpdateConfig updates an Arr Seed instance config.
func (h *ArrSeedHandler) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	id, err := parseConfigID(r)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid config ID")
		return
	}

	var payload ArrSeedConfigUpdatePayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	params := &models.ArrSeedConfigUpdateParams{
		Enabled:              payload.Enabled,
		TargetQbitInstanceID: payload.TargetQbitInstanceID,
		Category:             payload.Category,
		ArrDockerPath:        payload.ArrDockerPath,
		HostDataPath:         payload.HostDataPath,
		TorrentSavePath:      payload.TorrentSavePath,
		ScanIntervalMinutes:  payload.ScanIntervalMinutes,
		UnmonitorAfterSeed:   payload.UnmonitorAfterSeed,
		TagAfterSeed:         payload.TagAfterSeed,
	}

	updated, err := h.store.UpdateConfig(r.Context(), id, params)
	if err != nil {
		if errors.Is(err, models.ErrArrSeedConfigNotFound) {
			RespondError(w, http.StatusNotFound, "Config not found")
			return
		}
		log.Error().Err(err).Int("id", id).Msg("arrseed: failed to update config")
		RespondError(w, http.StatusInternalServerError, "Failed to update config")
		return
	}

	RespondJSON(w, http.StatusOK, updated)
}

// DeleteConfig deletes an Arr Seed instance config.
func (h *ArrSeedHandler) DeleteConfig(w http.ResponseWriter, r *http.Request) {
	id, err := parseConfigID(r)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid config ID")
		return
	}

	if err := h.store.DeleteConfig(r.Context(), id); err != nil {
		if errors.Is(err, models.ErrArrSeedConfigNotFound) {
			RespondError(w, http.StatusNotFound, "Config not found")
			return
		}
		log.Error().Err(err).Int("id", id).Msg("arrseed: failed to delete config")
		RespondError(w, http.StatusInternalServerError, "Failed to delete config")
		return
	}

	RespondJSON(w, http.StatusNoContent, nil)
}

// --- Scan Operations ---

// TriggerScan starts a manual scan for a config.
func (h *ArrSeedHandler) TriggerScan(w http.ResponseWriter, r *http.Request) {
	id, err := parseConfigID(r)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid config ID")
		return
	}

	runID, err := h.service.ArrSeedStartManualScan(r.Context(), id)
	if err != nil {
		if errors.Is(err, models.ErrArrSeedRunAlreadyActive) {
			RespondError(w, http.StatusConflict, "A scan is already running for this config")
			return
		}
		log.Error().Err(err).Int("id", id).Msg("arrseed: failed to trigger scan")
		RespondError(w, http.StatusInternalServerError, "Failed to trigger scan")
		return
	}

	RespondJSON(w, http.StatusOK, map[string]int64{"runId": runID})
}

// StopScan gracefully stops a running scan (finishes current item, then stops).
func (h *ArrSeedHandler) StopScan(w http.ResponseWriter, r *http.Request) {
	id, err := parseConfigID(r)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid config ID")
		return
	}

	if err := h.service.ArrSeedStopScan(r.Context(), id); err != nil {
		log.Error().Err(err).Int("id", id).Msg("arrseed: failed to stop scan")
		RespondError(w, http.StatusInternalServerError, "Failed to stop scan")
		return
	}

	RespondJSON(w, http.StatusNoContent, nil)
}

// CancelScan immediately kills a running scan.
func (h *ArrSeedHandler) CancelScan(w http.ResponseWriter, r *http.Request) {
	id, err := parseConfigID(r)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid config ID")
		return
	}

	if err := h.service.ArrSeedCancelScan(r.Context(), id); err != nil {
		log.Error().Err(err).Int("id", id).Msg("arrseed: failed to cancel scan")
		RespondError(w, http.StatusInternalServerError, "Failed to cancel scan")
		return
	}

	RespondJSON(w, http.StatusNoContent, nil)
}

// GetScanStatus returns the live scan progress for a config.
func (h *ArrSeedHandler) GetScanStatus(w http.ResponseWriter, r *http.Request) {
	id, err := parseConfigID(r)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid config ID")
		return
	}

	progress := h.service.ArrSeedGetScanProgress(id)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(progress)
}

// --- History ---

// ListRuns lists run history for a config.
func (h *ArrSeedHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	id, err := parseConfigID(r)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid config ID")
		return
	}

	runs, err := h.store.ListRuns(r.Context(), id, 50)
	if err != nil {
		log.Error().Err(err).Int("id", id).Msg("arrseed: failed to list runs")
		RespondError(w, http.StatusInternalServerError, "Failed to list runs")
		return
	}

	if runs == nil {
		runs = []*models.ArrSeedRun{}
	}

	RespondJSON(w, http.StatusOK, runs)
}

// ListItems lists tracked items for a config.
func (h *ArrSeedHandler) ListItems(w http.ResponseWriter, r *http.Request) {
	id, err := parseConfigID(r)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid config ID")
		return
	}

	var statusFilter *models.ArrSeedItemStatus
	if s := r.URL.Query().Get("status"); s != "" {
		status := models.ArrSeedItemStatus(s)
		statusFilter = &status
	}

	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, parseErr := strconv.Atoi(l); parseErr == nil && parsed > 0 {
			limit = parsed
		}
	}

	offset := 0
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, parseErr := strconv.Atoi(o); parseErr == nil && parsed >= 0 {
			offset = parsed
		}
	}

	items, err := h.store.ListItems(r.Context(), id, statusFilter, limit, offset)
	if err != nil {
		log.Error().Err(err).Int("id", id).Msg("arrseed: failed to list items")
		RespondError(w, http.StatusInternalServerError, "Failed to list items")
		return
	}

	if items == nil {
		items = []*models.ArrSeedItem{}
	}

	RespondJSON(w, http.StatusOK, items)
}

// ResetItems resets all items for a config to allow re-scanning.
func (h *ArrSeedHandler) ResetItems(w http.ResponseWriter, r *http.Request) {
	id, err := parseConfigID(r)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid config ID")
		return
	}

	if err := h.store.DeleteItemsForConfig(r.Context(), id); err != nil {
		log.Error().Err(err).Int("id", id).Msg("arrseed: failed to reset items")
		RespondError(w, http.StatusInternalServerError, "Failed to reset items")
		return
	}

	RespondJSON(w, http.StatusNoContent, nil)
}

func parseConfigID(r *http.Request) (int, error) {
	idStr := chi.URLParam(r, "configID")
	return strconv.Atoi(idStr)
}
