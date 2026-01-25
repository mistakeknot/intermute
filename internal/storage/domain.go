package storage

import "github.com/mistakeknot/intermute/internal/core"

// DomainStore extends Store with domain entity operations
type DomainStore interface {
	Store

	// Spec operations
	CreateSpec(spec core.Spec) (core.Spec, error)
	GetSpec(project, id string) (core.Spec, error)
	ListSpecs(project, status string) ([]core.Spec, error)
	UpdateSpec(spec core.Spec) (core.Spec, error)
	DeleteSpec(project, id string) error

	// Epic operations
	CreateEpic(epic core.Epic) (core.Epic, error)
	GetEpic(project, id string) (core.Epic, error)
	ListEpics(project, specID string) ([]core.Epic, error)
	UpdateEpic(epic core.Epic) (core.Epic, error)
	DeleteEpic(project, id string) error

	// Story operations
	CreateStory(story core.Story) (core.Story, error)
	GetStory(project, id string) (core.Story, error)
	ListStories(project, epicID string) ([]core.Story, error)
	UpdateStory(story core.Story) (core.Story, error)
	DeleteStory(project, id string) error

	// Task operations
	CreateTask(task core.Task) (core.Task, error)
	GetTask(project, id string) (core.Task, error)
	ListTasks(project, status, agent string) ([]core.Task, error)
	UpdateTask(task core.Task) (core.Task, error)
	DeleteTask(project, id string) error

	// Insight operations
	CreateInsight(insight core.Insight) (core.Insight, error)
	GetInsight(project, id string) (core.Insight, error)
	ListInsights(project, specID, category string) ([]core.Insight, error)
	LinkInsightToSpec(project, insightID, specID string) error
	DeleteInsight(project, id string) error

	// Session operations
	CreateSession(session core.Session) (core.Session, error)
	GetSession(project, id string) (core.Session, error)
	ListSessions(project, status string) ([]core.Session, error)
	UpdateSession(session core.Session) (core.Session, error)
	DeleteSession(project, id string) error
}
