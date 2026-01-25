package sqlite

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mistakeknot/intermute/internal/core"
)

// Spec operations

func (s *Store) CreateSpec(spec core.Spec) (core.Spec, error) {
	if spec.ID == "" {
		spec.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if spec.CreatedAt.IsZero() {
		spec.CreatedAt = now
	}
	if spec.UpdatedAt.IsZero() {
		spec.UpdatedAt = now
	}
	if spec.Status == "" {
		spec.Status = core.SpecStatusDraft
	}

	_, err := s.db.Exec(
		`INSERT INTO specs (id, project, title, vision, users, problem, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		spec.ID, spec.Project, spec.Title, spec.Vision, spec.Users, spec.Problem,
		string(spec.Status), spec.CreatedAt.Format(time.RFC3339Nano), spec.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return core.Spec{}, fmt.Errorf("create spec: %w", err)
	}
	return spec, nil
}

func (s *Store) GetSpec(project, id string) (core.Spec, error) {
	row := s.db.QueryRow(
		`SELECT id, project, title, vision, users, problem, status, created_at, updated_at
		 FROM specs WHERE project = ? AND id = ?`,
		project, id,
	)
	return scanSpec(row)
}

func (s *Store) ListSpecs(project string, status string) ([]core.Spec, error) {
	query := `SELECT id, project, title, vision, users, problem, status, created_at, updated_at FROM specs`
	var args []any
	if project != "" {
		query += " WHERE project = ?"
		args = append(args, project)
		if status != "" {
			query += " AND status = ?"
			args = append(args, status)
		}
	} else if status != "" {
		query += " WHERE status = ?"
		args = append(args, status)
	}
	query += " ORDER BY updated_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list specs: %w", err)
	}
	defer rows.Close()

	var specs []core.Spec
	for rows.Next() {
		spec, err := scanSpecRow(rows)
		if err != nil {
			return nil, err
		}
		specs = append(specs, spec)
	}
	return specs, rows.Err()
}

func (s *Store) UpdateSpec(spec core.Spec) (core.Spec, error) {
	spec.UpdatedAt = time.Now().UTC()
	_, err := s.db.Exec(
		`UPDATE specs SET title = ?, vision = ?, users = ?, problem = ?, status = ?, updated_at = ?
		 WHERE project = ? AND id = ?`,
		spec.Title, spec.Vision, spec.Users, spec.Problem, string(spec.Status),
		spec.UpdatedAt.Format(time.RFC3339Nano), spec.Project, spec.ID,
	)
	if err != nil {
		return core.Spec{}, fmt.Errorf("update spec: %w", err)
	}
	return spec, nil
}

func (s *Store) DeleteSpec(project, id string) error {
	_, err := s.db.Exec(`DELETE FROM specs WHERE project = ? AND id = ?`, project, id)
	if err != nil {
		return fmt.Errorf("delete spec: %w", err)
	}
	return nil
}

// Epic operations

func (s *Store) CreateEpic(epic core.Epic) (core.Epic, error) {
	if epic.ID == "" {
		epic.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if epic.CreatedAt.IsZero() {
		epic.CreatedAt = now
	}
	if epic.UpdatedAt.IsZero() {
		epic.UpdatedAt = now
	}
	if epic.Status == "" {
		epic.Status = core.EpicStatusOpen
	}

	_, err := s.db.Exec(
		`INSERT INTO epics (id, project, spec_id, title, description, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		epic.ID, epic.Project, epic.SpecID, epic.Title, epic.Description,
		string(epic.Status), epic.CreatedAt.Format(time.RFC3339Nano), epic.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return core.Epic{}, fmt.Errorf("create epic: %w", err)
	}
	return epic, nil
}

func (s *Store) GetEpic(project, id string) (core.Epic, error) {
	row := s.db.QueryRow(
		`SELECT id, project, spec_id, title, description, status, created_at, updated_at
		 FROM epics WHERE project = ? AND id = ?`,
		project, id,
	)
	return scanEpic(row)
}

func (s *Store) ListEpics(project, specID string) ([]core.Epic, error) {
	query := `SELECT id, project, spec_id, title, description, status, created_at, updated_at FROM epics`
	var args []any
	if project != "" {
		query += " WHERE project = ?"
		args = append(args, project)
		if specID != "" {
			query += " AND spec_id = ?"
			args = append(args, specID)
		}
	} else if specID != "" {
		query += " WHERE spec_id = ?"
		args = append(args, specID)
	}
	query += " ORDER BY updated_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list epics: %w", err)
	}
	defer rows.Close()

	var epics []core.Epic
	for rows.Next() {
		epic, err := scanEpicRow(rows)
		if err != nil {
			return nil, err
		}
		epics = append(epics, epic)
	}
	return epics, rows.Err()
}

