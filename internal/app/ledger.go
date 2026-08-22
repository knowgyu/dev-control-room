package app

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/knowgyu/dev-control-room/internal/domain"
)

// eventLedger is a temporary spike adapter behind the application boundary.
// The durable SQLite event repository is intentionally Milestone 1 work.
type eventLedger struct {
	path string
	mu   sync.Mutex
}

func newEventLedger(home string) *eventLedger {
	return &eventLedger{path: filepath.Join(home, "events.jsonl")}
}

func (l *eventLedger) append(event domain.Event) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(l.path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	return json.NewEncoder(file).Encode(event)
}

func (l *eventLedger) recent(limit int) ([]domain.Event, error) {
	if limit <= 0 {
		return []domain.Event{}, nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	file, err := os.Open(l.path)
	if errors.Is(err, os.ErrNotExist) {
		return []domain.Event{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	events := make([]domain.Event, 0, limit)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 1<<20)
	for scanner.Scan() {
		var event domain.Event
		if json.Unmarshal(scanner.Bytes(), &event) == nil {
			events = append(events, event)
			if len(events) > limit {
				events = events[1:]
			}
		}
	}
	return events, scanner.Err()
}
