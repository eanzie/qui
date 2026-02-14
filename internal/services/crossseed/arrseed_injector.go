// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package crossseed

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/anacrolix/torrent/metainfo"
	"github.com/rs/zerolog"

	"github.com/autobrr/qui/internal/models"
	qbsync "github.com/autobrr/qui/internal/qbittorrent"
	"github.com/autobrr/qui/internal/services/jackett"
	"github.com/autobrr/qui/pkg/hardlinktree"
)

const (
	arrSeedQbitBoolTrue  = "true"
	arrSeedQbitBoolFalse = "false"

	arrSeedQbitContentLayoutOriginal = "Original"
)

// arrSeedInjector handles downloading and injecting torrents into qBittorrent.
type arrSeedInjector struct {
	svc *Service // parent crossseed service
}

// arrSeedParsedTorrent contains parsed .torrent metadata.
type arrSeedParsedTorrent struct {
	Name     string
	InfoHash string
	Files    []arrSeedTorrentFile
}

type arrSeedTorrentFile struct {
	Path string
	Size int64
}

// Ignored extensions — scene metadata, subtitles, text files.
var arrSeedIgnoredExtensions = map[string]bool{
	".nfo": true, ".srr": true,
	".srt": true, ".sub": true, ".idx": true, ".ass": true, ".ssa": true, ".sup": true, ".vtt": true,
	".txt": true,
}

// Ignored path keywords — samples, proofs, extras.
var arrSeedIgnoredPathKeywords = []string{
	"sample", "!sample", "proof", "extras", "bonus", "trailer", "featurette",
}

// archiveExtensions for layout classification.
var arrSeedArchiveExtensions = buildArrSeedArchiveExtSet()

func buildArrSeedArchiveExtSet() map[string]bool {
	exts := map[string]bool{".rar": true, ".zip": true, ".7z": true}
	for i := 0; i <= 99; i++ {
		if i < 10 {
			exts[".r0"+strconv.Itoa(i)] = true
		} else {
			exts[".r"+strconv.Itoa(i)] = true
		}
	}
	return exts
}

