package core

import "time"

type EventType string

const (
	EventMessageCreated EventType = "message.created"
	EventMessageAck     EventType = "message.ack"
	EventMessageRead    EventType = "message.read"
	EventAgentHeartbeat EventType = "agent.heartbeat"
)

type Attachment struct {
	Name string
	Path string
}

type Message struct {
	ID          string
	ThreadID    string
	From        string
	To          []string
	Body        string
	Metadata    map[string]string
	Attachments []Attachment
	Importance  string
	AckRequired bool
	Status      string
	CreatedAt   time.Time
	Cursor      uint64
}

type Event struct {
	ID        string
	Type      EventType
	Agent     string
	Message   Message
	CreatedAt time.Time
	Cursor    uint64
}

type Agent struct {
	ID           string
	Name         string
	Project      string
	Capabilities []string
	Metadata     map[string]string
	Status       string
	LastSeen     time.Time
}
