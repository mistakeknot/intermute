package sqlite

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mistakeknot/intermute/internal/core"
)

// TestNewStoreAvoidsSQLITEBUSYUnderConcurrentWrites is the regression test
// for the wedge documented at COORDINATION.md (solwend) and reproduced
// live on 2026-05-11: a daemon would hit SQLITE_BUSY within seconds of
// startup when the sweeper goroutine and HTTP write handlers contended
// for the rollback-journal lock. Before the fix, sqlite.New left
// journal_mode=DELETE and no busy_timeout — any contended write returned
// BUSY immediately and could orphan the .db-journal file.
//
// This test simulates the daemon's actual concurrency pattern:
//   - 1 sweeper-like goroutine doing periodic deletes
//   - N HTTP-like goroutines doing AppendEvent + RegisterAgent
//
// Pre-fix expectation: SQLITE_BUSY errors appear within ~1s.
// Post-fix expectation: zero BUSY errors, all writes succeed.
func TestNewStoreAvoidsSQLITEBUSYUnderConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "wedge.db")

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	const (
		duration = 2 * time.Second
		writers  = 8
	)

	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var (
		wg       sync.WaitGroup
		errs     = make(chan error, 1024)
		writeOps int64
	)
	writeOpsMu := sync.Mutex{}
	bumpWrites := func() {
		writeOpsMu.Lock()
		writeOps++
		writeOpsMu.Unlock()
	}

	// Writers — simulate HTTP-handler-style write workload.
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for ctx.Err() == nil {
				_, err := store.AppendEvent(ctx, core.Event{
					Type:    core.EventMessageCreated,
					Project: "wedge-test",
					Message: core.Message{
						From: fmt.Sprintf("w%d", workerID),
						To:   []string{"inbox"},
						Body: "x",
					},
				})
				if err != nil && ctx.Err() == nil {
					errs <- fmt.Errorf("worker %d AppendEvent: %w", workerID, err)
					return
				}
				bumpWrites()
			}
		}(w)
	}

	// Sweeper-like goroutine — runs the same DELETE pattern the prod sweeper does.
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				_, err := store.SweepExpired(ctx, time.Now().UTC(), time.Now().UTC())
				if err != nil && ctx.Err() == nil {
					errs <- fmt.Errorf("sweep: %w", err)
					return
				}
			}
		}
	}()

	wg.Wait()
	close(errs)

	// Assert: zero SQLITE_BUSY errors. With the fix, busy_timeout=5000
	// lets contended writes wait rather than fail. Without the fix,
	// SQLITE_BUSY appears in the error stream almost immediately.
	var busyErrors []error
	var otherErrors []error
	for e := range errs {
		if strings.Contains(e.Error(), "SQLITE_BUSY") || strings.Contains(e.Error(), "database is locked") {
			busyErrors = append(busyErrors, e)
		} else {
			otherErrors = append(otherErrors, e)
		}
	}

	if len(busyErrors) > 0 {
		t.Errorf("expected zero SQLITE_BUSY errors after WAL+busy_timeout fix, got %d (first: %v)",
			len(busyErrors), busyErrors[0])
	}
	if len(otherErrors) > 0 {
		t.Errorf("unexpected non-BUSY errors: %d (first: %v)", len(otherErrors), otherErrors[0])
	}

	writeOpsMu.Lock()
	wops := writeOps
	writeOpsMu.Unlock()
	if wops < 100 {
		t.Errorf("expected hundreds of write ops in %v, got %d (writers likely all dead)", duration, wops)
	}
	t.Logf("ran %d write ops across %d concurrent writers + 1 sweeper for %v", wops, writers, duration)

	// Assert: no orphan rollback-journal file. WAL mode produces .db-wal
	// and .db-shm sidecars; if .db-journal exists, the DB fell back to
	// rollback-journal mode (the bug).
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".db-journal") {
			t.Errorf("rollback journal sidecar found (should be in WAL mode, not DELETE): %s", e.Name())
		}
	}
}

// TestNewStoreEnablesWALMode is a focused unit test: after New(), the DB
// reports journal_mode=wal. This is the prerequisite that prevents the
// wedge from being possible in the first place.
func TestNewStoreEnablesWALMode(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "wal.db")

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	var mode string
	row := store.db.QueryRow("PRAGMA journal_mode")
	if err := row.Scan(&mode); err != nil {
		t.Fatalf("PRAGMA journal_mode: %v", err)
	}
	if strings.ToLower(mode) != "wal" {
		t.Errorf("journal_mode = %q, want %q", mode, "wal")
	}
}

// TestStorePingSucceedsOnHealthyDB is the unit test for the new Store.Ping
// method that handleHealth uses to verify DB liveness.
func TestStorePingSucceedsOnHealthyDB(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "ping.db")

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err := store.Ping(ctx); err != nil {
		t.Errorf("Ping on fresh store: %v", err)
	}
}

// TestStorePingFailsAfterClose verifies Ping errors when the DB is closed —
// the failure mode handleHealth must surface as 503.
func TestStorePingFailsAfterClose(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "closed.db")

	store, err := New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_ = store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	if err := store.Ping(ctx); err == nil {
		t.Error("Ping after Close should error, got nil")
	}
}