func (s *Store) UpdateEpic(epic core.Epic) (core.Epic, error) {
	epic.UpdatedAt = time.Now().UTC()
	_, err := s.db.Exec(
		`UPDATE epics SET spec_id = ?, title = ?, description = ?, status = ?, updated_at = ?
		 WHERE project = ? AND id = ?`,
		epic.SpecID, epic.Title, epic.Description, string(epic.Status),
		epic.UpdatedAt.Format(time.RFC3339Nano), epic.Project, epic.ID,
	)
	if err != nil {
		return core.Epic{}, fmt.Errorf("update epic: %w", err)
	}
	return epic, nil
}

func (s *Store) DeleteEpic(project, id string) error {
	_, err := s.db.Exec(`DELETE FROM epics WHERE project = ? AND id = ?`, project, id)
	if err != nil {
		return fmt.Errorf("delete epic: %w", err)
	}
	return nil
}

// Story operations

func (s *Store) CreateStory(story core.Story) (core.Story, error) {
	if story.ID == "" {
		story.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if story.CreatedAt.IsZero() {
		story.CreatedAt = now
	}
	if story.UpdatedAt.IsZero() {
		story.UpdatedAt = now
	}
	if story.Status == "" {
		story.Status = core.StoryStatusTodo
	}

	acJSON, _ := json.Marshal(story.AcceptanceCriteria)
	_, err := s.db.Exec(
		`INSERT INTO stories (id, project, epic_id, title, acceptance_criteria_json, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		story.ID, story.Project, story.EpicID, story.Title, string(acJSON),
		string(story.Status), story.CreatedAt.Format(time.RFC3339Nano), story.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return core.Story{}, fmt.Errorf("create story: %w", err)
	}
	return story, nil
}

func (s *Store) GetStory(project, id string) (core.Story, error) {
	row := s.db.QueryRow(
		`SELECT id, project, epic_id, title, acceptance_criteria_json, status, created_at, updated_at
		 FROM stories WHERE project = ? AND id = ?`,
		project, id,
	)
	return scanStory(row)
}

func (s *Store) ListStories(project, epicID string) ([]core.Story, error) {
	query := `SELECT id, project, epic_id, title, acceptance_criteria_json, status, created_at, updated_at FROM stories`
	var args []any
	if project != "" {
		query += " WHERE project = ?"
		args = append(args, project)
		if epicID != "" {
			query += " AND epic_id = ?"
			args = append(args, epicID)
		}
	} else if epicID != "" {
		query += " WHERE epic_id = ?"
		args = append(args, epicID)
	}
	query += " ORDER BY updated_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list stories: %w", err)
	}
	defer rows.Close()

	var stories []core.Story
	for rows.Next() {
		story, err := scanStoryRow(rows)
		if err != nil {
			return nil, err
		}
		stories = append(stories, story)
	}
	return stories, rows.Err()
}

func (s *Store) UpdateStory(story core.Story) (core.Story, error) {
	story.UpdatedAt = time.Now().UTC()
	acJSON, _ := json.Marshal(story.AcceptanceCriteria)
	_, err := s.db.Exec(
		`UPDATE stories SET epic_id = ?, title = ?, acceptance_criteria_json = ?, status = ?, updated_at = ?
		 WHERE project = ? AND id = ?`,
		story.EpicID, story.Title, string(acJSON), string(story.Status),
		story.UpdatedAt.Format(time.RFC3339Nano), story.Project, story.ID,
	)
	if err != nil {
		return core.Story{}, fmt.Errorf("update story: %w", err)
	}
	return story, nil
}

func (s *Store) DeleteStory(project, id string) error {
	_, err := s.db.Exec(`DELETE FROM stories WHERE project = ? AND id = ?`, project, id)
	if err != nil {
		return fmt.Errorf("delete story: %w", err)
	}
	return nil
}

// Task operations

func (s *Store) CreateTask(task core.Task) (core.Task, error) {
	if task.ID == "" {
		task.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	if task.UpdatedAt.IsZero() {
		task.UpdatedAt = now
	}
	if task.Status == "" {
		task.Status = core.TaskStatusPending
	}

	_, err := s.db.Exec(
		`INSERT INTO tasks (id, project, story_id, title, agent, session_id, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.ID, task.Project, task.StoryID, task.Title, task.Agent, task.SessionID,
		string(task.Status), task.CreatedAt.Format(time.RFC3339Nano), task.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return core.Task{}, fmt.Errorf("create task: %w", err)
	}
	return task, nil
}

func (s *Store) GetTask(project, id string) (core.Task, error) {
	row := s.db.QueryRow(
		`SELECT id, project, story_id, title, agent, session_id, status, created_at, updated_at
		 FROM tasks WHERE project = ? AND id = ?`,
		project, id,
	)
	return scanTask(row)
}

func (s *Store) ListTasks(project, status, agent string) ([]core.Task, error) {
	query := `SELECT id, project, story_id, title, agent, session_id, status, created_at, updated_at FROM tasks WHERE 1=1`
	var args []any
	if project != "" {
		query += " AND project = ?"
		args = append(args, project)
	}
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	if agent != "" {
		query += " AND agent = ?"
		args = append(args, agent)
	}
	query += " ORDER BY updated_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	var tasks []core.Task
	for rows.Next() {
		task, err := scanTaskRow(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (s *Store) UpdateTask(task core.Task) (core.Task, error) {
	task.UpdatedAt = time.Now().UTC()
	_, err := s.db.Exec(
		`UPDATE tasks SET story_id = ?, title = ?, agent = ?, session_id = ?, status = ?, updated_at = ?
		 WHERE project = ? AND id = ?`,
		task.StoryID, task.Title, task.Agent, task.SessionID, string(task.Status),
		task.UpdatedAt.Format(time.RFC3339Nano), task.Project, task.ID,
	)
	if err != nil {
		return core.Task{}, fmt.Errorf("update task: %w", err)
	}
	return task, nil
}

func (s *Store) DeleteTask(project, id string) error {
	_, err := s.db.Exec(`DELETE FROM tasks WHERE project = ? AND id = ?`, project, id)
	if err != nil {
		return fmt.Errorf("delete task: %w", err)
	}
	return nil
}

// Insight operations

func (s *Store) CreateInsight(insight core.Insight) (core.Insight, error) {
	if insight.ID == "" {
		insight.ID = uuid.NewString()
	}
	if insight.CreatedAt.IsZero() {
		insight.CreatedAt = time.Now().UTC()
	}

	_, err := s.db.Exec(
		`INSERT INTO insights (id, project, spec_id, source, category, title, body, url, score, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		insight.ID, insight.Project, insight.SpecID, insight.Source, insight.Category,
		insight.Title, insight.Body, insight.URL, insight.Score, insight.CreatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return core.Insight{}, fmt.Errorf("create insight: %w", err)
	}
	return insight, nil
}

func (s *Store) GetInsight(project, id string) (core.Insight, error) {
	row := s.db.QueryRow(
		`SELECT id, project, spec_id, source, category, title, body, url, score, created_at
		 FROM insights WHERE project = ? AND id = ?`,
		project, id,
	)
	return scanInsight(row)
}

func (s *Store) ListInsights(project, specID, category string) ([]core.Insight, error) {
	query := `SELECT id, project, spec_id, source, category, title, body, url, score, created_at FROM insights WHERE 1=1`
	var args []any
	if project != "" {
		query += " AND project = ?"
		args = append(args, project)
	}
	if specID != "" {
		query += " AND spec_id = ?"
		args = append(args, specID)
	}
	if category != "" {
		query += " AND category = ?"
		args = append(args, category)
	}
	query += " ORDER BY score DESC, created_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list insights: %w", err)
	}
	defer rows.Close()

	var insights []core.Insight
	for rows.Next() {
		insight, err := scanInsightRow(rows)
		if err != nil {
			return nil, err
		}
		insights = append(insights, insight)
	}
	return insights, rows.Err()
}

func (s *Store) LinkInsightToSpec(project, insightID, specID string) error {
	_, err := s.db.Exec(
		`UPDATE insights SET spec_id = ? WHERE project = ? AND id = ?`,
		specID, project, insightID,
	)
	if err != nil {
		return fmt.Errorf("link insight: %w", err)
	}
	return nil
}

func (s *Store) DeleteInsight(project, id string) error {
	_, err := s.db.Exec(`DELETE FROM insights WHERE project = ? AND id = ?`, project, id)
	if err != nil {
		return fmt.Errorf("delete insight: %w", err)
	}
	return nil
}

// Session operations

func (s *Store) CreateSession(session core.Session) (core.Session, error) {
	if session.ID == "" {
		session.ID = uuid.NewString()
	}
	now := time.Now().UTC()
	if session.StartedAt.IsZero() {
		session.StartedAt = now
	}
	if session.UpdatedAt.IsZero() {
		session.UpdatedAt = now
	}
	if session.Status == "" {
		session.Status = core.SessionStatusRunning
	}

	_, err := s.db.Exec(
		`INSERT INTO sessions (id, project, name, agent, task_id, status, started_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.Project, session.Name, session.Agent, session.TaskID,
		string(session.Status), session.StartedAt.Format(time.RFC3339Nano), session.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return core.Session{}, fmt.Errorf("create session: %w", err)
	}
	return session, nil
}

func (s *Store) GetSession(project, id string) (core.Session, error) {
	row := s.db.QueryRow(
		`SELECT id, project, name, agent, task_id, status, started_at, updated_at
		 FROM sessions WHERE project = ? AND id = ?`,
		project, id,
	)
	return scanSession(row)
}

func (s *Store) ListSessions(project, status string) ([]core.Session, error) {
	query := `SELECT id, project, name, agent, task_id, status, started_at, updated_at FROM sessions WHERE 1=1`
	var args []any
	if project != "" {
		query += " AND project = ?"
		args = append(args, project)
	}
	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	query += " ORDER BY started_at DESC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	defer rows.Close()

	var sessions []core.Session
	for rows.Next() {
		session, err := scanSessionRow(rows)
		if err != nil {
			return nil, err
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func (s *Store) UpdateSession(session core.Session) (core.Session, error) {
	session.UpdatedAt = time.Now().UTC()
	_, err := s.db.Exec(
		`UPDATE sessions SET name = ?, agent = ?, task_id = ?, status = ?, updated_at = ?
		 WHERE project = ? AND id = ?`,
		session.Name, session.Agent, session.TaskID, string(session.Status),
		session.UpdatedAt.Format(time.RFC3339Nano), session.Project, session.ID,
	)
	if err != nil {
		return core.Session{}, fmt.Errorf("update session: %w", err)
	}
	return session, nil
}

func (s *Store) DeleteSession(project, id string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE project = ? AND id = ?`, project, id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// Scanner helpers

type scanner interface {
	Scan(dest ...any) error
}

func scanSpec(row scanner) (core.Spec, error) {
	var s core.Spec
	var vision, users, problem sql.NullString
	var createdAt, updatedAt, status string
	err := row.Scan(&s.ID, &s.Project, &s.Title, &vision, &users, &problem, &status, &createdAt, &updatedAt)
	if err != nil {
		return core.Spec{}, fmt.Errorf("scan spec: %w", err)
	}
	s.Vision = vision.String
	s.Users = users.String
	s.Problem = problem.String
	s.Status = core.SpecStatus(status)
	s.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	s.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return s, nil
}

func scanSpecRow(rows *sql.Rows) (core.Spec, error) {
	return scanSpec(rows)
}

func scanEpic(row scanner) (core.Epic, error) {
	var e core.Epic
	var specID, description sql.NullString
	var createdAt, updatedAt, status string
	err := row.Scan(&e.ID, &e.Project, &specID, &e.Title, &description, &status, &createdAt, &updatedAt)
	if err != nil {
		return core.Epic{}, fmt.Errorf("scan epic: %w", err)
	}
	e.SpecID = specID.String
	e.Description = description.String
	e.Status = core.EpicStatus(status)
	e.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	e.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return e, nil
}

func scanEpicRow(rows *sql.Rows) (core.Epic, error) {
	return scanEpic(rows)
}

func scanStory(row scanner) (core.Story, error) {
	var s core.Story
	var acJSON sql.NullString
	var createdAt, updatedAt, status string
	err := row.Scan(&s.ID, &s.Project, &s.EpicID, &s.Title, &acJSON, &status, &createdAt, &updatedAt)
	if err != nil {
		return core.Story{}, fmt.Errorf("scan story: %w", err)
	}
	if acJSON.Valid {
		_ = json.Unmarshal([]byte(acJSON.String), &s.AcceptanceCriteria)
	}
	s.Status = core.StoryStatus(status)
	s.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	s.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return s, nil
}

func scanStoryRow(rows *sql.Rows) (core.Story, error) {
	return scanStory(rows)
}

func scanTask(row scanner) (core.Task, error) {
	var t core.Task
	var storyID, agent, sessionID sql.NullString
	var createdAt, updatedAt, status string
	err := row.Scan(&t.ID, &t.Project, &storyID, &t.Title, &agent, &sessionID, &status, &createdAt, &updatedAt)
	if err != nil {
		return core.Task{}, fmt.Errorf("scan task: %w", err)
	}
	t.StoryID = storyID.String
	t.Agent = agent.String
	t.SessionID = sessionID.String
	t.Status = core.TaskStatus(status)
	t.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	t.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return t, nil
}

func scanTaskRow(rows *sql.Rows) (core.Task, error) {
	return scanTask(rows)
}

func scanInsight(row scanner) (core.Insight, error) {
	var i core.Insight
	var specID, body, url sql.NullString
	var createdAt string
	err := row.Scan(&i.ID, &i.Project, &specID, &i.Source, &i.Category, &i.Title, &body, &url, &i.Score, &createdAt)
	if err != nil {
		return core.Insight{}, fmt.Errorf("scan insight: %w", err)
	}
	i.SpecID = specID.String
	i.Body = body.String
	i.URL = url.String
	i.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	return i, nil
}

func scanInsightRow(rows *sql.Rows) (core.Insight, error) {
	return scanInsight(rows)
}

func scanSession(row scanner) (core.Session, error) {
	var s core.Session
	var taskID sql.NullString
	var startedAt, updatedAt, status string
	err := row.Scan(&s.ID, &s.Project, &s.Name, &s.Agent, &taskID, &status, &startedAt, &updatedAt)
	if err != nil {
		return core.Session{}, fmt.Errorf("scan session: %w", err)
	}
	s.TaskID = taskID.String
	s.Status = core.SessionStatus(status)
	s.StartedAt, _ = time.Parse(time.RFC3339Nano, startedAt)
	s.UpdatedAt, _ = time.Parse(time.RFC3339Nano, updatedAt)
	return s, nil
}

func scanSessionRow(rows *sql.Rows) (core.Session, error) {
	return scanSession(rows)
}
