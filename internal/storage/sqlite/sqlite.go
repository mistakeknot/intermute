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
	_, err := s.db.Exec(
		`INSERT INTO messages (project, message_id, thread_id, from_agent, to_json, body, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(project, message_id) DO UPDATE SET thread_id=excluded.thread_id, from_agent=excluded.from_agent, to_json=excluded.to_json, body=excluded.body`,
		project, msg.ID, msg.ThreadID, msg.From, string(toJSON), msg.Body, msg.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert message: %w", err)
	}
	return nil
}

func (s *Store) InboxSince(project, agent string, cursor uint64) ([]core.Message, error) {
	query := `SELECT i.cursor, i.project, m.message_id, m.thread_id, m.from_agent, m.to_json, m.body, m.created_at
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
			cur int64
			proj                                                  string
			msgID, threadID, fromAgent, toJSON, body, createdAt   string
		)
		if err := rows.Scan(&cur, &proj, &msgID, &threadID, &fromAgent, &toJSON, &body, &createdAt); err != nil {
			return nil, fmt.Errorf("scan inbox: %w", err)
		}
		var to []string
		_ = json.Unmarshal([]byte(toJSON), &to)
		parsed, _ := time.Parse(time.RFC3339Nano, createdAt)
		out = append(out, core.Message{
			ID:        msgID,
			ThreadID:  threadID,
			Project:   proj,
			From:      fromAgent,
			To:        to,
			Body:      body,
			CreatedAt: parsed,
			Cursor:    uint64(cur),
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

func (s *Store) Heartbeat(agentID string) (core.Agent, error) {
	now := time.Now().UTC()
	_, err := s.db.Exec(`UPDATE agents SET last_seen=? WHERE id=?`, now.Format(time.RFC3339Nano), agentID)
	if err != nil {
		return core.Agent{}, fmt.Errorf("heartbeat: %w", err)
	}

	row := s.db.QueryRow(`SELECT id, session_id, name, project, capabilities_json, metadata_json, status, created_at, last_seen FROM agents WHERE id=?`, agentID)
	var (
		id, sessionID, name, project, capsJSON, metaJSON, status, createdAt, lastSeen string
	)
	if err := row.Scan(&id, &sessionID, &name, &project, &capsJSON, &metaJSON, &status, &createdAt, &lastSeen); err != nil {
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
		Project:      project,
		Capabilities: caps,
		Metadata:     meta,
		Status:       status,
		CreatedAt:    createdAtTime,
		LastSeen:     lastSeenTime,
	}, nil
}