// arrSeedShouldIgnoreFile returns true if the file is a metadata/subtitle/sample file.
func arrSeedShouldIgnoreFile(path string) bool {
	lower := strings.ToLower(path)
	ext := filepath.Ext(lower)
	if arrSeedIgnoredExtensions[ext] {
		return true
	}
	for _, kw := range arrSeedIgnoredPathKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// arrSeedIsArchiveTorrent checks if the torrent's largest file is a RAR/archive.
func arrSeedIsArchiveTorrent(files []arrSeedTorrentFile) bool {
	var largestName string
	var largestSize int64
	for _, f := range files {
		if arrSeedShouldIgnoreFile(f.Path) {
			continue
		}
		if f.Size > largestSize {
			largestSize = f.Size
			largestName = f.Path
		}
	}
	if largestSize == 0 {
		return false
	}
	lower := strings.ToLower(largestName)
	for _, suffix := range []string{".tar.gz", ".tar.xz", ".tar.bz2", ".tar"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return arrSeedArchiveExtensions[filepath.Ext(lower)]
}

// tryInject downloads a .torrent for the search result, creates hardlinks, and adds to qBittorrent.
func (inj *arrSeedInjector) tryInject(
	ctx context.Context,
	item *ArrSeedMediaItem,
	result *jackett.SearchResult,
	config *models.ArrSeedInstanceConfig,
	injOpts *arrSeedInjectionOptions,
	l *zerolog.Logger,
) (*ArrSeedInjectResult, error) {
	if inj.svc.jackettService == nil {
		return nil, errors.New("jackett downloader is nil")
	}
	if inj.svc.syncManager == nil {
		return nil, errors.New("torrent adder is nil")
	}

	// Download the .torrent file
	l.Debug().
		Str("title", result.Title).
		Str("indexer", result.Indexer).
		Str("downloadURL", result.DownloadURL).
		Msg("arrseed: downloading .torrent file")
	torrentBytes, err := inj.svc.jackettService.DownloadTorrent(ctx, jackett.TorrentDownloadRequest{
		IndexerID:   result.IndexerID,
		DownloadURL: result.DownloadURL,
		GUID:        result.GUID,
		Title:       result.Title,
		Size:        result.Size,
		Pace:        true,
	})
	if err != nil {
		l.Warn().Err(err).Str("title", result.Title).Msg("arrseed: failed to download .torrent")
		return &ArrSeedInjectResult{ErrorMessage: fmt.Sprintf("download torrent: %v", err)}, err
	}
	l.Debug().Int("bytes", len(torrentBytes)).Msg("arrseed: .torrent downloaded")

	// Parse the torrent
	parsed, err := arrSeedParseTorrentBytes(torrentBytes)
	if err != nil {
		l.Warn().Err(err).Msg("arrseed: failed to parse .torrent")
		return &ArrSeedInjectResult{ErrorMessage: fmt.Sprintf("parse torrent: %v", err)}, err
	}
	l.Info().
		Str("torrentName", parsed.Name).
		Str("infoHash", parsed.InfoHash).
		Int("fileCount", len(parsed.Files)).
		Msg("arrseed: .torrent parsed")
	for _, f := range parsed.Files {
		l.Debug().Str("path", f.Path).Int64("size", f.Size).Msg("arrseed: torrent file entry")
	}

	// Reject archive-packed torrents
	if arrSeedIsArchiveTorrent(parsed.Files) {
		l.Info().Str("torrentName", parsed.Name).Msg("arrseed: skipping archive-packed torrent (RAR/ZIP)")
		return &ArrSeedInjectResult{ErrorMessage: "torrent contains archive files (RAR/ZIP), cannot hardlink"}, nil
	}

	// Check if torrent already exists in qBit
	if inj.svc.syncManager != nil {
		l.Debug().Str("hash", parsed.InfoHash).Int("instanceID", config.TargetQbitInstanceID).Msg("arrseed: checking if torrent exists in qBit")
		_, exists, checkErr := inj.svc.syncManager.HasTorrentByAnyHash(ctx, config.TargetQbitInstanceID, []string{parsed.InfoHash})
		if checkErr != nil {
			l.Debug().Err(checkErr).Str("hash", parsed.InfoHash).Msg("arrseed: failed to check torrent existence")
		} else if exists {
			l.Info().Str("hash", parsed.InfoHash).Msg("arrseed: torrent already exists in qBit, skipping")
			return &ArrSeedInjectResult{
				Success:     true,
				TorrentHash: parsed.InfoHash,
			}, nil
		}
		l.Debug().Msg("arrseed: torrent does not exist in qBit, proceeding with injection")
	}

	// Verify the source file(s) exist on disk
	if len(item.HostFiles) > 0 {
		for _, hf := range item.HostFiles {
			l.Debug().Str("path", hf.Path).Msg("arrseed: verifying source file exists")
			fi, statErr := os.Stat(hf.Path)
			if statErr != nil {
				l.Warn().Err(statErr).Str("path", hf.Path).Msg("arrseed: source file not found on disk")
				return &ArrSeedInjectResult{ErrorMessage: fmt.Sprintf("source file not found: %v", statErr)}, statErr
			}
			l.Debug().Str("path", hf.Path).Int64("diskSize", fi.Size()).Int64("expectedSize", hf.Size).Msg("arrseed: source file verified")
		}
	} else {
		l.Debug().Str("path", item.HostFilePath).Msg("arrseed: verifying source file exists")
		fileInfo, statErr := os.Stat(item.HostFilePath)
		if statErr != nil {
			l.Warn().Err(statErr).Str("path", item.HostFilePath).Msg("arrseed: source file not found on disk")
			return &ArrSeedInjectResult{ErrorMessage: fmt.Sprintf("source file not found: %v", statErr)}, statErr
		}
		l.Debug().Str("path", item.HostFilePath).Int64("diskSize", fileInfo.Size()).Int64("expectedSize", item.FileSize).Msg("arrseed: source file verified")
	}

	// Determine if partial upgrade is allowed
	allowPartial := item.ItemType == "season_pack" &&
		injOpts.EnableSeasonPackUpgrade &&
		item.CustomFormatScore >= item.MinFormatScore

	// Build hardlink tree
	savePath := config.TorrentSavePath
	l.Info().
		Str("savePath", savePath).
		Str("torrentName", parsed.Name).
		Str("sourceFile", item.HostFilePath).
		Bool("allowPartial", allowPartial).
		Msg("arrseed: building hardlink plan")
	plan, unmatchedCount, err := inj.buildHardlinkPlan(item, parsed, savePath, allowPartial, injOpts.SizeMismatchTolerancePercent)
	if err != nil {
		l.Warn().Err(err).Msg("arrseed: failed to build hardlink plan")
		return &ArrSeedInjectResult{ErrorMessage: fmt.Sprintf("build hardlink plan: %v", err)}, err
	}
	l.Info().
		Str("rootDir", plan.RootDir).
		Int("files", len(plan.Files)).
		Int("unmatched", unmatchedCount).
		Msg("arrseed: hardlink plan built")
	for _, f := range plan.Files {
		l.Debug().Str("src", f.SourcePath).Str("dst", f.TargetPath).Msg("arrseed: planned hardlink")
	}

	isPartialUpgrade := unmatchedCount > 0

	// Create hardlinks
	l.Info().Msg("arrseed: creating hardlinks")
	if err := hardlinktree.Create(plan); err != nil {
		l.Warn().Err(err).Msg("arrseed: failed to create hardlinks")
		return &ArrSeedInjectResult{ErrorMessage: fmt.Sprintf("create hardlinks: %v", err)}, err
	}
	l.Info().Str("rootDir", plan.RootDir).Msg("arrseed: hardlinks created successfully")

	// Build qBit add options
	options := arrSeedBuildAddOptions(config.Category, injOpts, savePath)

	// For partial upgrades, override paused/stopped so qBit downloads missing files.
	if isPartialUpgrade {
		options["paused"] = arrSeedQbitBoolFalse
		delete(options, "stopped")
	}

	l.Info().
		Str("category", config.Category).
		Str("savepath", options["savepath"]).
		Str("paused", options["paused"]).
		Int("instanceID", config.TargetQbitInstanceID).
		Bool("partialUpgrade", isPartialUpgrade).
		Msg("arrseed: adding torrent to qBittorrent")

	// Add the torrent to qBittorrent
	if err := inj.svc.syncManager.AddTorrent(ctx, config.TargetQbitInstanceID, torrentBytes, options); err != nil {
		l.Warn().Err(err).Str("hash", parsed.InfoHash).Msg("arrseed: failed to add torrent to qBit, rolling back hardlinks")
		if rollbackErr := hardlinktree.Rollback(plan); rollbackErr != nil {
			l.Warn().Err(rollbackErr).Str("rootDir", plan.RootDir).Msg("arrseed: failed to rollback hardlink tree")
		}
		return &ArrSeedInjectResult{ErrorMessage: fmt.Sprintf("add torrent: %v", err)}, err
	}
	l.Info().Str("hash", parsed.InfoHash).Msg("arrseed: torrent added to qBittorrent")

	if isPartialUpgrade {
		l.Info().
			Str("hash", parsed.InfoHash).
			Int("hardlinked", len(plan.Files)).
			Int("unmatched", unmatchedCount).
			Msg("arrseed: partial season pack upgrade — triggering recheck, qBit will download remaining files")
		if err := inj.svc.syncManager.BulkAction(ctx, config.TargetQbitInstanceID, []string{parsed.InfoHash}, "recheck"); err != nil {
			l.Warn().Err(err).Str("hash", parsed.InfoHash).Msg("arrseed: failed to trigger recheck for partial upgrade")
		}
	} else if injOpts.StartPaused {
		l.Info().Str("hash", parsed.InfoHash).Msg("arrseed: triggering recheck for paused torrent")
		if err := inj.svc.syncManager.BulkAction(ctx, config.TargetQbitInstanceID, []string{parsed.InfoHash}, "recheck"); err != nil {
			l.Warn().Err(err).Str("hash", parsed.InfoHash).Msg("arrseed: failed to trigger recheck")
		} else {
			l.Info().Str("hash", parsed.InfoHash).Msg("arrseed: recheck triggered, setting up resume-when-complete")
			inj.svc.syncManager.ResumeWhenComplete(config.TargetQbitInstanceID, []string{parsed.InfoHash}, qbsync.ResumeWhenCompleteOptions{
				Timeout: 60 * time.Minute,
			})
		}
	}

	return &ArrSeedInjectResult{
		Success:          true,
		TorrentHash:      parsed.InfoHash,
		IsPartialUpgrade: isPartialUpgrade,
		UnmatchedCount:   unmatchedCount,
	}, nil
}

func (inj *arrSeedInjector) buildHardlinkPlan(item *ArrSeedMediaItem, parsed *arrSeedParsedTorrent, savePath string, allowPartial bool, sizeTolerance float64) (*hardlinktree.TreePlan, int, error) {
	var existingFiles []hardlinktree.ExistingFile
	if len(item.HostFiles) > 0 {
		existingFiles = make([]hardlinktree.ExistingFile, 0, len(item.HostFiles))
		for _, hf := range item.HostFiles {
			existingFiles = append(existingFiles, hardlinktree.ExistingFile{
				AbsPath: hf.Path,
				RelPath: filepath.Base(hf.Path),
				Size:    hf.Size,
			})
		}
	} else {
		existingFiles = []hardlinktree.ExistingFile{
			{
				AbsPath: item.HostFilePath,
				RelPath: filepath.Base(item.HostFilePath),
				Size:    item.FileSize,
			},
		}
	}

	if err := os.MkdirAll(savePath, 0o750); err != nil {
		return nil, 0, fmt.Errorf("create save path: %w", err)
	}

	allCandidates := make([]hardlinktree.TorrentFile, 0, len(parsed.Files))
	for _, f := range parsed.Files {
		allCandidates = append(allCandidates, hardlinktree.TorrentFile{
			Path: f.Path,
			Size: f.Size,
		})
	}

	if !allowPartial {
		plan, err := hardlinktree.BuildPlan(allCandidates, existingFiles, hardlinktree.LayoutOriginal, parsed.Name, savePath)
		return plan, 0, err
	}

	// Partial mode: filter out metadata files, then match by size with tolerance.
	var contentCandidates []hardlinktree.TorrentFile
	for _, cf := range allCandidates {
		if !arrSeedShouldIgnoreFile(cf.Path) {
			contentCandidates = append(contentCandidates, cf)
		}
	}

	consumed := make([]bool, len(existingFiles))
	var matchedCandidates []hardlinktree.TorrentFile
	var matchedExisting []hardlinktree.ExistingFile

	// First pass: exact match by base filename + size.
	nameToExisting := make(map[string][]int)
	for i, ef := range existingFiles {
		base := strings.ToLower(filepath.Base(ef.AbsPath))
		nameToExisting[base] = append(nameToExisting[base], i)
	}

	matchedTorrentIdx := make(map[int]bool)
	for ci, cf := range contentCandidates {
		candBase := strings.ToLower(filepath.Base(cf.Path))
		for _, idx := range nameToExisting[candBase] {
			if consumed[idx] {
				continue
			}
			if existingFiles[idx].Size == cf.Size {
				matchedCandidates = append(matchedCandidates, cf)
				matchedExisting = append(matchedExisting, existingFiles[idx])
				consumed[idx] = true
				matchedTorrentIdx[ci] = true
				break
			}
		}
	}

	// Second pass: remaining unmatched candidates, try size-only with tolerance.
	for ci, cf := range contentCandidates {
		if matchedTorrentIdx[ci] {
			continue
		}
		for i, ef := range existingFiles {
			if consumed[i] {
				continue
			}
			if isSizeWithinTolerance(ef.Size, cf.Size, sizeTolerance) {
				matchedCandidates = append(matchedCandidates, cf)
				matchedExisting = append(matchedExisting, ef)
				consumed[i] = true
				break
			}
		}
	}

	unmatchedCount := len(contentCandidates) - len(matchedCandidates)

	if len(matchedCandidates) == 0 {
		return nil, unmatchedCount, fmt.Errorf("no matching files for partial upgrade (0/%d)", len(contentCandidates))
	}

	plan, err := hardlinktree.BuildPlan(matchedCandidates, matchedExisting, hardlinktree.LayoutOriginal, parsed.Name, savePath)
	return plan, unmatchedCount, err
}

func arrSeedParseTorrentBytes(data []byte) (*arrSeedParsedTorrent, error) {
	mi, err := metainfo.Load(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("parse torrent: %w", err)
	}

	info, err := mi.UnmarshalInfo()
	if err != nil {
		return nil, fmt.Errorf("unmarshal info: %w", err)
	}

	parsed := &arrSeedParsedTorrent{
		Name:     info.BestName(),
		InfoHash: mi.HashInfoBytes().HexString(),
	}

	if len(info.Files) == 0 {
		parsed.Files = []arrSeedTorrentFile{{
			Path: info.BestName(),
			Size: info.Length,
		}}
	} else {
		root := info.BestName()
		for _, f := range info.Files {
			parts := f.Path
			if root != "" && (len(parts) == 0 || parts[0] != root) {
				parts = append([]string{root}, parts...)
			}
			parsed.Files = append(parsed.Files, arrSeedTorrentFile{
				Path: filepath.Join(parts...),
				Size: f.Length,
			})
		}
	}

	return parsed, nil
}

func arrSeedBuildAddOptions(category string, injOpts *arrSeedInjectionOptions, savePath string) map[string]string {
	options := map[string]string{
		"autoTMM":       arrSeedQbitBoolFalse,
		"contentLayout": arrSeedQbitContentLayoutOriginal,
		"root_folder":   arrSeedQbitBoolFalse,
		"savepath":      savePath,
	}

	if category != "" {
		options["category"] = category
	}

	if len(injOpts.Tags) > 0 {
		options["tags"] = strings.Join(injOpts.Tags, ",")
	}

	if injOpts.StartPaused {
		options["paused"] = arrSeedQbitBoolTrue
		options["stopped"] = arrSeedQbitBoolTrue
	}

	return options
}
