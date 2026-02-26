package sqlite

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// CoordinationBridge mirrors reservations to Intercore's coordination_locks table.
// Used during the dual-write migration phase — Intermute writes to both
// file_reservations (primary) and coordination_locks (mirror).
type CoordinationBridge struct {
	db      *sql.DB
	enabled bool
}

// NewCoordinationBridge opens the Intercore DB for dual-write.
// If dbPath is empty, the bridge is disabled (no-op).
func NewCoordinationBridge(dbPath string) (*CoordinationBridge, error) {
	if dbPath == "" {
		return &CoordinationBridge{enabled: false}, nil
	}
	db, err := sql.Open("sqlite", "file:"+dbPath+"?_pragma=journal_mode%3DWAL&_pragma=busy_timeout%3D5000")
	if err != nil {
		return nil, fmt.Errorf("coordination bridge open: %w", err)
	}
	db.SetMaxOpenConns(1)

	// Verify the coordination_locks table exists.
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM coordination_locks LIMIT 1").Scan(&count); err != nil {
		db.Close()
		return nil, fmt.Errorf("coordination bridge: coordination_locks table not found: %w", err)
	}

	return &CoordinationBridge{db: db, enabled: true}, nil
}

// normalizeScope converts Intermute's project identifier to the canonical
// absolute path that Intercore uses for scope. Mismatched scopes cause false
// negatives in cross-system conflict detection.
//
// Only uses filepath.Abs — NOT git rev-parse, which would resolve relative to
// Intermute's CWD rather than the project's, silently breaking scope matching.
func normalizeScope(project string) string {
	if filepath.IsAbs(project) {
		return filepath.Clean(project)
	}
	abs, err := filepath.Abs(project)
	if err != nil {
		return project
	}
	return abs
}

// MirrorReserve writes a reservation to coordination_locks.
// Errors are logged but never returned — the bridge must not fail the primary operation.
func (b *CoordinationBridge) MirrorReserve(id, agentID, project, pattern string, exclusive bool, reason string, ttlSeconds int, createdAt time.Time, expiresAt time.Time) {
	if !b.enabled {
		return
	}
	project = normalizeScope(project)
	exclInt := 0
	if exclusive {
		exclInt = 1
	}
	var expiresAtUnix *int64
	if !expiresAt.IsZero() {
		v := expiresAt.Unix()
		expiresAtUnix = &v
	}
	_, err := b.db.Exec(`INSERT OR IGNORE INTO coordination_locks
		(id, type, owner, scope, pattern, exclusive, reason, ttl_seconds, created_at, expires_at)
		VALUES (?, 'file_reservation', ?, ?, ?, ?, ?, ?, ?, ?)`,
		id, agentID, project, pattern, exclInt, reason, ttlSeconds, createdAt.Unix(), expiresAtUnix)
	if err != nil {
		log.Printf("coordination bridge: mirror reserve %s: %v", id, err)
	}
}

// MirrorRelease marks a lock as released in coordination_locks.
func (b *CoordinationBridge) MirrorRelease(id string) {
	if !b.enabled {
		return
	}
	_, err := b.db.Exec(`UPDATE coordination_locks SET released_at = ? WHERE id = ? AND released_at IS NULL`,
		time.Now().Unix(), id)
	if err != nil {
		log.Printf("coordination bridge: mirror release %s: %v", id, err)
	}
}

// Close closes the bridge database connection.
func (b *CoordinationBridge) Close() error {
	if b.db != nil {
		return b.db.Close()
	}
	return nil
}

// DiscoverIntercoreDB walks up from projectDir looking for .clavain/intercore.db.
func DiscoverIntercoreDB(projectDir string) string {
	if projectDir == "" {
		var err error
		projectDir, err = os.Getwd()
		if err != nil {
			return ""
		}
	}
	dir := projectDir
	for {
		candidate := filepath.Join(dir, ".clavain", "intercore.db")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
