package qac

// Action identifies the allocation result selected by a policy.
type Action string

const (
	ActionContinue Action = "continue"
	ActionEscalate Action = "escalate"
	ActionRelease  Action = "release"
	ActionStop     Action = "stop"
)
