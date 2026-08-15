// Copyright (c) 2025-2026, s0up and the autobrr contributors.
// SPDX-License-Identifier: GPL-2.0-or-later

package crossseed

import (
	"bytes"
	"testing"

	"github.com/autobrr/go-torrent/bencode"
	"github.com/autobrr/go-torrent/metainfo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArrSeedExtractEpisodeID(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     string
	}{
		{name: "S01E02 pattern", filename: "Show.S01E02.720p.mkv", want: "e02"},
		{name: "S03E15 pattern", filename: "Show.S03E15.HDTV.mkv", want: "e15"},
		{name: "standalone E at start", filename: "E05.Something.mkv", want: "e05"},
		{name: "standalone E mid-string dots", filename: "Something.E12.720p.mkv", want: "e12"},
		{name: "standalone E underscores", filename: "Something_E03_720p.mkv", want: "e03"},
		{name: "standalone E hyphens", filename: "Something-E07-720p.mkv", want: "e07"},
		{name: "standalone E spaces", filename: "Something E99 720p.mkv", want: "e99"},
		{name: "no episode marker", filename: "NoEpisodeHere.mkv", want: ""},
		{name: "E in middle of word", filename: "BEST2024.mkv", want: ""},
		{name: "lowercase s01e02", filename: "s01e02.lowercase.mkv", want: "e02"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := arrSeedExtractEpisodeID(tt.filename)
			assert.Equal(t, tt.want, got)
		})
	}
}

func buildTestTorrent(t *testing.T, name string, files []metainfo.FileInfo) []byte {
	t.Helper()

	info := metainfo.Info{
		PieceLength: 262144,
		Pieces:      make([]byte, 20), // 1 fake piece hash
		Name:        name,
	}
	if len(files) == 0 {
		info.Length = 1024 // single-file torrent
	} else {
		info.Files = files
	}

	mi := metainfo.MetaInfo{}
	mi.InfoBytes, _ = bencode.Marshal(info)

	var buf bytes.Buffer
	require.NoError(t, mi.Write(&buf))
	return buf.Bytes()
}

func TestArrSeedParseTorrentBytes(t *testing.T) {
	t.Run("single file torrent", func(t *testing.T) {
		data := buildTestTorrent(t, "test.mkv", nil)

		parsed, err := arrSeedParseTorrentBytes(data)
		require.NoError(t, err)

		assert.Equal(t, "test.mkv", parsed.Name)
		require.Len(t, parsed.Files, 1)
		assert.Equal(t, "test.mkv", parsed.Files[0].Path)
	})

	t.Run("multi-file torrent", func(t *testing.T) {
		files := []metainfo.FileInfo{
			{Path: []string{"video.mkv"}, Length: 2048},
			{Path: []string{"subs", "eng.srt"}, Length: 512},
		}
		data := buildTestTorrent(t, "TestRelease", files)

		parsed, err := arrSeedParseTorrentBytes(data)
		require.NoError(t, err)

		assert.Equal(t, "TestRelease", parsed.Name)
		require.Len(t, parsed.Files, 2)

		// The parser prepends the torrent name as root when the first path
		// segment does not already match it.
		paths := make([]string, len(parsed.Files))
		for i, f := range parsed.Files {
			paths[i] = f.Path
		}
		assert.Contains(t, paths, "TestRelease/video.mkv")
		assert.Contains(t, paths, "TestRelease/subs/eng.srt")
	})

	t.Run("info hash is non-empty hex", func(t *testing.T) {
		data := buildTestTorrent(t, "hash-check.mkv", nil)

		parsed, err := arrSeedParseTorrentBytes(data)
		require.NoError(t, err)

		assert.NotEmpty(t, parsed.InfoHash)
		assert.Len(t, parsed.InfoHash, 40, "info hash should be 40-char hex string")
	})

	t.Run("invalid bytes returns error", func(t *testing.T) {
		_, err := arrSeedParseTorrentBytes([]byte("not a torrent"))
		assert.Error(t, err)
	})
}
