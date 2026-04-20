package core

// WindowTarget identifies a tmux pane for a recipient agent.
// Declared in core (not livetransport) so HTTP handlers can pass targets
// around via the LiveDelivery interface without importing the tmux
// implementation package.
type WindowTarget struct {
	AgentID    string
	TmuxTarget string // e.g. "session:window.pane" or just "session"
}
