package sqlite

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/google/uuid"
	"github.com/mistakeknot/intermute/internal/core"
	"github.com/mistakeknot/intermute/internal/storage"
)

//go:embed schema.sql
var schema string

type Store struct {
	db *sql.DB
}

func New(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("db path required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := applySchema(db); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func NewInMemory() (*Store, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := applySchema(db); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

func applySchema(db *sql.DB) error {
	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	if err := migrateMessages(db); err != nil {
		return err
	}
	if err := migrateInboxIndex(db); err != nil {
		return err
	}
	if err := migrateThreadIndex(db); err != nil {
		return err
	}
	if err := migrateMessagesMetadata(db); err != nil {
		return err
	}
	if err := migrateDomainVersions(db); err != nil {
		return err
	}
	return nil
}

func (s *Store) AppendEvent(ev core.Event) (uint64, error) {
	if ev.ID == "" {
		ev.ID = uuid.NewString()
	}
	if ev.CreatedAt.IsZero() {
		ev.CreatedAt = time.Now().UTC()
	}
	project := strings.TrimSpace(ev.Project)
	if project == "" {
		project = strings.TrimSpace(ev.Message.Project)
	}
	if ev.Message.CreatedAt.IsZero() {
		ev.Message.CreatedAt = ev.CreatedAt
	}
	ev.Message.Project = project

	toJSON, _ := json.Marshal(ev.Message.To)

	res, err := s.db.Exec(
		`INSERT INTO events (id, type, agent, project, message_id, thread_id, from_agent, to_json, body, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ev.ID, string(ev.Type), ev.Agent, project, ev.Message.ID, ev.Message.ThreadID, ev.Message.From, string(toJSON), ev.Message.Body, ev.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return 0, fmt.Errorf("insert event: %w", err)
	}
	cursor, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("cursor: %w", err)
	}

	if ev.Type == core.EventMessageCreated {
		if err := s.upsertMessage(project, ev.Message); err != nil {
			return 0, err
		}
		recipients := ev.Message.To
		if len(recipients) == 0 && ev.Agent != "" {
			recipients = []string{ev.Agent}
		}
		for _, agent := range recipients {
			if _, err := s.db.Exec(
				`INSERT INTO inbox_index (project, agent, cursor, message_id) VALUES (?, ?, ?, ?)`,
				project, agent, cursor, ev.Message.ID,
			); err != nil {
				return 0, fmt.Errorf("insert inbox: %w", err)
			}
		}
		// Insert into message_recipients for per-recipient tracking
		if err := s.insertRecipients(project, ev.Message.ID, ev.Message.To, "to"); err != nil {
			return 0, err
		}
		if err := s.insertRecipients(project, ev.Message.ID, ev.Message.CC, "cc"); err != nil {
			return 0, err
		}
		if err := s.insertRecipients(project, ev.Message.ID, ev.Message.BCC, "bcc"); err != nil {
			return 0, err
		}
		// Update thread_index if message has a thread ID
		if ev.Message.ThreadID != "" {
			participants := append([]string{ev.Message.From}, recipients...)
			for _, agent := range participants {
				if _, err := s.db.Exec(
					`INSERT INTO thread_index (project, thread_id, agent, last_cursor, message_count)
					 VALUES (?, ?, ?, ?, 1)
					 ON CONFLICT(project, thread_id, agent) DO UPDATE SET
					   last_cursor = excluded.last_cursor,
					   message_count = thread_index.message_count + 1`,
					project, ev.Message.ThreadID, agent, cursor,
				); err != nil {
					return 0, fmt.Errorf("upsert thread_index: %w", err)
				}
			}
		}
	}

	return uint64(cursor), nil
}

func (s *Store) upsertMessage(project string, msg core.Message) error {
	if project == "" {
		project = msg.Project
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now().UTC()
	}
	toJSON, _ := json.Marshal(msg.To)
	ccJSON, _ := json.Marshal(msg.CC)
	bccJSON, _ := json.Marshal(msg.BCC)
	ackRequired := 0
	if msg.AckRequired {
		ackRequired = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO messages (project, message_id, thread_id, from_agent, to_json, cc_json, bcc_json, subject, body, importance, ack_required, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(project, message_id) DO UPDATE SET thread_id=excluded.thread_id, from_agent=excluded.from_agent, to_json=excluded.to_json, cc_json=excluded.cc_json, bcc_json=excluded.bcc_json, subject=excluded.subject, body=excluded.body, importance=excluded.importance, ack_required=excluded.ack_required`,
		project, msg.ID, msg.ThreadID, msg.From, string(toJSON), string(ccJSON), string(bccJSON), msg.Subject, msg.Body, msg.Importance, ackRequired, msg.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert message: %w", err)
	}
	return nil
}

func (s *Store) InboxSince(project, agent string, cursor uint64) ([]core.Message, error) {
	query := `SELECT i.cursor, i.project, m.message_id, m.thread_id, m.from_agent, m.to_json,
		COALESCE(m.cc_json, '[]'), COALESCE(m.bcc_json, '[]'), COALESCE(m.subject, ''),
		m.body, COALESCE(m.importance, ''), COALESCE(m.ack_required, 0), m.created_at
	 FROM inbox_index i
	 JOIN messages m ON m.project = i.project AND m.message_id = i.message_id
	 WHERE i.agent = ? AND i.cursor > ?`
	args := []any{agent, cursor}
	if project != "" {
		query += " AND i.project = ?"
		args = append(args, project)
	}
	query += " ORDER BY i.cursor ASC"
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query inbox: %w", err)
	}
	defer rows.Close()

	var out []core.Message
	for rows.Next() {
		var (
			cur                                                                            int64
			proj                                                                           string
			msgID, threadID, fromAgent, toJSON, ccJSON, bccJSON, subject, body, importance string
			ackRequired                                                                    int
			createdAt                                                                      string
		)
		if err := rows.Scan(&cur, &proj, &msgID, &threadID, &fromAgent, &toJSON, &ccJSON, &bccJSON, &subject, &body, &importance, &ackRequired, &createdAt); err != nil {
			return nil, fmt.Errorf("scan inbox: %w", err)
		}
		var to, cc, bcc []string
		_ = json.Unmarshal([]byte(toJSON), &to)
		_ = json.Unmarshal([]byte(ccJSON), &cc)
		_ = json.Unmarshal([]byte(bccJSON), &bcc)
		parsed, _ := time.Parse(time.RFC3339Nano, createdAt)
		out = append(out, core.Message{
			ID:          msgID,
			ThreadID:    threadID,
			Project:     proj,
			From:        fromAgent,
			To:          to,
			CC:          cc,
			BCC:         bcc,
			Subject:     subject,
			Body:        body,
			Importance:  importance,
			AckRequired: ackRequired == 1,
			CreatedAt:   parsed,
			Cursor:      uint64(cur),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return out, nil
}

func (s *Store) ThreadMessages(project, threadID string, cursor uint64) ([]core.Message, error) {
	query := `SELECT i.cursor, m.project, m.message_id, m.thread_id, m.from_agent, m.to_json,
		COALESCE(m.cc_json, '[]'), COALESCE(m.bcc_json, '[]'), COALESCE(m.subject, ''),
		m.body, COALESCE(m.importance, ''), COALESCE(m.ack_required, 0), m.created_at
	 FROM inbox_index i
	 JOIN messages m ON m.project = i.project AND m.message_id = i.message_id
	 WHERE m.project = ? AND m.thread_id = ? AND i.cursor > ?
	 GROUP BY m.message_id
	 ORDER BY m.created_at ASC`
	rows, err := s.db.Query(query, project, threadID, cursor)
	if err != nil {
		return nil, fmt.Errorf("query thread: %w", err)
	}
	defer rows.Close()

	var out []core.Message
	for rows.Next() {
		var (
			cur                                                                        int64
			proj                                                                       string
			msgID, thID, fromAgent, toJSON, ccJSON, bccJSON, subject, body, importance string
			ackRequired                                                                int
			createdAt                                                                  string
		)
		if err := rows.Scan(&cur, &proj, &msgID, &thID, &fromAgent, &toJSON, &ccJSON, &bccJSON, &subject, &body, &importance, &ackRequired, &createdAt); err != nil {
			return nil, fmt.Errorf("scan thread: %w", err)
		}
		var to, cc, bcc []string
		_ = json.Unmarshal([]byte(toJSON), &to)
		_ = json.Unmarshal([]byte(ccJSON), &cc)
		_ = json.Unmarshal([]byte(bccJSON), &bcc)
		parsed, _ := time.Parse(time.RFC3339Nano, createdAt)
		out = append(out, core.Message{
			ID:          msgID,
			ThreadID:    thID,
			Project:     proj,
			From:        fromAgent,
			To:          to,
			CC:          cc,
			BCC:         bcc,
			Subject:     subject,
			Body:        body,
			Importance:  importance,
			AckRequired: ackRequired == 1,
			CreatedAt:   parsed,
			Cursor:      uint64(cur),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return out, nil
}

func (s *Store) ListThreads(project, agent string, cursor uint64, limit int) ([]storage.ThreadSummary, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `SELECT t.thread_id, t.last_cursor, t.message_count, m.from_agent, m.body, m.created_at
	 FROM thread_index t
	 JOIN messages m ON m.project = t.project AND m.thread_id = t.thread_id
	 WHERE t.project = ? AND t.agent = ?`
	args := []any{project, agent}
	// With DESC ordering, cursor represents an upper bound for older pages.
	if cursor > 0 {
		query += " AND t.last_cursor < ?"
		args = append(args, cursor)
	}
	query += `
	 AND m.created_at = (
	   SELECT MAX(m2.created_at) FROM messages m2
	   WHERE m2.project = t.project AND m2.thread_id = t.thread_id
	 )
	 ORDER BY t.last_cursor DESC
	 LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query threads: %w", err)
	}
	defer rows.Close()

	var out []storage.ThreadSummary
	for rows.Next() {
		var (
			threadID, lastFrom, lastBody, lastAt string
			lastCursor                           int64
			messageCount                         int
		)
		if err := rows.Scan(&threadID, &lastCursor, &messageCount, &lastFrom, &lastBody, &lastAt); err != nil {
			return nil, fmt.Errorf("scan thread: %w", err)
		}
		parsed, _ := time.Parse(time.RFC3339Nano, lastAt)
		out = append(out, storage.ThreadSummary{
			ThreadID:     threadID,
			LastCursor:   uint64(lastCursor),
			MessageCount: messageCount,
			LastFrom:     lastFrom,
			LastBody:     lastBody,
			LastAt:       parsed,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return out, nil
}

func migrateMessages(db *sql.DB) error {
	if !tableExists(db, "messages") {
		return nil
	}
	hasProject := tableHasColumn(db, "messages", "project")
	if hasProject && tableHasCompositePK(db, "messages") {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin migrate messages: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`ALTER TABLE messages RENAME TO messages_old`); err != nil {
		return fmt.Errorf("rename messages: %w", err)
	}
	if _, err := tx.Exec(`CREATE TABLE messages (
		project TEXT NOT NULL DEFAULT '',
		message_id TEXT NOT NULL,
		thread_id TEXT,
		from_agent TEXT,
		to_json TEXT,
		body TEXT,
		created_at TEXT NOT NULL,
		PRIMARY KEY (project, message_id)
	)`); err != nil {
		return fmt.Errorf("create messages: %w", err)
	}
	projectExpr := "''"
	if hasProject {
		projectExpr = "COALESCE(project, '')"
	}
	insertSQL := fmt.Sprintf(`INSERT INTO messages (project, message_id, thread_id, from_agent, to_json, body, created_at)
		SELECT %s, message_id, thread_id, from_agent, to_json, body, created_at FROM messages_old`, projectExpr)
	if _, err := tx.Exec(insertSQL); err != nil {
		return fmt.Errorf("copy messages: %w", err)
	}
	if _, err := tx.Exec(`DROP TABLE messages_old`); err != nil {
		return fmt.Errorf("drop messages_old: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migrate messages: %w", err)
	}
	return nil
}

func migrateInboxIndex(db *sql.DB) error {
	if !tableExists(db, "inbox_index") {
		return nil
	}
	if tableHasColumn(db, "inbox_index", "project") {
		_, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_inbox_agent_cursor ON inbox_index(project, agent, cursor)`)
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin migrate inbox: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`ALTER TABLE inbox_index RENAME TO inbox_index_old`); err != nil {
		return fmt.Errorf("rename inbox_index: %w", err)
	}
	if _, err := tx.Exec(`CREATE TABLE inbox_index (
		project TEXT NOT NULL DEFAULT '',
		agent TEXT NOT NULL,
		cursor INTEGER NOT NULL,
		message_id TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("create inbox_index: %w", err)
	}
	if _, err := tx.Exec(`INSERT INTO inbox_index (project, agent, cursor, message_id)
		SELECT '', agent, cursor, message_id FROM inbox_index_old`); err != nil {
		return fmt.Errorf("copy inbox_index: %w", err)
	}
	if _, err := tx.Exec(`DROP TABLE inbox_index_old`); err != nil {
		return fmt.Errorf("drop inbox_index_old: %w", err)
	}
	if _, err := tx.Exec(`CREATE INDEX idx_inbox_agent_cursor ON inbox_index(project, agent, cursor)`); err != nil {
		return fmt.Errorf("index inbox_index: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migrate inbox: %w", err)
	}
	return nil
}

func migrateThreadIndex(db *sql.DB) error {
	// Backfill thread_index from existing messages with thread_id
	if !tableExists(db, "thread_index") {
		return nil
	}
	// Check if there are already entries - if so, skip backfill
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM thread_index`).Scan(&count); err != nil {
		return fmt.Errorf("count thread_index: %w", err)
	}
	if count > 0 {
		return nil
	}
	// Backfill from messages that have thread_id
	_, err := db.Exec(`
		WITH participants AS (
			SELECT m.project, m.thread_id, i.agent AS agent, i.cursor AS cursor, m.message_id
			FROM messages m
			JOIN inbox_index i ON i.project = m.project AND i.message_id = m.message_id
			WHERE m.thread_id IS NOT NULL AND m.thread_id != ''
			UNION ALL
			SELECT m.project, m.thread_id, m.from_agent AS agent, e.cursor AS cursor, m.message_id
			FROM messages m
			JOIN events e ON e.project = m.project AND e.message_id = m.message_id AND e.type = ?
			WHERE m.thread_id IS NOT NULL AND m.thread_id != '' AND m.from_agent IS NOT NULL AND m.from_agent != ''
		)
		INSERT INTO thread_index (project, thread_id, agent, last_cursor, message_count)
		SELECT project, thread_id, agent, MAX(cursor), COUNT(DISTINCT message_id)
		FROM participants
		GROUP BY project, thread_id, agent
	`, string(core.EventMessageCreated))
	if err != nil {
		return fmt.Errorf("backfill thread_index: %w", err)
	}
	return nil
}

func tableExists(db *sql.DB, table string) bool {
	row := db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table)
	var name string
	return row.Scan(&name) == nil
}

func tableHasColumn(db *sql.DB, table, column string) bool {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false
	}
	defer rows.Close()
	for rows.Next() {
		var (
			cid       int
			name      string
			ctype     string
			notnull   int
			dfltValue any
			pk        int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return false
		}
		if name == column {
			return true
		}
	}
	return false
}

func tableHasCompositePK(db *sql.DB, table string) bool {
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		return false
	}
	defer rows.Close()
	var hasProjectPK, hasMessagePK bool
	for rows.Next() {
		var (
			cid       int
			name      string
			ctype     string
			notnull   int
			dfltValue any
			pk        int
		)
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return false
		}
		if name == "project" && pk > 0 {
			hasProjectPK = true
		}
		if name == "message_id" && pk > 0 {
			hasMessagePK = true
		}
	}
	return hasProjectPK && hasMessagePK
}

func (s *Store) RegisterAgent(agent core.Agent) (core.Agent, error) {
	if agent.ID == "" {
		agent.ID = uuid.NewString()
	}
	if agent.SessionID == "" {
		agent.SessionID = uuid.NewString()
	}
	now := time.Now().UTC()
	if agent.CreatedAt.IsZero() {
		agent.CreatedAt = now
	}
	if agent.LastSeen.IsZero() {
		agent.LastSeen = now
	}

	capsJSON, _ := json.Marshal(agent.Capabilities)
	metaJSON, _ := json.Marshal(agent.Metadata)

	_, err := s.db.Exec(
		`INSERT INTO agents (id, session_id, name, project, capabilities_json, metadata_json, status, created_at, last_seen)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET session_id=excluded.session_id, name=excluded.name, project=excluded.project,
		 capabilities_json=excluded.capabilities_json, metadata_json=excluded.metadata_json, status=excluded.status, last_seen=excluded.last_seen`,
		agent.ID, agent.SessionID, agent.Name, agent.Project, string(capsJSON), string(metaJSON), agent.Status,
		agent.CreatedAt.Format(time.RFC3339Nano), agent.LastSeen.Format(time.RFC3339Nano),
	)
	if err != nil {
		return core.Agent{}, fmt.Errorf("register agent: %w", err)
	}
	return agent, nil
}

func (s *Store) Heartbeat(project, agentID string) (core.Agent, error) {
	now := time.Now().UTC()
	var query string
	var args []any
	if project != "" {
		query = `UPDATE agents SET last_seen=? WHERE id=? AND project=?`
		args = []any{now.Format(time.RFC3339Nano), agentID, project}
	} else {
		query = `UPDATE agents SET last_seen=? WHERE id=?`
		args = []any{now.Format(time.RFC3339Nano), agentID}
	}
	res, err := s.db.Exec(query, args...)
	if err != nil {
		return core.Agent{}, fmt.Errorf("heartbeat: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return core.Agent{}, fmt.Errorf("agent not found")
	}

	row := s.db.QueryRow(`SELECT id, session_id, name, project, capabilities_json, metadata_json, status, created_at, last_seen FROM agents WHERE id=?`, agentID)
	var (
		id, sessionID, name, proj, capsJSON, metaJSON, status, createdAt, lastSeen string
	)
	if err := row.Scan(&id, &sessionID, &name, &proj, &capsJSON, &metaJSON, &status, &createdAt, &lastSeen); err != nil {
		return core.Agent{}, fmt.Errorf("heartbeat fetch: %w", err)
	}
	var caps []string
	_ = json.Unmarshal([]byte(capsJSON), &caps)
	meta := map[string]string{}
	_ = json.Unmarshal([]byte(metaJSON), &meta)
	createdAtTime, _ := time.Parse(time.RFC3339Nano, createdAt)
	lastSeenTime, _ := time.Parse(time.RFC3339Nano, lastSeen)

	return core.Agent{
		ID:           id,
		SessionID:    sessionID,
		Name:         name,
		Project:      proj,
		Capabilities: caps,
		Metadata:     meta,
		Status:       status,
		CreatedAt:    createdAtTime,
		LastSeen:     lastSeenTime,
	}, nil
}

func (s *Store) ListAgents(project string) ([]core.Agent, error) {
	query := `SELECT id, session_id, name, project, capabilities_json, metadata_json, status, created_at, last_seen
		FROM agents`
	var args []any
	if project != "" {
		query += " WHERE project = ?"
		args = append(args, project)
	}
	query += " ORDER BY last_seen DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query agents: %w", err)
	}
	defer rows.Close()

	var out []core.Agent
	for rows.Next() {
		var (
			id, sessionID, name, proj, capsJSON, metaJSON, status, createdAt, lastSeen string
		)
		if err := rows.Scan(&id, &sessionID, &name, &proj, &capsJSON, &metaJSON, &status, &createdAt, &lastSeen); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		var caps []string
		_ = json.Unmarshal([]byte(capsJSON), &caps)
		meta := map[string]string{}
		_ = json.Unmarshal([]byte(metaJSON), &meta)
		createdAtTime, _ := time.Parse(time.RFC3339Nano, createdAt)
		lastSeenTime, _ := time.Parse(time.RFC3339Nano, lastSeen)

		out = append(out, core.Agent{
			ID:           id,
			SessionID:    sessionID,
			Name:         name,
			Project:      proj,
			Capabilities: caps,
			Metadata:     meta,
			Status:       status,
			CreatedAt:    createdAtTime,
			LastSeen:     lastSeenTime,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return out, nil
}

// migrateMessagesMetadata adds cc_json, bcc_json, subject, importance, ack_required columns to messages table
func migrateMessagesMetadata(db *sql.DB) error {
	if !tableExists(db, "messages") {
		return nil
	}
	// Check if new columns already exist
	if tableHasColumn(db, "messages", "subject") {
		return nil
	}
	// Add new columns one at a time
	cols := []struct {
		name string
		def  string
	}{
		{"cc_json", "TEXT"},
		{"bcc_json", "TEXT"},
		{"subject", "TEXT"},
		{"importance", "TEXT"},
		{"ack_required", "INTEGER NOT NULL DEFAULT 0"},
	}
	for _, col := range cols {
		if !tableHasColumn(db, "messages", col.name) {
			if _, err := db.Exec(fmt.Sprintf("ALTER TABLE messages ADD COLUMN %s %s", col.name, col.def)); err != nil {
				return fmt.Errorf("add column %s: %w", col.name, err)
			}
		}
	}
	return nil
}

// insertRecipients adds recipients to the message_recipients table
func (s *Store) insertRecipients(project, messageID string, agents []string, kind string) error {
	for _, agent := range agents {
		if _, err := s.db.Exec(
			`INSERT INTO message_recipients (project, message_id, agent_id, kind)
			 VALUES (?, ?, ?, ?)
			 ON CONFLICT(project, message_id, agent_id) DO NOTHING`,
			project, messageID, agent, kind,
		); err != nil {
			return fmt.Errorf("insert recipient %s: %w", agent, err)
		}
	}
	return nil
}

// MarkRead marks a message as read by a specific recipient
func (s *Store) MarkRead(project, messageID, agentID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(
		`UPDATE message_recipients SET read_at = ? WHERE project = ? AND message_id = ? AND agent_id = ? AND read_at IS NULL`,
		now, project, messageID, agentID,
	)
	if err != nil {
		return fmt.Errorf("mark read: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		// Either already read or not a recipient - check if recipient exists
		var exists int
		s.db.QueryRow(`SELECT 1 FROM message_recipients WHERE project = ? AND message_id = ? AND agent_id = ?`,
			project, messageID, agentID).Scan(&exists)
		if exists == 0 {
			return fmt.Errorf("agent %s is not a recipient of message %s", agentID, messageID)
		}
	}
	return nil
}

// MarkAck marks a message as acknowledged by a specific recipient
func (s *Store) MarkAck(project, messageID, agentID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(
		`UPDATE message_recipients SET ack_at = ? WHERE project = ? AND message_id = ? AND agent_id = ? AND ack_at IS NULL`,
		now, project, messageID, agentID,
	)
	if err != nil {
		return fmt.Errorf("mark ack: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		var exists int
		s.db.QueryRow(`SELECT 1 FROM message_recipients WHERE project = ? AND message_id = ? AND agent_id = ?`,
			project, messageID, agentID).Scan(&exists)
		if exists == 0 {
			return fmt.Errorf("agent %s is not a recipient of message %s", agentID, messageID)
		}
	}
	return nil
}

// RecipientStatus returns the read/ack status for all recipients of a message
func (s *Store) RecipientStatus(project, messageID string) (map[string]*core.RecipientStatus, error) {
	rows, err := s.db.Query(
		`SELECT agent_id, kind, read_at, ack_at FROM message_recipients WHERE project = ? AND message_id = ?`,
		project, messageID,
	)
	if err != nil {
		return nil, fmt.Errorf("query recipients: %w", err)
	}
	defer rows.Close()

	result := make(map[string]*core.RecipientStatus)
	for rows.Next() {
		var (
			agentID, kind string
			readAt, ackAt sql.NullString
		)
		if err := rows.Scan(&agentID, &kind, &readAt, &ackAt); err != nil {
			return nil, fmt.Errorf("scan recipient: %w", err)
		}
		status := &core.RecipientStatus{
			AgentID: agentID,
			Kind:    kind,
		}
		if readAt.Valid {
			t, _ := time.Parse(time.RFC3339Nano, readAt.String)
			status.ReadAt = &t
		}
		if ackAt.Valid {
			t, _ := time.Parse(time.RFC3339Nano, ackAt.String)
			status.AckAt = &t
		}
		result[agentID] = status
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return result, nil
}

// InboxCounts returns the total and unread message counts for an agent
func (s *Store) InboxCounts(project, agentID string) (total int, unread int, err error) {
	// Total count from inbox_index
	row := s.db.QueryRow(
		`SELECT COUNT(*) FROM inbox_index WHERE project = ? AND agent = ?`,
		project, agentID,
	)
	if err := row.Scan(&total); err != nil {
		return 0, 0, fmt.Errorf("count total: %w", err)
	}

	// Unread count from message_recipients (where read_at IS NULL)
	row = s.db.QueryRow(
		`SELECT COUNT(*) FROM message_recipients WHERE project = ? AND agent_id = ? AND read_at IS NULL`,
		project, agentID,
	)
	if err := row.Scan(&unread); err != nil {
		return 0, 0, fmt.Errorf("count unread: %w", err)
	}

	return total, unread, nil
}

// Reserve creates a new file reservation
func (s *Store) Reserve(r core.Reservation) (*core.Reservation, error) {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	r.CreatedAt = now
	if r.TTL == 0 {
		r.TTL = 30 * time.Minute // Default TTL
	}
	r.ExpiresAt = now.Add(r.TTL) // Negative TTL will create already-expired reservation

	// Validate the incoming pattern early so callers get deterministic failures.
	if _, err := globPatternsOverlap(r.PathPattern, r.PathPattern); err != nil {
		return nil, fmt.Errorf("invalid reservation pattern %q: %w", r.PathPattern, err)
	}

	exclusive := 0
	if r.Exclusive {
		exclusive = 1
	}

	tx, err := s.db.BeginTx(context.Background(), nil)
	if err != nil {
		return nil, fmt.Errorf("begin reservation tx: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	activeRows, err := tx.Query(
		`SELECT id, path_pattern, exclusive
		 FROM file_reservations
		 WHERE project = ? AND released_at IS NULL AND expires_at > ? AND agent_id != ?`,
		r.Project, now.Format(time.RFC3339Nano), r.AgentID,
	)
	if err != nil {
		return nil, fmt.Errorf("query active reservations: %w", err)
	}
	defer activeRows.Close()

	for activeRows.Next() {
		var (
			existingID      string
			existingPattern string
			existingExcl    int
		)
		if err := activeRows.Scan(&existingID, &existingPattern, &existingExcl); err != nil {
			return nil, fmt.Errorf("scan active reservation: %w", err)
		}
		// Shared reservations can overlap each other.
		if !r.Exclusive && existingExcl == 0 {
			continue
		}
		overlap, err := globPatternsOverlap(r.PathPattern, existingPattern)
		if err != nil {
			return nil, fmt.Errorf("check reservation overlap against %q: %w", existingPattern, err)
		}
		if overlap {
			return nil, fmt.Errorf("reservation conflict with active reservation %s (%s)", existingID, existingPattern)
		}
	}
	if err := activeRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active reservations: %w", err)
	}
	if err := activeRows.Close(); err != nil {
		return nil, fmt.Errorf("close active reservations: %w", err)
	}

	_, err = tx.Exec(
		`INSERT INTO file_reservations (id, agent_id, project, path_pattern, exclusive, reason, created_at, expires_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		r.ID, r.AgentID, r.Project, r.PathPattern, exclusive, r.Reason,
		r.CreatedAt.Format(time.RFC3339Nano), r.ExpiresAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("insert reservation: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit reservation tx: %w", err)
	}
	return &r, nil
}

// GetReservation returns a reservation by ID
func (s *Store) GetReservation(id string) (*core.Reservation, error) {
	var (
		res                  core.Reservation
		exclusive            int
		createdAt, expiresAt string
		releasedAt           sql.NullString
	)
	err := s.db.QueryRow(
		`SELECT id, agent_id, project, path_pattern, exclusive, reason, created_at, expires_at, released_at
		 FROM file_reservations
		 WHERE id = ?`,
		id,
	).Scan(
		&res.ID, &res.AgentID, &res.Project, &res.PathPattern, &exclusive, &res.Reason, &createdAt, &expiresAt, &releasedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("reservation not found")
		}
		return nil, fmt.Errorf("get reservation: %w", err)
	}

	res.Exclusive = exclusive == 1
	res.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	res.ExpiresAt, _ = time.Parse(time.RFC3339Nano, expiresAt)
	if releasedAt.Valid {
		t, _ := time.Parse(time.RFC3339Nano, releasedAt.String)
		res.ReleasedAt = &t
	}
	return &res, nil
}

// ReleaseReservation marks a reservation as released
func (s *Store) ReleaseReservation(id string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(
		`UPDATE file_reservations SET released_at = ? WHERE id = ? AND released_at IS NULL`,
		now, id,
	)
	if err != nil {
		return fmt.Errorf("release reservation: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("reservation not found or already released")
	}
	return nil
}

// ActiveReservations returns all non-expired, non-released reservations for a project
func (s *Store) ActiveReservations(project string) ([]core.Reservation, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	rows, err := s.db.Query(
		`SELECT id, agent_id, project, path_pattern, exclusive, reason, created_at, expires_at
		 FROM file_reservations
		 WHERE project = ? AND released_at IS NULL AND expires_at > ?
		 ORDER BY created_at DESC`,
		project, now,
	)
	if err != nil {
		return nil, fmt.Errorf("query reservations: %w", err)
	}
	defer rows.Close()

	return s.scanReservations(rows)
}

// AgentReservations returns all reservations held by an agent (including expired but not released)
func (s *Store) AgentReservations(agentID string) ([]core.Reservation, error) {
	rows, err := s.db.Query(
		`SELECT id, agent_id, project, path_pattern, exclusive, reason, created_at, expires_at, released_at
		 FROM file_reservations
		 WHERE agent_id = ?
		 ORDER BY created_at DESC`,
		agentID,
	)
	if err != nil {
		return nil, fmt.Errorf("query agent reservations: %w", err)
	}
	defer rows.Close()

	return s.scanReservationsWithRelease(rows)
}

func (s *Store) scanReservations(rows *sql.Rows) ([]core.Reservation, error) {
	var out []core.Reservation
	for rows.Next() {
		var (
			id, agentID, project, pattern, reason string
			exclusive                             int
			createdAt, expiresAt                  string
		)
		if err := rows.Scan(&id, &agentID, &project, &pattern, &exclusive, &reason, &createdAt, &expiresAt); err != nil {
			return nil, fmt.Errorf("scan reservation: %w", err)
		}
		created, _ := time.Parse(time.RFC3339Nano, createdAt)
		expires, _ := time.Parse(time.RFC3339Nano, expiresAt)
		out = append(out, core.Reservation{
			ID:          id,
			AgentID:     agentID,
			Project:     project,
			PathPattern: pattern,
			Exclusive:   exclusive == 1,
			Reason:      reason,
			CreatedAt:   created,
			ExpiresAt:   expires,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return out, nil
}

func (s *Store) scanReservationsWithRelease(rows *sql.Rows) ([]core.Reservation, error) {
	var out []core.Reservation
	for rows.Next() {
		var (
			id, agentID, project, pattern, reason string
			exclusive                             int
			createdAt, expiresAt                  string
			releasedAt                            sql.NullString
		)
		if err := rows.Scan(&id, &agentID, &project, &pattern, &exclusive, &reason, &createdAt, &expiresAt, &releasedAt); err != nil {
			return nil, fmt.Errorf("scan reservation: %w", err)
		}
		created, _ := time.Parse(time.RFC3339Nano, createdAt)
		expires, _ := time.Parse(time.RFC3339Nano, expiresAt)
		r := core.Reservation{
			ID:          id,
			AgentID:     agentID,
			Project:     project,
			PathPattern: pattern,
			Exclusive:   exclusive == 1,
			Reason:      reason,
			CreatedAt:   created,
			ExpiresAt:   expires,
		}
		if releasedAt.Valid {
			t, _ := time.Parse(time.RFC3339Nano, releasedAt.String)
			r.ReleasedAt = &t
		}
		out = append(out, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows: %w", err)
	}
	return out, nil
}

type globTokenKind int

const (
	globTokenLiteral globTokenKind = iota
	globTokenAny
	globTokenStar
	globTokenClass
)

type runeRange struct {
	lo rune
	hi rune
}

type globToken struct {
	kind   globTokenKind
	lit    rune
	ranges []runeRange
}

const maxRune = rune(0x10FFFF)

var nonSeparatorRanges = []runeRange{
	{lo: 0, hi: '/' - 1},
	{lo: '/' + 1, hi: maxRune},
}

func globPatternsOverlap(a, b string) (bool, error) {
	a = filepath.ToSlash(a)
	b = filepath.ToSlash(b)

	segmentsA := strings.Split(a, "/")
	segmentsB := strings.Split(b, "/")
	if len(segmentsA) != len(segmentsB) {
		return false, nil
	}

	for i := range segmentsA {
		overlap, err := segmentPatternsOverlap(segmentsA[i], segmentsB[i])
		if err != nil {
			return false, err
		}
		if !overlap {
			return false, nil
		}
	}

	return true, nil
}

func segmentPatternsOverlap(a, b string) (bool, error) {
	tokensA, err := parseGlobSegment(a)
	if err != nil {
		return false, err
	}
	tokensB, err := parseGlobSegment(b)
	if err != nil {
		return false, err
	}

	type state struct {
		i int
		j int
	}

	addClosure := func(initial state, seen map[state]struct{}, queue *[]state) {
		stack := []state{initial}
		for len(stack) > 0 {
			curr := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if _, ok := seen[curr]; ok {
				continue
			}
			seen[curr] = struct{}{}
			*queue = append(*queue, curr)
			if curr.i < len(tokensA) && tokensA[curr.i].kind == globTokenStar {
				stack = append(stack, state{i: curr.i + 1, j: curr.j})
			}
			if curr.j < len(tokensB) && tokensB[curr.j].kind == globTokenStar {
				stack = append(stack, state{i: curr.i, j: curr.j + 1})
			}
		}
	}

	seen := make(map[state]struct{})
	queue := make([]state, 0, (len(tokensA)+1)*(len(tokensB)+1))
	addClosure(state{i: 0, j: 0}, seen, &queue)

	for idx := 0; idx < len(queue); idx++ {
		curr := queue[idx]
		if curr.i == len(tokensA) && curr.j == len(tokensB) {
			return true, nil
		}
		if curr.i == len(tokensA) || curr.j == len(tokensB) {
			continue
		}

		aNext, aRanges := tokenConsume(tokensA, curr.i)
		bNext, bRanges := tokenConsume(tokensB, curr.j)
		if !rangesOverlap(aRanges, bRanges) {
			continue
		}

		addClosure(state{i: aNext, j: bNext}, seen, &queue)
	}

	return false, nil
}

func tokenConsume(tokens []globToken, idx int) (next int, ranges []runeRange) {
	tok := tokens[idx]
	if tok.kind == globTokenStar {
		return idx, nonSeparatorRanges
	}
	if tok.kind == globTokenLiteral {
		return idx + 1, []runeRange{{lo: tok.lit, hi: tok.lit}}
	}
	return idx + 1, tok.ranges
}

func parseGlobSegment(segment string) ([]globToken, error) {
	runes := []rune(segment)
	tokens := make([]globToken, 0, len(runes))

	for i := 0; i < len(runes); {
		ch := runes[i]
		switch ch {
		case '*':
			tokens = append(tokens, globToken{kind: globTokenStar})
			i++
		case '?':
			tokens = append(tokens, globToken{kind: globTokenAny, ranges: nonSeparatorRanges})
			i++
		case '[':
			tok, next, err := parseGlobClass(runes, i)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, tok)
			i = next
		case '\\':
			if i+1 >= len(runes) {
				return nil, fmt.Errorf("bad pattern")
			}
			tokens = append(tokens, globToken{kind: globTokenLiteral, lit: runes[i+1]})
			i += 2
		default:
			tokens = append(tokens, globToken{kind: globTokenLiteral, lit: ch})
			i++
		}
	}

	return tokens, nil
}

func parseGlobClass(runes []rune, start int) (globToken, int, error) {
	i := start + 1
	if i >= len(runes) {
		return globToken{}, 0, fmt.Errorf("bad pattern")
	}
	negated := false
	if runes[i] == '^' {
		negated = true
		i++
	}

	var ranges []runeRange
	hadItem := false
	closed := false

	for i < len(runes) {
		if runes[i] == ']' && hadItem {
			i++
			closed = true
			break
		}

		lo, next, err := readClassRune(runes, i)
		if err != nil {
			return globToken{}, 0, err
		}
		i = next

		if i+1 < len(runes) && runes[i] == '-' && runes[i+1] != ']' {
			hi, nextHi, err := readClassRune(runes, i+1)
			if err != nil {
				return globToken{}, 0, err
			}
			if hi < lo {
				return globToken{}, 0, fmt.Errorf("bad pattern")
			}
			ranges = append(ranges, runeRange{lo: lo, hi: hi})
			i = nextHi
			hadItem = true
			continue
		}

		ranges = append(ranges, runeRange{lo: lo, hi: lo})
		hadItem = true
	}

	if !closed {
		return globToken{}, 0, fmt.Errorf("bad pattern")
	}

	ranges = normalizeRanges(ranges)
	if negated {
		ranges = subtractRanges(nonSeparatorRanges, ranges)
	} else {
		ranges = intersectRanges(ranges, nonSeparatorRanges)
	}

	return globToken{kind: globTokenClass, ranges: ranges}, i, nil
}

func readClassRune(runes []rune, idx int) (rune, int, error) {
	if idx >= len(runes) {
		return 0, 0, fmt.Errorf("bad pattern")
	}
	if runes[idx] != '\\' {
		return runes[idx], idx + 1, nil
	}
	if idx+1 >= len(runes) {
		return 0, 0, fmt.Errorf("bad pattern")
	}
	return runes[idx+1], idx + 2, nil
}

func rangesOverlap(a, b []runeRange) bool {
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i].hi < b[j].lo {
			i++
			continue
		}
		if b[j].hi < a[i].lo {
			j++
			continue
		}
		return true
	}
	return false
}

func intersectRanges(a, b []runeRange) []runeRange {
	a = normalizeRanges(a)
	b = normalizeRanges(b)
	out := make([]runeRange, 0)
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		lo := maxIntRune(a[i].lo, b[j].lo)
		hi := minIntRune(a[i].hi, b[j].hi)
		if lo <= hi {
			out = append(out, runeRange{lo: lo, hi: hi})
		}
		if a[i].hi < b[j].hi {
			i++
		} else {
			j++
		}
	}
	return out
}

func subtractRanges(base, subtract []runeRange) []runeRange {
	base = normalizeRanges(base)
	subtract = normalizeRanges(subtract)

	out := make([]runeRange, 0, len(base))
	for _, b := range base {
		current := []runeRange{b}
		for _, s := range subtract {
			next := make([]runeRange, 0, len(current)+1)
			for _, c := range current {
				if s.hi < c.lo || s.lo > c.hi {
					next = append(next, c)
					continue
				}
				if s.lo > c.lo {
					next = append(next, runeRange{lo: c.lo, hi: s.lo - 1})
				}
				if s.hi < c.hi {
					next = append(next, runeRange{lo: s.hi + 1, hi: c.hi})
				}
			}
			current = next
			if len(current) == 0 {
				break
			}
		}
		out = append(out, current...)
	}
	return out
}

func normalizeRanges(ranges []runeRange) []runeRange {
	if len(ranges) <= 1 {
		return ranges
	}

	cp := append([]runeRange(nil), ranges...)
	sort.Slice(cp, func(i, j int) bool {
		if cp[i].lo == cp[j].lo {
			return cp[i].hi < cp[j].hi
		}
		return cp[i].lo < cp[j].lo
	})

	out := make([]runeRange, 0, len(cp))
	current := cp[0]
	for _, rr := range cp[1:] {
		if rr.lo <= current.hi+1 {
			if rr.hi > current.hi {
				current.hi = rr.hi
			}
			continue
		}
		out = append(out, current)
		current = rr
	}
	out = append(out, current)
	return out
}

func maxIntRune(a, b rune) rune {
	if a > b {
		return a
	}
	return b
}

func minIntRune(a, b rune) rune {
	if a < b {
		return a
	}
	return b
}

// migrateDomainVersions adds version columns to domain tables (specs, epics, stories, tasks)
func migrateDomainVersions(db *sql.DB) error {
	tables := []string{"specs", "epics", "stories", "tasks"}
	for _, table := range tables {
		if !tableExists(db, table) {
			continue
		}
		if tableHasColumn(db, table, "version") {
			continue
		}
		if _, err := db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN version INTEGER NOT NULL DEFAULT 1", table)); err != nil {
			return fmt.Errorf("add version column to %s: %w", table, err)
		}
	}
	return nil
}
