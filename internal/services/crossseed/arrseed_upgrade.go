// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package crossseed

import (
	"context"
	"time"

	qbt "github.com/autobrr/go-qbittorrent"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/autobrr/qui/internal/models"
	"github.com/autobrr/qui/internal/services/arr"
)

// arrSeedMonitorPartialUpgrade polls qBit for a partial season pack torrent and triggers
// Sonarr import + post-seed actions when the download completes.
func (r *ArrSeedRunner) monitorPartialUpgrade(
	instanceID int,
	torrentHash string,
	arrClient *arr.Client,
	mediaItem *ArrSeedMediaItem,
	config *models.ArrSeedInstanceConfig,
) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
	defer cancel()

	l := log.With().
		Str("hash", torrentHash).
		Int("instanceID", instanceID).
		Str("release", mediaItem.ReleaseName).
		Logger()

	l.Info().Msg("arrseed: monitoring partial upgrade download")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			l.Warn().Msg("arrseed: partial upgrade monitor timed out")
			return
		case <-ticker.C:
		}

		torrent, found, err := r.svc.syncManager.HasTorrentByAnyHash(ctx, instanceID, []string{torrentHash})
		if err != nil {
			l.Debug().Err(err).Msg("arrseed: failed to check torrent status")
			continue
		}
		if !found || torrent == nil {
			l.Warn().Msg("arrseed: torrent no longer found in qBit")
			return
		}

		switch torrent.State {
		case qbt.TorrentStateCheckingDl, qbt.TorrentStateCheckingUp,
			qbt.TorrentStateCheckingResumeData, qbt.TorrentStateAllocating:
			continue
		}

		if torrent.AmountLeft > 0 {
			continue
		}

		l.Info().Msg("arrseed: partial upgrade download complete, triggering import")

		r.triggerArrImport(ctx, arrClient, mediaItem, &l)
		r.postSeedExecutor.execute(ctx, arrClient, mediaItem, config, &l)

		l.Info().Msg("arrseed: partial upgrade post-download actions complete")
		return
	}
}

// triggerArrImport triggers Sonarr/Radarr to rescan and import new files.
func (r *ArrSeedRunner) triggerArrImport(ctx context.Context, client *arr.Client, item *ArrSeedMediaItem, l *zerolog.Logger) {
	var cmdName string
	params := map[string]any{}

	switch item.ArrInstanceType {
	case "sonarr":
		cmdName = "RescanSeries"
		params["seriesId"] = item.SeriesID
	case "radarr":
		cmdName = "RescanMovie"
		params["movieId"] = item.MovieID
	default:
		l.Warn().Str("type", item.ArrInstanceType).Msg("arrseed: unknown arr type for import")
		return
	}

	cmd, err := client.SendCommand(ctx, cmdName, params)
	if err != nil {
		l.Warn().Err(err).Str("command", cmdName).Msg("arrseed: failed to send import command")
		return
	}

	l.Info().Int("commandId", cmd.ID).Str("command", cmdName).Msg("arrseed: import command sent, waiting for completion")

	pollCtx, pollCancel := context.WithTimeout(ctx, 5*time.Minute)
	defer pollCancel()

	pollTicker := time.NewTicker(5 * time.Second)
	defer pollTicker.Stop()

	for {
		select {
		case <-pollCtx.Done():
			l.Warn().Int("commandId", cmd.ID).Msg("arrseed: timed out waiting for import command")
			return
		case <-pollTicker.C:
		}

		status, err := client.GetCommandStatus(pollCtx, cmd.ID)
		if err != nil {
			l.Debug().Err(err).Msg("arrseed: failed to check command status")
			continue
		}

		if status.Status == "completed" {
			l.Info().Int("commandId", cmd.ID).Msg("arrseed: import command completed")
			return
		}
		if status.Status == "failed" {
			l.Warn().Int("commandId", cmd.ID).Msg("arrseed: import command failed")
			return
		}
	}
}
