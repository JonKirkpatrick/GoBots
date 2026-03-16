package stadium

// StadiumEvent represents an event emitted by the manager for dashboard subscribers.
type StadiumEvent struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}
