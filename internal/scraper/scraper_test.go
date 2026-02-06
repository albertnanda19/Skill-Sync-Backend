package scraper

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"skill-sync/internal/database"

	"github.com/google/uuid"
)

type fakeRow struct {
	vals []any
	err  error
}

func (r fakeRow) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.vals) {
		return fmt.Errorf("scan dest mismatch")
	}
	for i := range dest {
		switch d := dest[i].(type) {
		case *uuid.UUID:
			val, ok := r.vals[i].(uuid.UUID)
			if !ok {
				return fmt.Errorf("scan type mismatch uuid")
			}
			*d = val
		case *bool:
			val, ok := r.vals[i].(bool)
			if !ok {
				return fmt.Errorf("scan type mismatch bool")
			}
			*d = val
		default:
			return fmt.Errorf("unsupported scan type")
		}
	}
	return nil
}

type fakeDB struct {
	mu sync.Mutex

	sourcesByName map[string]uuid.UUID
	jobsByKey     map[string]rawJobInput
	scrapeRuns    map[uuid.UUID]string
}

func newFakeDB() *fakeDB {
	return &fakeDB{
		sourcesByName: map[string]uuid.UUID{},
		jobsByKey:     map[string]rawJobInput{},
		scrapeRuns:    map[uuid.UUID]string{},
	}
}

func (db *fakeDB) Ping(ctx context.Context) error { return nil }
func (db *fakeDB) Close() error                   { return nil }
func (db *fakeDB) SQLDB() *sql.DB                 { return nil }

func (db *fakeDB) Begin(ctx context.Context) (database.Tx, error) {
	return nil, fmt.Errorf("not implemented")
}

func (db *fakeDB) Exec(ctx context.Context, query string, args ...any) (int64, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	q := strings.ToLower(strings.TrimSpace(query))

	switch {
	case strings.HasPrefix(q, "insert into job_sources"):
		name := args[0].(string)
		if _, ok := db.sourcesByName[name]; !ok {
			db.sourcesByName[name] = uuid.New()
			return 1, nil
		}
		return 0, nil

	case strings.HasPrefix(q, "insert into scrape_runs"):
		runID := args[0].(uuid.UUID)
		db.scrapeRuns[runID] = "running"
		return 1, nil

	case strings.HasPrefix(q, "update scrape_runs"):
		runID := args[0].(uuid.UUID)
		status := args[2].(string)
		db.scrapeRuns[runID] = status
		return 1, nil

	case strings.HasPrefix(q, "insert into scrape_logs"):
		return 1, nil

	case strings.HasPrefix(q, "insert into jobs"):
		// args: id, source_id, external_job_id, title, company, location, employment_type,
		// description, raw_description, posted_at, scraped_at, url, is_active
		sourceID := args[1].(uuid.UUID)
		externalIDAny := args[2]
		externalID := ""
		if externalIDAny != nil {
			externalID = externalIDAny.(string)
		}
		urlAny := args[11]
		url := ""
		if urlAny != nil {
			url = urlAny.(string)
		}
		key := sourceID.String() + "|" + externalID
		if _, ok := db.jobsByKey[key]; ok {
			return 0, nil
		}
		in := rawJobInput{IsActive: true}
		if v := args[3]; v != nil {
			in.Title = v.(string)
		}
		if v := args[4]; v != nil {
			in.Company = v.(string)
		}
		if v := args[5]; v != nil {
			in.Location = v.(string)
		}
		in.ExternalJobID = externalID
		in.URL = url
		db.jobsByKey[key] = in
		return 1, nil
	default:
		return 0, nil
	}
}

func (db *fakeDB) Query(ctx context.Context, query string, args ...any) (database.Rows, error) {
	return nil, fmt.Errorf("not implemented")
}

func (db *fakeDB) QueryRow(ctx context.Context, query string, args ...any) database.Row {
	db.mu.Lock()
	defer db.mu.Unlock()

	q := strings.ToLower(strings.TrimSpace(query))

	switch {
	case strings.HasPrefix(q, "select id from job_sources"):
		name := args[0].(string)
		id, ok := db.sourcesByName[name]
		if !ok {
			return fakeRow{err: fmt.Errorf("no rows")}
		}
		return fakeRow{vals: []any{id}}

	case strings.HasPrefix(q, "select exists(select 1 from jobs"):
		sourceID := args[0].(uuid.UUID)
		url := args[1].(string)
		for k, v := range db.jobsByKey {
			_ = k
			if v.URL == url {
				return fakeRow{vals: []any{true}}
			}
		}
		_ = sourceID
		return fakeRow{vals: []any{false}}

	default:
		return fakeRow{err: fmt.Errorf("unsupported queryrow")}
	}
}

func TestDevtoScraper_SuccessAndIdempotent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/listings", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{
			"id": 1,
			"title": "Backend Engineer",
			"category": "jobs",
			"organization_name": "Acme",
			"company_name": "Acme",
			"location": "Remote",
			"published_at": "2025-01-01T00:00:00Z",
			"url": "https://dev.to/listings/jobs/backend-engineer"
		}]`))
	})
	mux.HandleFunc("/api/listings/1", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{
			"id": 1,
			"title": "Backend Engineer",
			"organization_name": "Acme",
			"company_name": "Acme",
			"location": "Remote",
			"published_at": "2025-01-01T00:00:00Z",
			"body_markdown": "job desc",
			"url": "https://dev.to/listings/jobs/backend-engineer"
		}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	db := newFakeDB()
	s := NewDevtoScraper(db)
	s.apiBase = server.URL
	s.siteBase = server.URL

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.Scrape(ctx, 1, 3); err != nil {
		t.Fatalf("scrape error: %v", err)
	}
	if err := s.Scrape(ctx, 1, 3); err != nil {
		t.Fatalf("scrape error (2nd): %v", err)
	}

	db.mu.Lock()
	defer db.mu.Unlock()
	if got := len(db.jobsByKey); got != 1 {
		t.Fatalf("expected 1 job inserted, got %d", got)
	}
	for _, j := range db.jobsByKey {
		if strings.TrimSpace(j.Title) == "" {
			t.Fatalf("expected non-empty title")
		}
		if strings.TrimSpace(j.URL) == "" {
			t.Fatalf("expected non-empty url")
		}
	}
}

func TestJobStreetScraper_SuccessAndIdempotent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/id/job-search/jobs", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><body><a href="/job/abc">Job</a></body></html>`))
	})
	mux.HandleFunc("/job/abc", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<html><head><title>Backend Go</title></head><body><h1>Backend Go</h1><div>desc</div></body></html>`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	db := newFakeDB()
	s := NewJobStreetScraperWithBaseURL(db, server.URL)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.Scrape(ctx, "", 1, 3); err != nil {
		t.Fatalf("scrape error: %v", err)
	}
	if err := s.Scrape(ctx, "", 1, 3); err != nil {
		t.Fatalf("scrape error (2nd): %v", err)
	}

	db.mu.Lock()
	defer db.mu.Unlock()
	if got := len(db.jobsByKey); got != 1 {
		t.Fatalf("expected 1 job inserted, got %d", got)
	}
	for _, j := range db.jobsByKey {
		if strings.TrimSpace(j.ExternalJobID) == "" {
			t.Fatalf("expected non-empty external id")
		}
		if !strings.Contains(j.URL, "/job/abc") {
			t.Fatalf("expected url to contain /job/abc, got %s", j.URL)
		}
	}
}
