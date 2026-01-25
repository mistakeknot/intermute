package sqlite

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	return nil
}

func (s *Store) AppendEvent(ev core.Event) (uint64, error) {
	if ev.ID == "" {
		ev.ID = uuid.NewString()
	}
	if ev.CreatedAt.IsZero() {
		ev.CreatedAt = time.Now().UTC()
	}

	toJSON, _ := json.Marshal(ev.Message.To)

	res, err := s.db.Exec(
		`INSERT INTO events (id, type, agent, message_id, thread_id, from_agent, to_json, body, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ev.ID, string(ev.Type), ev.Agent, ev.Message.ID, ev.Message.ThreadID, ev.Message.From, string(toJSON), ev.Message.Body, ev.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return 0, fmt.Errorf("insert event: %w", err)
	}
	cursor, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("cursor: %w", err)
	}

	if ev.Type == core.EventMessageCreated {
		if err := s.upsertMessage(ev.Message); err != nil {
			return 0, err
		}
		recipients := ev.Message.To
		if len(recipients) == 0 && ev.Agent != "" {
			recipients = []string{ev.Agent}
		}
		for _, agent := range recipients {
			if _, err := s.db.Exec(
				`INSERT INTO inbox_index (agent, cursor, message_id) VALUES (?, ?, ?)`,
				agent, cursor, ev.Message.ID,
			); err != nil {
				return 0, fmt.Errorf("insert inbox: %w", err)
			}
		}
	}

	return uint64(cursor), nil
}

func (s *Store) upsertMessage(msg core.Message) error {
	toJSON, _ := json.Marshal(msg.To)
	_, err := s.db.Exec(
		`INSERT INTO messages (message_id, thread_id, from_agent, to_json, body, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(message_id) DO UPDATE SET thread_id=excluded.thread_id, from_agent=excluded.from_agent, to_json=excluded.to_json, body=excluded.body`,
		msg.ID, msg.ThreadID, msg.From, string(toJSON), msg.Body, msg.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("upsert message: %w", err)
	}
	return nil
}

func (s *Store) InboxSince(agent string, cursor uint64) ([]core.Message, error) {
	rows, err := s.db.Query(
		`SELECT i.cursor, m.message_id, m.thread_id, m.from_agent, m.to_json, m.body, m.created_at
		 FROM inbox_index i
		 JOIN messages m ON m.message_id = i.message_id
		 WHERE i.agent = ? AND i.cursor > ?
		 ORDER BY i.cursor ASC`, agent, cursor,
	)
	if err != nil {
		return nil, fmt.Errorf("query inbox: %w", err)
	}
	defer rows.Close()

	var out []core.Message
	for rows.Next() {
		var (
			cur int64
			msgID, threadID, fromAgent, toJSON, body, createdAt string
		)
		if err := rows.Scan(&cur, &msgID, &threadID, &fromAgent, &toJSON, &body, &createdAt); err != nil {
			return nil, fmt.Errorf("scan inbox: %w", err)
		}
		var to []string
		_ = json.Unmarshal([]byte(toJSON), &to)
		parsed, _ := time.Parse(time.RFC3339Nano, createdAt)
		out = append(out, core.Message{
			ID:        msgID,
			ThreadID:  threadID,
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
