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
}

func (m *Manager) PublishArenaList() {
	m.mu.Lock()
	subscribers, events := m.prepareArenaListBroadcastLocked()
	m.mu.Unlock()
	m.publishEvents(subscribers, events)
}

func (m *Manager) prepareArenaListBroadcastLocked() ([]chan StadiumEvent, []StadiumEvent) {
	subscribers := make([]chan StadiumEvent, 0, len(m.subscribers))
	for ch := range m.subscribers {
		subscribers = append(subscribers, ch)
	}

	events := []StadiumEvent{
		{Type: "arena_list", Payload: m.listMatches()},
		{Type: "manager_state", Payload: m.snapshotLocked()},
	}

	return subscribers, events
}

func (m *Manager) publishEvents(subscribers []chan StadiumEvent, events []StadiumEvent) {
	for _, event := range events {
		for _, ch := range subscribers {
			select {
			case ch <- event:
			default:
				// Client is too slow; avoid blocking the whole server
			}
		}
	}
}
