package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/mistakeknot/intermute/internal/core"
	_ "modernc.org/sqlite"
)

// newRaceStore creates a file-backed SQLite store with WAL mode and busy
// timeout, suitable for concurrent access from multiple goroutines.
// In-memory ":memory:" doesn't work because each connection gets a separate DB.
func newRaceStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "race.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	// SQLite is single-writer; limit to 1 connection to avoid SQLITE_BUSY.
	// This also ensures PRAGMAs apply to the same connection.
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		t.Fatalf("wal mode: %v", err)
	}
	if err := applySchema(db); err != nil {
		t.Fatalf("schema: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		os.RemoveAll(dir)
	})
	return &Store{db: &queryLogger{inner: db}}
}

// TestConcurrentAppendEvent verifies that concurrent event appends don't race.
// 10 goroutines each append 10 messages; all 100 should be delivered.
func TestConcurrentAppendEvent(t *testing.T) {
	st := newRaceStore(t)
	ctx := context.Background()
	const workers = 10
	const msgsPerWorker = 10

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < msgsPerWorker; j++ {
				_, err := st.AppendEvent(ctx, core.Event{
					Type:    core.EventMessageCreated,
					Project: "race-proj",
					Message: core.Message{
						From: fmt.Sprintf("worker-%d", workerID),
						To:   []string{"inbox-agent"},
						Body: fmt.Sprintf("msg-%d-%d", workerID, j),
					},
				})
				if err != nil {
					t.Errorf("worker %d msg %d: %v", workerID, j, err)
				}
			}
		}(i)
	}
	wg.Wait()

	// Verify all 100 messages are in the inbox
	msgs, err := st.InboxSince(ctx, "race-proj", "inbox-agent", 0, 0)
	if err != nil {
		t.Fatalf("inbox: %v", err)
	}
	if len(msgs) != workers*msgsPerWorker {
		t.Fatalf("expected %d messages, got %d", workers*msgsPerWorker, len(msgs))
	}
}

// TestConcurrentReservation verifies that overlapping exclusive reservations
// are serialized correctly — exactly 1 of 5 concurrent attempts should win.
func TestConcurrentReservation(t *testing.T) {
	st := newRaceStore(t)
	const workers = 5

	var (
		wg       sync.WaitGroup
		wins     atomic.Int32
		failures atomic.Int32
	)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, err := st.Reserve(context.Background(), core.Reservation{
				AgentID:     fmt.Sprintf("agent-%d", id),
				Project:     "race-proj",
				PathPattern: "shared/file.go",
				Exclusive:   true,
				Reason:      fmt.Sprintf("worker %d", id),
			})
			if err != nil {
				failures.Add(1)
			} else {
				wins.Add(1)
			}
		}(i)
	}
	wg.Wait()

	if wins.Load() != 1 {
		t.Fatalf("expected exactly 1 reservation win, got %d wins and %d failures", wins.Load(), failures.Load())
	}
	if failures.Load() != int32(workers-1) {
		t.Fatalf("expected %d failures, got %d", workers-1, failures.Load())
	}
}

// TestConcurrentOptimisticLock verifies that concurrent spec updates
// respect version checking — only 1 of 5 should succeed per round.
func TestConcurrentOptimisticLock(t *testing.T) {
	st := newRaceStore(t)
	ctx := context.Background()

	// Create a spec
	spec, err := st.CreateSpec(ctx, core.Spec{
		Project: "race-proj",
		Title:   "Race Spec",
		Status:  core.SpecStatusDraft,
	})
	if err != nil {
		t.Fatalf("create spec: %v", err)
	}

	const workers = 5
	var (
		wg       sync.WaitGroup
		wins     atomic.Int32
		failures atomic.Int32
	)

	// All workers try to update with the same (current) version
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, err := st.UpdateSpec(ctx, core.Spec{
				ID:      spec.ID,
				Project: "race-proj",
				Title:   fmt.Sprintf("Updated by worker %d", id),
				Status:  core.SpecStatusResearch,
				Version: spec.Version,
			})
			if err != nil {
				failures.Add(1)
			} else {
				wins.Add(1)
			}
		}(i)
	}
	wg.Wait()

	if wins.Load() != 1 {
		t.Fatalf("expected exactly 1 optimistic lock win, got %d wins and %d failures", wins.Load(), failures.Load())
	}
}

// TestConcurrentInboxReads verifies that reading inbox while messages are
// being written doesn't cause data races.
func TestConcurrentInboxReads(t *testing.T) {
	st := newRaceStore(t)
	ctx := context.Background()

	const writers = 1
	const readers = 3
	const msgsToWrite = 20

	// Start writer
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < msgsToWrite; i++ {
			_, err := st.AppendEvent(ctx, core.Event{
				Type:    core.EventMessageCreated,
				Project: "race-proj",
				Message: core.Message{
					From: "writer",
					To:   []string{"reader-agent"},
					Body: fmt.Sprintf("msg-%d", i),
				},
			})
			if err != nil {
				t.Errorf("write %d: %v", i, err)
			}
		}
	}()

	// Start readers (concurrent with writer)
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			for i := 0; i < msgsToWrite; i++ {
				msgs, err := st.InboxSince(ctx, "race-proj", "reader-agent", 0, 0)
				if err != nil {
					t.Errorf("reader %d iteration %d: %v", readerID, i, err)
					return
				}
				// Just verify no panic and valid data
				_ = len(msgs)
			}
		}(r)
	}

	wg.Wait()

	// Final check: all messages should be visible
	msgs, err := st.InboxSince(ctx, "race-proj", "reader-agent", 0, 0)
	if err != nil {
		t.Fatalf("final inbox: %v", err)
	}
	if len(msgs) != msgsToWrite {
		t.Fatalf("expected %d messages, got %d", msgsToWrite, len(msgs))
	}
}
