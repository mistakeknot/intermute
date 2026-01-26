CREATE TABLE IF NOT EXISTS events (
  cursor INTEGER PRIMARY KEY AUTOINCREMENT,
  id TEXT NOT NULL,
  type TEXT NOT NULL,
  agent TEXT,
  project TEXT NOT NULL DEFAULT '',
  message_id TEXT,
  thread_id TEXT,
  from_agent TEXT,
  to_json TEXT,
  body TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS messages (
  project TEXT NOT NULL DEFAULT '',
  message_id TEXT NOT NULL,
  thread_id TEXT,
  from_agent TEXT,
  to_json TEXT,
  cc_json TEXT,
  bcc_json TEXT,
  subject TEXT,
  body TEXT,
  importance TEXT,
  ack_required INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  PRIMARY KEY (project, message_id)
);

CREATE TABLE IF NOT EXISTS inbox_index (
  project TEXT NOT NULL DEFAULT '',
  agent TEXT NOT NULL,
  cursor INTEGER NOT NULL,
  message_id TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_inbox_agent_cursor ON inbox_index(project, agent, cursor);

CREATE TABLE IF NOT EXISTS message_recipients (
  project TEXT NOT NULL DEFAULT '',
  message_id TEXT NOT NULL,
  agent_id TEXT NOT NULL,
  kind TEXT NOT NULL DEFAULT 'to',
  read_at TEXT,
  ack_at TEXT,
  PRIMARY KEY (project, message_id, agent_id)
);

CREATE INDEX IF NOT EXISTS idx_recipients_agent ON message_recipients(project, agent_id);

CREATE TABLE IF NOT EXISTS agents (
  id TEXT PRIMARY KEY,
  session_id TEXT,
  name TEXT NOT NULL,
  project TEXT,
  capabilities_json TEXT,
  metadata_json TEXT,
  status TEXT,
  created_at TEXT NOT NULL,
  last_seen TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS thread_index (
  project TEXT NOT NULL DEFAULT '',
  thread_id TEXT NOT NULL,
  agent TEXT NOT NULL,
  last_cursor INTEGER NOT NULL,
  message_count INTEGER NOT NULL DEFAULT 1,
  PRIMARY KEY (project, thread_id, agent)
);

CREATE INDEX IF NOT EXISTS idx_thread_agent_cursor
  ON thread_index(project, agent, last_cursor DESC);

CREATE INDEX IF NOT EXISTS idx_messages_thread
  ON messages(project, thread_id, created_at);

-- Domain tables for Autarch integration

CREATE TABLE IF NOT EXISTS specs (
  id TEXT NOT NULL,
  project TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL,
  vision TEXT,
  users TEXT,
  problem TEXT,
  status TEXT NOT NULL DEFAULT 'draft',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (project, id)
);

CREATE INDEX IF NOT EXISTS idx_specs_status ON specs(project, status);

CREATE TABLE IF NOT EXISTS epics (
  id TEXT NOT NULL,
  project TEXT NOT NULL DEFAULT '',
  spec_id TEXT,
  title TEXT NOT NULL,
  description TEXT,
  status TEXT NOT NULL DEFAULT 'open',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (project, id)
);

CREATE INDEX IF NOT EXISTS idx_epics_spec ON epics(project, spec_id);
CREATE INDEX IF NOT EXISTS idx_epics_status ON epics(project, status);

CREATE TABLE IF NOT EXISTS stories (
  id TEXT NOT NULL,
  project TEXT NOT NULL DEFAULT '',
  epic_id TEXT NOT NULL,
  title TEXT NOT NULL,
  acceptance_criteria_json TEXT,
  status TEXT NOT NULL DEFAULT 'todo',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (project, id)
);

CREATE INDEX IF NOT EXISTS idx_stories_epic ON stories(project, epic_id);
CREATE INDEX IF NOT EXISTS idx_stories_status ON stories(project, status);

CREATE TABLE IF NOT EXISTS tasks (
  id TEXT NOT NULL,
  project TEXT NOT NULL DEFAULT '',
  story_id TEXT,
  title TEXT NOT NULL,
  agent TEXT,
  session_id TEXT,
  status TEXT NOT NULL DEFAULT 'pending',
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (project, id)
);

CREATE INDEX IF NOT EXISTS idx_tasks_story ON tasks(project, story_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(project, status);
CREATE INDEX IF NOT EXISTS idx_tasks_agent ON tasks(project, agent);

CREATE TABLE IF NOT EXISTS insights (
  id TEXT NOT NULL,
  project TEXT NOT NULL DEFAULT '',
  spec_id TEXT,
  source TEXT NOT NULL,
  category TEXT NOT NULL,
  title TEXT NOT NULL,
  body TEXT,
  url TEXT,
  score REAL NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  PRIMARY KEY (project, id)
);

CREATE INDEX IF NOT EXISTS idx_insights_spec ON insights(project, spec_id);
CREATE INDEX IF NOT EXISTS idx_insights_category ON insights(project, category);
CREATE INDEX IF NOT EXISTS idx_insights_source ON insights(project, source);

CREATE TABLE IF NOT EXISTS sessions (
  id TEXT NOT NULL,
  project TEXT NOT NULL DEFAULT '',
  name TEXT NOT NULL,
  agent TEXT NOT NULL,
  task_id TEXT,
  status TEXT NOT NULL DEFAULT 'running',
  started_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (project, id)
);

CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(project, status);
CREATE INDEX IF NOT EXISTS idx_sessions_agent ON sessions(project, agent);

-- CUJ (Critical User Journey) tables

CREATE TABLE IF NOT EXISTS cujs (
  id TEXT NOT NULL,
  project TEXT NOT NULL DEFAULT '',
  spec_id TEXT NOT NULL,
  title TEXT NOT NULL,
  persona TEXT,
  priority TEXT NOT NULL DEFAULT 'medium',
  entry_point TEXT,
  exit_point TEXT,
  steps_json TEXT,
  success_criteria_json TEXT,
  error_recovery_json TEXT,
  status TEXT NOT NULL DEFAULT 'draft',
  version INTEGER NOT NULL DEFAULT 1,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  PRIMARY KEY (project, id)
);

CREATE INDEX IF NOT EXISTS idx_cujs_spec ON cujs(project, spec_id);
CREATE INDEX IF NOT EXISTS idx_cujs_status ON cujs(project, status);
CREATE INDEX IF NOT EXISTS idx_cujs_priority ON cujs(project, priority);

CREATE TABLE IF NOT EXISTS cuj_feature_links (
  project TEXT NOT NULL DEFAULT '',
  cuj_id TEXT NOT NULL,
  feature_id TEXT NOT NULL,
  linked_at TEXT NOT NULL,
  PRIMARY KEY (project, cuj_id, feature_id)
);

CREATE INDEX IF NOT EXISTS idx_cuj_links_cuj ON cuj_feature_links(project, cuj_id);
CREATE INDEX IF NOT EXISTS idx_cuj_links_feature ON cuj_feature_links(project, feature_id);
