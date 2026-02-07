package ws

import (
	"encoding/json"
	"strings"
	"sync/atomic"
	"time"
)

type JobsUpdatedEvent struct {
	Type      string `json:"type"`
	Keyword   string `json:"keyword"`
	Source    string `json:"source"`
	Timestamp string `json:"timestamp"`
}

var defaultHub atomic.Pointer[Hub]

func SetDefaultHub(h *Hub) {
	defaultHub.Store(h)
}

func NotifyJobsUpdated(keyword string, source string) {
	h := defaultHub.Load()
	if h == nil {
		return
	}

	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return
	}

	evt := JobsUpdatedEvent{
		Type:      "jobs_updated",
		Keyword:   keyword,
		Source:    source,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	b, err := json.Marshal(evt)
	if err != nil {
		return
	}

	h.Broadcast(keyword, b)
}
