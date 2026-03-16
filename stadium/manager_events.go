package stadium

func (m *Manager) Subscribe() chan StadiumEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan StadiumEvent, 10)
	m.subscribers[ch] = struct{}{}
	return ch
}

func (m *Manager) Unsubscribe(ch chan StadiumEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.subscribers, ch)
	close(ch)
}

func (m *Manager) PublishArenaList() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.broadcastArenaListLocked()
}

func (m *Manager) broadcastArenaListLocked() {
	m.broadcastLocked("arena_list", m.listMatches())
	m.broadcastLocked("manager_state", m.snapshotLocked())
}

func (m *Manager) broadcastLocked(eventType string, payload interface{}) {
	event := StadiumEvent{Type: eventType, Payload: payload}
	for ch := range m.subscribers {
		select {
		case ch <- event:
		default:
			// Client is too slow; avoid blocking the whole server
		}
	}
}
