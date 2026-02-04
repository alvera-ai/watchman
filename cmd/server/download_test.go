// Copyright 2022 The Moov Authors
// Use of this source code is governed by an Apache License
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moov-io/watchman/internal/download"
	"github.com/moov-io/watchman/internal/fshelp"
	"github.com/moov-io/watchman/internal/index"
	"github.com/moov-io/watchman/internal/tfidf"
	"github.com/moov-io/watchman/pkg/search"

	"github.com/moov-io/base/log"
	"github.com/stretchr/testify/require"
)

func TestGetRefreshInterval(t *testing.T) {
	conf := download.Config{
		RefreshInterval: 2 * time.Minute,
	}
	got := getRefreshInterval(conf)
	require.Equal(t, 2*time.Minute, got)

	t.Setenv("DATA_REFRESH_INTERVAL", "1h")

	got = getRefreshInterval(conf)
	require.Equal(t, 1*time.Hour, got)
}

func TestDownloader_setupPeriodicRefreshing(t *testing.T) {
	ctx, cancelFunc := context.WithCancel(context.Background())
	logger := log.NewTestLogger()

	pkg, err := fshelp.FindPkgDir()
	require.NoError(t, err)

	conf := download.Config{
		InitialDataDirectory: filepath.Join(pkg, "ofac", "testdata"),
	}

	dl, err := download.NewDownloader(logger, conf, nil)
	require.NoError(t, err)

	indexedLists := index.NewLists(nil) // only in-memory

	go func() {
		time.Sleep(500 * time.Millisecond)
		cancelFunc()
	}()

	errs := make(chan error, 1)
	err = setupPeriodicRefreshing(ctx, logger, errs, conf, dl, indexedLists)
	require.NoError(t, err)

	cancelFunc()
	require.NoError(t, <-errs)
}

func TestSetupPeriodicRefreshing_TickerActuallyFires(t *testing.T) {
	// Use a very short refresh interval to test ticker behavior
	t.Setenv("DATA_REFRESH_INTERVAL", "50ms")

	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	logger := log.NewTestLogger()

	// Track RefreshAll calls with a mock
	var refreshCount atomic.Int32
	mockDl := &mockDownloader{
		refreshFn: func(ctx context.Context) (download.Stats, error) {
			refreshCount.Add(1)
			return download.Stats{
				Entities:   []search.Entity[search.Value]{},
				Lists:      map[string]int{},
				ListHashes: map[string]string{},
				StartedAt:  time.Now(),
				EndedAt:    time.Now(),
			}, nil
		},
	}

	mockIdx := &mockLists{}
	errs := make(chan error, 1)
	conf := download.Config{} // RefreshInterval overridden by env var

	err := setupPeriodicRefreshing(ctx, logger, errs, conf, mockDl, mockIdx)
	require.NoError(t, err)

	// Initial call happens immediately in refreshAllSources (line 22)
	// Then ticker should fire at 50ms, 100ms, etc.
	// Wait long enough for at least 2 ticker fires
	time.Sleep(150 * time.Millisecond)

	// Should have: 1 initial + at least 2 periodic = 3+
	count := refreshCount.Load()
	require.GreaterOrEqual(t, count, int32(3),
		"expected at least 3 refresh calls (1 initial + 2 periodic), got %d", count)

	// Clean shutdown
	cancelFunc()
	require.NoError(t, <-errs)
}

// mockDownloader implements download.Downloader for testing
type mockDownloader struct {
	refreshFn func(ctx context.Context) (download.Stats, error)
}

func (m *mockDownloader) RefreshAll(ctx context.Context) (download.Stats, error) {
	return m.refreshFn(ctx)
}

// mockLists implements index.Lists for testing
type mockLists struct {
	updateCount atomic.Int32
}

func (m *mockLists) GetEntities(ctx context.Context, source search.SourceList) ([]search.Entity[search.Value], error) {
	return nil, nil
}

func (m *mockLists) Update(latest download.Stats) {
	m.updateCount.Add(1)
}

func (m *mockLists) LatestStats() download.Stats {
	return download.Stats{}
}

func (m *mockLists) GetTFIDFIndex() *tfidf.Index {
	return nil
}
