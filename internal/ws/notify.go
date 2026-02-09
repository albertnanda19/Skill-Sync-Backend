package ws

import (
	"encoding/json"
	"strings"
	"sync/atomic"
	"time"
)

type JobsUpdatedEvent struct {
	Type            string `json:"type"`
	Keyword         string `json:"keyword"`
	Source          string `json:"source"`
	Timestamp       string `json:"timestamp"`
	HasNewData      *bool  `json:"has_new_data,omitempty"`
	MaxJobCreatedAt string `json:"max_job_created_at,omitempty"`
}

var defaultHub atomic.Pointer[Hub]

var onZeroClients atomic.Value

func SetDefaultHub(h *Hub) {
	defaultHub.Store(h)
}

func SetOnZeroClients(fn func(keyword string)) {
	onZeroClients.Store(fn)
}

func getOnZeroClients() func(keyword string) {
	v := onZeroClients.Load()
	if v == nil {
		return nil
	}
	fn, _ := v.(func(string))
	return fn
}

func NotifyJobsUpdated(keyword string, source string) {
	NotifyJobsUpdatedWithState(keyword, source, nil, time.Time{})
}

func NotifyJobsUpdatedWithState(keyword string, source string, hasNewData *bool, maxJobCreatedAt time.Time) {
	h := defaultHub.Load()
	if h == nil {
		return
	}

	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return
	}

	evt := JobsUpdatedEvent{
		Type:       "jobs_updated",
		Keyword:    keyword,
		Source:     source,
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		HasNewData: hasNewData,
	}
	if !maxJobCreatedAt.IsZero() {
		evt.MaxJobCreatedAt = maxJobCreatedAt.UTC().Format(time.RFC3339Nano)
	}
	b, err := json.Marshal(evt)
	if err != nil {
		return
	}

	h.Broadcast(keyword, b)
}
