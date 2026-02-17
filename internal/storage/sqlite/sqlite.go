package sqlite

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"github.com/google/uuid"
	"github.com/mistakeknot/intermute/internal/core"
	"github.com/mistakeknot/intermute/internal/glob"
	"github.com/mistakeknot/intermute/internal/storage"
)

//go:embed schema.sql
var schema string

type Store struct {
	db dbHandle
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
	return &Store{db: &queryLogger{inner: db}}, nil
}

func NewInMemory() (*Store, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := applySchema(db); err != nil {
		return nil, err
	}
	return &Store{db: &queryLogger{inner: db}}, nil
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
	if err := migrateAgentSessionID(db); err != nil {
		return err
	}
	return nil
}

func migrateAgentSessionID(db *sql.DB) error {
	if !tableExists(db, "agents") {
		return nil
	}
	_, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_agents_session_id
		ON agents(session_id) WHERE session_id IS NOT NULL AND session_id != ''`)
	if err != nil {
		return fmt.Errorf("create session_id index: %w", err)
	}
	return nil
}

func (s *Store) AppendEvent(_ context.Context, ev core.Event) (uint64, error) {
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

	toJSON, err := json.Marshal(ev.Message.To)
	if err != nil {
		return 0, fmt.Errorf("marshal recipients: %w", err)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin append event: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.Exec(
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
		if err := s.upsertMessageTx(tx, project, ev.Message); err != nil {
			return 0, err
		}
		recipients := ev.Message.To
		if len(recipients) == 0 && ev.Agent != "" {
			recipients = []string{ev.Agent}
		}
		for _, agent := range recipients {
			if _, err := tx.Exec(
				`INSERT INTO inbox_index (project, agent, cursor, message_id) VALUES (?, ?, ?, ?)`,
				project, agent, cursor, ev.Message.ID,
			); err != nil {
				return 0, fmt.Errorf("insert inbox: %w", err)
			}
		}
		// Insert into message_recipients for per-recipient tracking
		if err := s.insertRecipientsTx(tx, project, ev.Message.ID, ev.Message.To, "to"); err != nil {
			return 0, err
		}
		if err := s.insertRecipientsTx(tx, project, ev.Message.ID, ev.Message.CC, "cc"); err != nil {
			return 0, err
		}
		if err := s.insertRecipientsTx(tx, project, ev.Message.ID, ev.Message.BCC, "bcc"); err != nil {
			return 0, err
		}
		// Update thread_index if message has a thread ID
		if ev.Message.ThreadID != "" {
			participants := append([]string{ev.Message.From}, recipients...)
			lastBody := ev.Message.Body
			if len(lastBody) > 200 {
				lastBody = lastBody[:200]
			}
			for _, agent := range participants {
				if _, err := tx.Exec(
					`INSERT INTO thread_index (project, thread_id, agent, last_cursor, message_count,
					   last_message_from, last_message_body, last_message_at)
					 VALUES (?, ?, ?, ?, 1, ?, ?, ?)
					 ON CONFLICT(project, thread_id, agent) DO UPDATE SET
					   last_cursor = excluded.last_cursor,
					   message_count = thread_index.message_count + 1,
					   last_message_from = excluded.last_message_from,
					   last_message_body = excluded.last_message_body,
					   last_message_at = excluded.last_message_at`,
					project, ev.Message.ThreadID, agent, cursor,
					ev.Message.From, lastBody, ev.CreatedAt.Format(time.RFC3339Nano),
				); err != nil {
					return 0, fmt.Errorf("upsert thread_index: %w", err)
				}
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit append event: %w", err)
	}
	return uint64(cursor), nil
}

func (s *Store) upsertMessageTx(tx *sql.Tx, project string, msg core.Message) error {
	if project == "" {
		project = msg.Project
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now().UTC()
	}
	toJSON, err := json.Marshal(msg.To)
	if err != nil {
		return fmt.Errorf("marshal to: %w", err)
	}
	ccJSON, err := json.Marshal(msg.CC)
	if err != nil {
		return fmt.Errorf("marshal cc: %w", err)
	}
	bccJSON, err := json.Marshal(msg.BCC)
	if err != nil {
		return fmt.Errorf("marshal bcc: %w", err)
	}
	ackRequired := 0
	if msg.AckRequired {
		ackRequired = 1
	}
	if _, err := tx.Exec(
		`INSERT INTO messages (project, message_id, thread_id, from_agent, to_json, cc_json, bcc_json, subject, body, importance, ack_required, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(project, message_id) DO UPDATE SET thread_id=excluded.thread_id, from_agent=excluded.from_agent, to_json=excluded.to_json, cc_json=excluded.cc_json, bcc_json=excluded.bcc_json, subject=excluded.subject, body=excluded.body, importance=excluded.importance, ack_required=excluded.ack_required`,
		project, msg.ID, msg.ThreadID, msg.From, string(toJSON), string(ccJSON), string(bccJSON), msg.Subject, msg.Body, msg.Importance, ackRequired, msg.CreatedAt.Format(time.RFC3339Nano),
	); err != nil {
		return fmt.Errorf("upsert message: %w", err)
	}
	return nil
}

func (s *Store) InboxSince(_ context.Context, project, agent string, cursor uint64, limit int) ([]core.Message, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
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
	query += " ORDER BY i.cursor ASC LIMIT ?"
	args = append(args, limit)
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
		if err := json.Unmarshal([]byte(toJSON), &to); err != nil {
			log.Printf("WARN: corrupt to_json for message %s: %v", msgID, err)
		}
		if err := json.Unmarshal([]byte(ccJSON), &cc); err != nil {
			log.Printf("WARN: corrupt cc_json for message %s: %v", msgID, err)
		}
		if err := json.Unmarshal([]byte(bccJSON), &bcc); err != nil {
			log.Printf("WARN: corrupt bcc_json for message %s: %v", msgID, err)
		}
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

func (s *Store) ThreadMessages(_ context.Context, project, threadID string, cursor uint64) ([]core.Message, error) {
	query := `SELECT MAX(i.cursor) AS cursor, m.project, m.message_id, m.thread_id, m.from_agent, m.to_json,
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
		if err := json.Unmarshal([]byte(toJSON), &to); err != nil {
			log.Printf("WARN: corrupt to_json for message %s: %v", msgID, err)
		}
		if err := json.Unmarshal([]byte(ccJSON), &cc); err != nil {
			log.Printf("WARN: corrupt cc_json for message %s: %v", msgID, err)
		}
		if err := json.Unmarshal([]byte(bccJSON), &bcc); err != nil {
			log.Printf("WARN: corrupt bcc_json for message %s: %v", msgID, err)
		}
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

func (s *Store) ListThreads(_ context.Context, project, agent string, cursor uint64, limit int) ([]storage.ThreadSummary, error) {
	if limit <= 0 {
		limit = 50
	}
	query := `SELECT thread_id, last_cursor, message_count,
	   last_message_from, last_message_body, last_message_at
	 FROM thread_index
	 WHERE project = ? AND agent = ?`
	args := []any{project, agent}
	// With DESC ordering, cursor represents an upper bound for older pages.
	if cursor > 0 {
		query += " AND last_cursor < ?"
		args = append(args, cursor)
	}
	query += `
	 ORDER BY last_cursor DESC
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
			SELECT m.project, m.thread_id, i.agent AS agent, i.cursor AS cursor, m.message_id,
			       m.from_agent AS msg_from, SUBSTR(m.body, 1, 200) AS msg_body, m.created_at AS msg_at
			FROM messages m
			JOIN inbox_index i ON i.project = m.project AND i.message_id = m.message_id
			WHERE m.thread_id IS NOT NULL AND m.thread_id != ''
			UNION ALL
			SELECT m.project, m.thread_id, m.from_agent AS agent, e.cursor AS cursor, m.message_id,
			       m.from_agent AS msg_from, SUBSTR(m.body, 1, 200) AS msg_body, m.created_at AS msg_at
			FROM messages m
			JOIN events e ON e.project = m.project AND e.message_id = m.message_id AND e.type = ?
			WHERE m.thread_id IS NOT NULL AND m.thread_id != '' AND m.from_agent IS NOT NULL AND m.from_agent != ''
		),
		latest AS (
			SELECT project, thread_id, agent, MAX(cursor) AS last_cursor,
			       COUNT(DISTINCT message_id) AS message_count
			FROM participants
			GROUP BY project, thread_id, agent
		),
		last_msg AS (
			SELECT p.project, p.thread_id, p.agent, p.msg_from, p.msg_body, p.msg_at,
			       ROW_NUMBER() OVER (PARTITION BY p.project, p.thread_id, p.agent ORDER BY p.msg_at DESC) AS rn
			FROM participants p
		)
		INSERT INTO thread_index (project, thread_id, agent, last_cursor, message_count,
		   last_message_from, last_message_body, last_message_at)
		SELECT l.project, l.thread_id, l.agent, l.last_cursor, l.message_count,
		       COALESCE(lm.msg_from, ''), COALESCE(lm.msg_body, ''), COALESCE(lm.msg_at, '')
		FROM latest l
		LEFT JOIN last_msg lm ON lm.project = l.project AND lm.thread_id = l.thread_id
		   AND lm.agent = l.agent AND lm.rn = 1
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

func (s *Store) RegisterAgent(_ context.Context, agent core.Agent) (core.Agent, error) {
	now := time.Now().UTC()
	if agent.CreatedAt.IsZero() {
		agent.CreatedAt = now
	}
	if agent.LastSeen.IsZero() {
		agent.LastSeen = now
	}

	capsJSON, err := json.Marshal(agent.Capabilities)
	if err != nil {
		return core.Agent{}, fmt.Errorf("marshal capabilities: %w", err)
	}
	metaJSON, err := json.Marshal(agent.Metadata)
	if err != nil {
		return core.Agent{}, fmt.Errorf("marshal metadata: %w", err)
	}

	// Session identity reuse: if session_id is provided, check for existing agent
	if agent.SessionID != "" {
		if _, err := uuid.Parse(agent.SessionID); err != nil {
			return core.Agent{}, fmt.Errorf("invalid session_id %q: must be a valid UUID", agent.SessionID)
		}

		tx, err := s.db.BeginTx(context.Background(), nil)
		if err != nil {
			return core.Agent{}, fmt.Errorf("begin session reuse tx: %w", err)
		}
		defer func() { _ = tx.Rollback() }()

		var existingID, existingLastSeen string
		err = tx.QueryRow(`SELECT id, last_seen FROM agents WHERE session_id = ?`, agent.SessionID).Scan(&existingID, &existingLastSeen)
		if err == nil {
			// Found existing agent with this session_id
			lastSeen, _ := time.Parse(time.RFC3339Nano, existingLastSeen)
			if time.Since(lastSeen) < core.SessionStaleThreshold {
				return core.Agent{}, core.ErrActiveSessionConflict
			}
			// Agent is stale — check for active reservations
			var activeCount int
			err = tx.QueryRow(
				`SELECT COUNT(*) FROM file_reservations WHERE agent_id = ? AND released_at IS NULL AND expires_at > ?`,
				existingID, now.Format(time.RFC3339Nano),
			).Scan(&activeCount)
			if err != nil {
				return core.Agent{}, fmt.Errorf("check active reservations: %w", err)
			}
			if activeCount > 0 {
				return core.Agent{}, core.ErrActiveSessionConflict
			}
			// Reuse the existing agent: update its fields, keep its ID
			if _, err := tx.Exec(
				`UPDATE agents SET name=?, capabilities_json=?, metadata_json=?, status=?, last_seen=? WHERE id=?`,
				agent.Name, string(capsJSON), string(metaJSON), agent.Status,
				agent.LastSeen.Format(time.RFC3339Nano), existingID,
			); err != nil {
				return core.Agent{}, fmt.Errorf("reuse agent: %w", err)
			}
			if err := tx.Commit(); err != nil {
				return core.Agent{}, fmt.Errorf("commit session reuse: %w", err)
			}
			agent.ID = existingID
			return agent, nil
		}
		if err != sql.ErrNoRows {
			return core.Agent{}, fmt.Errorf("lookup session: %w", err)
		}
		// Not found — fall through to insert (within the transaction)
		if agent.ID == "" {
			agent.ID = uuid.NewString()
		}
		if _, err := tx.Exec(
			`INSERT INTO agents (id, session_id, name, project, capabilities_json, metadata_json, status, created_at, last_seen)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			agent.ID, agent.SessionID, agent.Name, agent.Project, string(capsJSON), string(metaJSON), agent.Status,
			agent.CreatedAt.Format(time.RFC3339Nano), agent.LastSeen.Format(time.RFC3339Nano),
		); err != nil {
			return core.Agent{}, fmt.Errorf("register agent: %w", err)
		}
		if err := tx.Commit(); err != nil {
			return core.Agent{}, fmt.Errorf("commit register: %w", err)
		}
		return agent, nil
	}

	// No session_id provided — original behavior
	if agent.ID == "" {
		agent.ID = uuid.NewString()
	}
	if agent.SessionID == "" {
		agent.SessionID = uuid.NewString()
	}
	if _, err := s.db.Exec(
		`INSERT INTO agents (id, session_id, name, project, capabilities_json, metadata_json, status, created_at, last_seen)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET session_id=excluded.session_id, name=excluded.name, project=excluded.project,
		 capabilities_json=excluded.capabilities_json, metadata_json=excluded.metadata_json, status=excluded.status, last_seen=excluded.last_seen`,
		agent.ID, agent.SessionID, agent.Name, agent.Project, string(capsJSON), string(metaJSON), agent.Status,
		agent.CreatedAt.Format(time.RFC3339Nano), agent.LastSeen.Format(time.RFC3339Nano),
	); err != nil {
		return core.Agent{}, fmt.Errorf("register agent: %w", err)
	}
	return agent, nil
}

