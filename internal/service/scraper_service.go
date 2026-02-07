package service

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"skill-sync/internal/repository"
)

type SearchParams struct {
	Title       string
	CompanyName string
	Location    string
	Skills      []string
	Limit       int
	Offset      int
}

func (p SearchParams) HasFilter() bool {
	if strings.TrimSpace(p.Title) != "" {
		return true
	}
	if strings.TrimSpace(p.CompanyName) != "" {
		return true
	}
	if strings.TrimSpace(p.Location) != "" {
		return true
	}
	for _, s := range p.Skills {
		if strings.TrimSpace(s) != "" {
			return true
		}
	}
	return false
}

type ScraperService interface {
	Search(ctx context.Context, params SearchParams) ([]repository.JobUpsert, error)
}

type scraperSource interface {
	Name() string
	Search(ctx context.Context, params SearchParams) ([]repository.JobUpsert, error)
}

type DefaultScraperService struct {
	sources []scraperSource
	logger  *log.Logger
}

func NewScraperService(logger *log.Logger, sources ...scraperSource) *DefaultScraperService {
	return &DefaultScraperService{sources: sources, logger: logger}
}

func (s *DefaultScraperService) Search(ctx context.Context, params SearchParams) ([]repository.JobUpsert, error) {
	if s == nil {
		return nil, nil
	}
	if !params.HasFilter() {
		return nil, nil
	}

	type res struct {
		source string
		jobs   []repository.JobUpsert
		err    error
	}

	outCh := make(chan res, len(s.sources))
	wg := sync.WaitGroup{}

	for _, src := range s.sources {
		src := src
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx2, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			jobs, err := src.Search(ctx2, params)
			outCh <- res{source: src.Name(), jobs: jobs, err: err}
		}()
	}

	wg.Wait()
	close(outCh)

	all := make([]repository.JobUpsert, 0)
	var okCount int
	var lastErr error

	for r := range outCh {
		if r.err != nil {
			lastErr = r.err
			if s.logger != nil {
				s.logger.Printf("scrape source=%s error=%v", r.source, r.err)
			}
			continue
		}
		okCount++
		if s.logger != nil {
			s.logger.Printf("scrape source=%s jobs=%d", r.source, len(r.jobs))
		}
		all = append(all, r.jobs...)
	}

	if okCount == 0 && lastErr != nil {
		return nil, lastErr
	}

	dedup := make(map[string]repository.JobUpsert)
	for _, j := range all {
		k := strings.ToLower(strings.TrimSpace(j.SourceName)) + "|" + strings.TrimSpace(j.SourceURL)
		if strings.TrimSpace(j.SourceURL) == "" {
			continue
		}
		if _, ok := dedup[k]; ok {
			continue
		}
		dedup[k] = j
	}

	out := make([]repository.JobUpsert, 0, len(dedup))
	for _, v := range dedup {
		out = append(out, v)
	}
	return out, nil
}

var _ ScraperService = (*DefaultScraperService)(nil)
