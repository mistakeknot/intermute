package storage

import (
	"context"

	"github.com/mistakeknot/intermute/internal/core"
)

// DomainStore extends Store with domain entity operations
type DomainStore interface {
	Store

	// Spec operations
	CreateSpec(ctx context.Context, spec core.Spec) (core.Spec, error)
	GetSpec(ctx context.Context, project, id string) (core.Spec, error)
	ListSpecs(ctx context.Context, project, status string) ([]core.Spec, error)
	UpdateSpec(ctx context.Context, spec core.Spec) (core.Spec, error)
	DeleteSpec(ctx context.Context, project, id string) error

	// Epic operations
	CreateEpic(ctx context.Context, epic core.Epic) (core.Epic, error)
	GetEpic(ctx context.Context, project, id string) (core.Epic, error)
	ListEpics(ctx context.Context, project, specID string) ([]core.Epic, error)
	UpdateEpic(ctx context.Context, epic core.Epic) (core.Epic, error)
	DeleteEpic(ctx context.Context, project, id string) error

	// Story operations
	CreateStory(ctx context.Context, story core.Story) (core.Story, error)
	GetStory(ctx context.Context, project, id string) (core.Story, error)
	ListStories(ctx context.Context, project, epicID string) ([]core.Story, error)
	UpdateStory(ctx context.Context, story core.Story) (core.Story, error)
	DeleteStory(ctx context.Context, project, id string) error

	// Task operations
	CreateTask(ctx context.Context, task core.Task) (core.Task, error)
	GetTask(ctx context.Context, project, id string) (core.Task, error)
	ListTasks(ctx context.Context, project, status, agent string) ([]core.Task, error)
	UpdateTask(ctx context.Context, task core.Task) (core.Task, error)
	DeleteTask(ctx context.Context, project, id string) error

	// Insight operations
	CreateInsight(ctx context.Context, insight core.Insight) (core.Insight, error)
	GetInsight(ctx context.Context, project, id string) (core.Insight, error)
	ListInsights(ctx context.Context, project, specID, category string) ([]core.Insight, error)
	LinkInsightToSpec(ctx context.Context, project, insightID, specID string) error
	DeleteInsight(ctx context.Context, project, id string) error

	// Session operations
	CreateSession(ctx context.Context, session core.Session) (core.Session, error)
	GetSession(ctx context.Context, project, id string) (core.Session, error)
	ListSessions(ctx context.Context, project, status string) ([]core.Session, error)
	UpdateSession(ctx context.Context, session core.Session) (core.Session, error)
	DeleteSession(ctx context.Context, project, id string) error

	// CUJ (Critical User Journey) operations
	CreateCUJ(ctx context.Context, cuj core.CriticalUserJourney) (core.CriticalUserJourney, error)
	GetCUJ(ctx context.Context, project, id string) (core.CriticalUserJourney, error)
	ListCUJs(ctx context.Context, project, specID string) ([]core.CriticalUserJourney, error)
	UpdateCUJ(ctx context.Context, cuj core.CriticalUserJourney) (core.CriticalUserJourney, error)
	DeleteCUJ(ctx context.Context, project, id string) error
	LinkCUJToFeature(ctx context.Context, project, cujID, featureID string) error
	UnlinkCUJFromFeature(ctx context.Context, project, cujID, featureID string) error
	GetCUJFeatureLinks(ctx context.Context, project, cujID string) ([]core.CUJFeatureLink, error)
}
