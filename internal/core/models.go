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
	Project     string
	From        string
	To          []string
	CC          []string          // Carbon copy recipients
	BCC         []string          // Blind carbon copy recipients
	Subject     string            // Message subject line
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
	Project   string
	Message   Message
	CreatedAt time.Time
	Cursor    uint64
}

type Agent struct {
	ID           string
	SessionID    string
	Name         string
	Project      string
	Capabilities []string
	Metadata     map[string]string
	Status       string
	LastSeen     time.Time
	CreatedAt    time.Time
}

// RecipientStatus tracks read/ack status for a message recipient
type RecipientStatus struct {
	AgentID string     // Recipient agent name
	Kind    string     // to, cc, or bcc
	ReadAt  *time.Time // When the recipient read the message
	AckAt   *time.Time // When the recipient acknowledged the message
}

// IsRead returns true if the recipient has read the message
func (r *RecipientStatus) IsRead() bool { return r.ReadAt != nil }

// IsAcked returns true if the recipient has acknowledged the message
func (r *RecipientStatus) IsAcked() bool { return r.AckAt != nil }
