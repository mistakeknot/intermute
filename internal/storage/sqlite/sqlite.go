package sqlite

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
			cur                                                                            int64
			proj                                                                           string
			msgID, thID, fromAgent, toJSON, ccJSON, bccJSON, subject, body, importance     string
			ackRequired                                                                    int
			createdAt                                                                      string
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
			ThreadID:   thID,
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
			agentID, kind   string
			readAt, ackAt   sql.NullString
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
