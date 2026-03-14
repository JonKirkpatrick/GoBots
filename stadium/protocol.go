package stadium

import "encoding/json"

// Response represents the standardized format for messages sent from the stadium manager to bots, including status, type, and payload.
type Response struct {
	Status  string `json:"status"`  // "ok", "err", "update"
	Type    string `json:"type"`    // "move", "gameover", "state", "info"
	Payload string `json:"payload"` // The actual message or data
}

// SendJSON marshals the Response struct into JSON and sends it to the bot's connection.
func (s *Session) SendJSON(res Response) {
	data, _ := json.Marshal(res)
	s.Conn.Write(append(data, '\n'))
}