func (s *Store) Heartbeat(_ context.Context, project, agentID string) (core.Agent, error) {
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
	if err := json.Unmarshal([]byte(capsJSON), &caps); err != nil {
		log.Printf("WARN: corrupt capabilities_json for agent %s: %v", agentID, err)
	}
	meta := map[string]string{}
	if err := json.Unmarshal([]byte(metaJSON), &meta); err != nil {
		log.Printf("WARN: corrupt metadata_json for agent %s: %v", agentID, err)
	}
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

func (s *Store) ListAgents(_ context.Context, project string) ([]core.Agent, error) {
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
		if err := json.Unmarshal([]byte(capsJSON), &caps); err != nil {
			log.Printf("WARN: corrupt capabilities_json for agent %s: %v", id, err)
		}
		meta := map[string]string{}
		if err := json.Unmarshal([]byte(metaJSON), &meta); err != nil {
			log.Printf("WARN: corrupt metadata_json for agent %s: %v", id, err)
		}
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

func (s *Store) UpdateAgentMetadata(_ context.Context, agentID string, meta map[string]string) (core.Agent, error) {
	now := time.Now().UTC()

	// Read existing metadata
	var existingMetaJSON string
	err := s.db.QueryRow(`SELECT metadata_json FROM agents WHERE id=?`, agentID).Scan(&existingMetaJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return core.Agent{}, fmt.Errorf("agent not found")
		}
		return core.Agent{}, fmt.Errorf("read metadata: %w", err)
	}

	// Merge: existing keys preserved, incoming keys overwrite
	existing := map[string]string{}
	if err := json.Unmarshal([]byte(existingMetaJSON), &existing); err != nil {
		existing = map[string]string{}
	}
	for k, v := range meta {
		existing[k] = v
	}

	mergedJSON, err := json.Marshal(existing)
	if err != nil {
		return core.Agent{}, fmt.Errorf("marshal merged metadata: %w", err)
	}

	// Update metadata + last_seen (free heartbeat)
	res, err := s.db.Exec(
		`UPDATE agents SET metadata_json=?, last_seen=? WHERE id=?`,
		string(mergedJSON), now.Format(time.RFC3339Nano), agentID,
	)
	if err != nil {
		return core.Agent{}, fmt.Errorf("update metadata: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return core.Agent{}, fmt.Errorf("agent not found")
	}

	// Fetch and return updated agent
	row := s.db.QueryRow(`SELECT id, session_id, name, project, capabilities_json, metadata_json, status, created_at, last_seen FROM agents WHERE id=?`, agentID)
	var (
		id, sessionID, name, proj, capsJSON, metaJSON, status, createdAt, lastSeen string
	)
	if err := row.Scan(&id, &sessionID, &name, &proj, &capsJSON, &metaJSON, &status, &createdAt, &lastSeen); err != nil {
		return core.Agent{}, fmt.Errorf("fetch updated agent: %w", err)
	}
	var caps []string
	if err := json.Unmarshal([]byte(capsJSON), &caps); err != nil {
		log.Printf("WARN: corrupt capabilities_json for agent %s: %v", agentID, err)
	}
	updatedMeta := map[string]string{}
	if err := json.Unmarshal([]byte(metaJSON), &updatedMeta); err != nil {
		log.Printf("WARN: corrupt metadata_json for agent %s: %v", agentID, err)
	}
	createdAtTime, _ := time.Parse(time.RFC3339Nano, createdAt)
	lastSeenTime, _ := time.Parse(time.RFC3339Nano, lastSeen)

	return core.Agent{
		ID:           id,
		SessionID:    sessionID,
		Name:         name,
		Project:      proj,
		Capabilities: caps,
		Metadata:     updatedMeta,
		Status:       status,
		CreatedAt:    createdAtTime,
		LastSeen:     lastSeenTime,
	}, nil
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

// insertRecipientsTx adds recipients to the message_recipients table within a transaction
func (s *Store) insertRecipientsTx(tx *sql.Tx, project, messageID string, agents []string, kind string) error {
	for _, agent := range agents {
		if _, err := tx.Exec(
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
func (s *Store) MarkRead(_ context.Context, project, messageID, agentID string) error {
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
func (s *Store) MarkAck(_ context.Context, project, messageID, agentID string) error {
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
func (s *Store) RecipientStatus(_ context.Context, project, messageID string) (map[string]*core.RecipientStatus, error) {
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

// InboxCounts returns the total and unread message counts for an agent.
// Both counts are read within a single transaction for snapshot consistency.
func (s *Store) InboxCounts(_ context.Context, project, agentID string) (total int, unread int, err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, 0, fmt.Errorf("begin inbox counts: %w", err)
	}
	defer tx.Rollback()

	// Total count from inbox_index
	if err := tx.QueryRow(
		`SELECT COUNT(*) FROM inbox_index WHERE project = ? AND agent = ?`,
		project, agentID,
	).Scan(&total); err != nil {
		return 0, 0, fmt.Errorf("count total: %w", err)
	}

	// Unread count from message_recipients (where read_at IS NULL)
	if err := tx.QueryRow(
		`SELECT COUNT(*) FROM message_recipients WHERE project = ? AND agent_id = ? AND read_at IS NULL`,
		project, agentID,
	).Scan(&unread); err != nil {
		return 0, 0, fmt.Errorf("count unread: %w", err)
	}

	return total, unread, nil
}

// Reserve creates a new file reservation
func (s *Store) Reserve(_ context.Context, r core.Reservation) (*core.Reservation, error) {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	r.CreatedAt = now
	if r.TTL == 0 {
		r.TTL = 30 * time.Minute // Default TTL
	}
	r.ExpiresAt = now.Add(r.TTL) // Negative TTL will create already-expired reservation

	// Validate pattern complexity to prevent NFA state explosion.
	if err := glob.ValidateComplexity(r.PathPattern); err != nil {
		return nil, fmt.Errorf("invalid reservation pattern %q: %w", r.PathPattern, err)
	}

	// Validate the incoming pattern early so callers get deterministic failures.
	if _, err := glob.PatternsOverlap(r.PathPattern, r.PathPattern); err != nil {
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
		`SELECT r.id, r.agent_id, COALESCE(a.name, r.agent_id), r.path_pattern, r.exclusive, r.reason, r.expires_at
		 FROM file_reservations r
		 LEFT JOIN agents a ON r.agent_id = a.id
		 WHERE r.project = ? AND r.released_at IS NULL AND r.expires_at > ? AND r.agent_id != ?`,
		r.Project, now.Format(time.RFC3339Nano), r.AgentID,
	)
	if err != nil {
		return nil, fmt.Errorf("query active reservations: %w", err)
	}
	defer activeRows.Close()

	var conflicts []core.ConflictDetail
	for activeRows.Next() {
		var (
			existingID      string
			existingAgentID string
			existingName    string
			existingPattern string
			existingExcl    int
			existingReason  sql.NullString
			existingExpires string
		)
		if err := activeRows.Scan(&existingID, &existingAgentID, &existingName, &existingPattern, &existingExcl, &existingReason, &existingExpires); err != nil {
			return nil, fmt.Errorf("scan active reservation: %w", err)
		}
		// Shared reservations can overlap each other.
		if !r.Exclusive && existingExcl == 0 {
			continue
		}
		overlap, err := glob.PatternsOverlap(r.PathPattern, existingPattern)
		if err != nil {
			return nil, fmt.Errorf("check reservation overlap against %q: %w", existingPattern, err)
		}
		if overlap {
			expiresAt, _ := time.Parse(time.RFC3339Nano, existingExpires)
			conflicts = append(conflicts, core.ConflictDetail{
				ReservationID: existingID,
				AgentID:       existingAgentID,
				AgentName:     existingName,
				Pattern:       existingPattern,
				Reason:        existingReason.String,
				ExpiresAt:     expiresAt,
			})
		}
	}
	if err := activeRows.Err(); err != nil {
		return nil, fmt.Errorf("iterate active reservations: %w", err)
	}
	if err := activeRows.Close(); err != nil {
		return nil, fmt.Errorf("close active reservations: %w", err)
	}
	if len(conflicts) > 0 {
		return nil, &core.ConflictError{Conflicts: conflicts}
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
func (s *Store) GetReservation(_ context.Context, id string) (*core.Reservation, error) {
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
			return nil, core.ErrNotFound
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

// ReleaseReservation marks a reservation as released, enforcing agent ownership atomically
func (s *Store) ReleaseReservation(_ context.Context, id, agentID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(
		`UPDATE file_reservations SET released_at = ? WHERE id = ? AND agent_id = ? AND released_at IS NULL`,
		now, id, agentID,
	)
	if err != nil {
		return fmt.Errorf("release reservation: %w", err)
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return core.ErrNotFound
	}
	return nil
}

// ActiveReservations returns all non-expired, non-released reservations for a project
func (s *Store) ActiveReservations(_ context.Context, project string) ([]core.Reservation, error) {
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
func (s *Store) AgentReservations(_ context.Context, agentID string) ([]core.Reservation, error) {
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

// CheckConflicts returns active reservations that would conflict with the given pattern.
func (s *Store) CheckConflicts(_ context.Context, project, pathPattern string, exclusive bool) ([]core.ConflictDetail, error) {
	if err := glob.ValidateComplexity(pathPattern); err != nil {
		return nil, fmt.Errorf("invalid pattern %q: %w", pathPattern, err)
	}

	now := time.Now().UTC()
	rows, err := s.db.Query(
		`SELECT r.id, r.agent_id, COALESCE(a.name, r.agent_id), r.path_pattern, r.exclusive, r.reason, r.expires_at
		 FROM file_reservations r
		 LEFT JOIN agents a ON r.agent_id = a.id
		 WHERE r.project = ? AND r.released_at IS NULL AND r.expires_at > ?`,
		project, now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("query active reservations: %w", err)
	}
	defer rows.Close()

	var conflicts []core.ConflictDetail
	for rows.Next() {
		var (
			id, agentID, name, pattern string
			excl                       int
			reasonNull                 sql.NullString
			expiresStr                 string
		)
		if err := rows.Scan(&id, &agentID, &name, &pattern, &excl, &reasonNull, &expiresStr); err != nil {
			return nil, fmt.Errorf("scan: %w", err)
		}
		if !exclusive && excl == 0 {
			continue // shared-shared is always allowed
		}
		overlap, err := glob.PatternsOverlap(pathPattern, pattern)
		if err != nil {
			continue // skip invalid patterns
		}
		if overlap {
			expiresAt, _ := time.Parse(time.RFC3339Nano, expiresStr)
			conflicts = append(conflicts, core.ConflictDetail{
				ReservationID: id,
				AgentID:       agentID,
				AgentName:     name,
				Pattern:       pattern,
				Reason:        reasonNull.String,
				ExpiresAt:     expiresAt,
			})
		}
	}
	return conflicts, rows.Err()
}

// SweepExpired deletes unreleased reservations that have expired and whose
// owning agent has not heartbeated recently. Returns deleted reservations.
func (s *Store) SweepExpired(_ context.Context, expiredBefore time.Time, heartbeatAfter time.Time) ([]core.Reservation, error) {
	rows, err := s.db.Query(
		`DELETE FROM file_reservations
		 WHERE released_at IS NULL
		   AND expires_at < ?
		   AND agent_id NOT IN (
		     SELECT id FROM agents WHERE last_seen > ?
		   )
		 RETURNING id, agent_id, project, path_pattern, exclusive, reason, created_at, expires_at`,
		expiredBefore.Format(time.RFC3339Nano),
		heartbeatAfter.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("sweep expired reservations: %w", err)
	}
	defer rows.Close()

	return s.scanReservations(rows)
}

// Close checkpoints the WAL and closes the database connection.
func (s *Store) Close() error {
	if ql, ok := s.db.(*queryLogger); ok {
		_, _ = ql.inner.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
		return ql.inner.Close()
	}
	return nil
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
