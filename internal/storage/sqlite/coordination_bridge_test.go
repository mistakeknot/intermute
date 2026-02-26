package sqlite

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/mistakeknot/intermute/internal/core"
)

// setupIntercoreDB creates a temp SQLite DB with the coordination_locks table.
func setupIntercoreDB(t *testing.T) (string, *sql.DB) {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "intercore.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open intercore db: %v", err)
	}
	db.SetMaxOpenConns(1)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS coordination_locks (
		id TEXT PRIMARY KEY,
		type TEXT NOT NULL CHECK(type IN ('file_reservation', 'named_lock', 'write_set')),
		owner TEXT NOT NULL,
		scope TEXT NOT NULL,
		pattern TEXT NOT NULL,
		exclusive INTEGER NOT NULL DEFAULT 1,
		reason TEXT,
		ttl_seconds INTEGER,
		created_at INTEGER NOT NULL,
		expires_at INTEGER,
		released_at INTEGER,
		dispatch_id TEXT,
		run_id TEXT
	)`)
	if err != nil {
		t.Fatalf("create coordination_locks: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return dbPath, db
}

func TestCoordinationBridge_Disabled(t *testing.T) {
	bridge, err := NewCoordinationBridge("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bridge.enabled {
		t.Fatal("expected disabled bridge")
	}
	// Should be no-ops.
	bridge.MirrorReserve("id1", "agent1", "/proj", "*.go", true, "test", 900, time.Now(), time.Now().Add(15*time.Minute))
	bridge.MirrorRelease("id1")
	if err := bridge.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestCoordinationBridge_MirrorReserve(t *testing.T) {
	dbPath, icDB := setupIntercoreDB(t)

	bridge, err := NewCoordinationBridge(dbPath)
	if err != nil {
		t.Fatalf("bridge open: %v", err)
	}
	defer bridge.Close()

	now := time.Now().UTC()
	expires := now.Add(15 * time.Minute)
	bridge.MirrorReserve("res-1", "agent-a", "/home/mk/projects/Demarch", "*.go", true, "editing", 900, now, expires)

	// Verify the row exists in coordination_locks.
	var id, lockType, owner, scope, pattern string
	var exclusive int
	err = icDB.QueryRow("SELECT id, type, owner, scope, pattern, exclusive FROM coordination_locks WHERE id = ?", "res-1").
		Scan(&id, &lockType, &owner, &scope, &pattern, &exclusive)
	if err != nil {
		t.Fatalf("query mirrored lock: %v", err)
	}
	if lockType != "file_reservation" {
		t.Errorf("type = %q, want file_reservation", lockType)
	}
	if owner != "agent-a" {
		t.Errorf("owner = %q, want agent-a", owner)
	}
	if scope != "/home/mk/projects/Demarch" {
		t.Errorf("scope = %q, want /home/mk/projects/Demarch", scope)
	}
	if pattern != "*.go" {
		t.Errorf("pattern = %q, want *.go", pattern)
	}
	if exclusive != 1 {
		t.Errorf("exclusive = %d, want 1", exclusive)
	}
}

func TestCoordinationBridge_MirrorRelease(t *testing.T) {
	dbPath, icDB := setupIntercoreDB(t)

	bridge, err := NewCoordinationBridge(dbPath)
	if err != nil {
		t.Fatalf("bridge open: %v", err)
	}
	defer bridge.Close()

	now := time.Now().UTC()
	bridge.MirrorReserve("res-2", "agent-b", "/proj", "main.go", true, "test", 900, now, now.Add(15*time.Minute))
	bridge.MirrorRelease("res-2")

	var releasedAt sql.NullInt64
	err = icDB.QueryRow("SELECT released_at FROM coordination_locks WHERE id = ?", "res-2").Scan(&releasedAt)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if !releasedAt.Valid {
		t.Fatal("expected released_at to be set")
	}
}

func TestCoordinationBridge_DualWriteFromStore(t *testing.T) {
	dbPath, icDB := setupIntercoreDB(t)

	bridge, err := NewCoordinationBridge(dbPath)
	if err != nil {
		t.Fatalf("bridge open: %v", err)
	}
	defer bridge.Close()

	store, err := NewInMemory()
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	store.SetCoordinationBridge(bridge)

	ctx := context.Background()

	// Register an agent so the store works.
	store.RegisterAgent(ctx, core.Agent{ID: "agent-1", Name: "test-agent"})

	// Reserve via the store — should dual-write.
	res, err := store.Reserve(ctx, core.Reservation{
		AgentID:     "agent-1",
		Project:     "/home/mk/projects/Demarch",
		PathPattern: "internal/*.go",
		Exclusive:   true,
		Reason:      "task-7-test",
		TTL:         15 * time.Minute,
	})
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}

	// Check the mirror.
	var count int
	err = icDB.QueryRow("SELECT COUNT(*) FROM coordination_locks WHERE id = ?", res.ID).Scan(&count)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Errorf("mirror count = %d, want 1", count)
	}

	// Release via the store — should mirror release.
	err = store.ReleaseReservation(ctx, res.ID, "agent-1")
	if err != nil {
		t.Fatalf("release: %v", err)
	}

	var releasedAt sql.NullInt64
	err = icDB.QueryRow("SELECT released_at FROM coordination_locks WHERE id = ?", res.ID).Scan(&releasedAt)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if !releasedAt.Valid {
		t.Fatal("expected released_at to be set after release")
	}
}

func TestCoordinationBridge_ErrorsDoNotFailPrimary(t *testing.T) {
	// Create a bridge pointing at a DB with no coordination_locks table — should fail to open.
	dir := t.TempDir()
	badDBPath := filepath.Join(dir, "bad.db")
	db, _ := sql.Open("sqlite", badDBPath)
	db.Exec("CREATE TABLE dummy (id TEXT)")
	db.Close()

	_, err := NewCoordinationBridge(badDBPath)
	if err == nil {
		t.Fatal("expected error for missing table")
	}
}

func TestDiscoverIntercoreDB(t *testing.T) {
	dir := t.TempDir()
	clavainDir := filepath.Join(dir, ".clavain")
	os.MkdirAll(clavainDir, 0755)
	dbPath := filepath.Join(clavainDir, "intercore.db")
	os.WriteFile(dbPath, []byte{}, 0644)

	// Discovery from a subdirectory should find it.
	subDir := filepath.Join(dir, "src", "pkg")
	os.MkdirAll(subDir, 0755)

	found := DiscoverIntercoreDB(subDir)
	if found != dbPath {
		t.Errorf("found = %q, want %q", found, dbPath)
	}

	// Discovery from unrelated dir should return empty.
	otherDir := t.TempDir()
	found = DiscoverIntercoreDB(otherDir)
	if found != "" {
		t.Errorf("expected empty, got %q", found)
	}
}
