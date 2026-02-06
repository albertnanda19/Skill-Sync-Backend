package pipeline

import (
	"context"
	"log"
	"regexp"
	"sort"
	"strings"
	"time"

	"skill-sync/internal/repository"

	"github.com/google/uuid"
)

type JobSkillExtractionPipeline struct {
	jobs  repository.JobRepository
	reqs  repository.JobRequiredSkillRepository
	log   *log.Logger
	limit int
}

func NewJobSkillExtractionPipeline(jobs repository.JobRepository, reqs repository.JobRequiredSkillRepository, logger *log.Logger) *JobSkillExtractionPipeline {
	if logger == nil {
		logger = log.Default()
	}
	return &JobSkillExtractionPipeline{jobs: jobs, reqs: reqs, log: logger, limit: 100}
}

type RunParams struct {
	Workers int
	Limit   int
}

type JobSkillExtractionResult struct {
	JobID      uuid.UUID
	SkillCount int
	Err        error
	Duration   time.Duration
}

func (p *JobSkillExtractionPipeline) Run(ctx context.Context, params RunParams) error {
	if p == nil || p.jobs == nil || p.reqs == nil {
		return nil
	}
	workers := params.Workers
	if workers <= 0 {
		workers = 5
	}
	limit := params.Limit
	if limit <= 0 {
		limit = p.limit
	}

	skillsByName, err := p.reqs.LoadSkillsByName(ctx)
	if err != nil {
		return err
	}

	offset := 0
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		batch, err := p.jobs.ListActiveJobsWithoutSkills(ctx, limit, offset)
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			return nil
		}

		pool := NewWorkerPool(workers, workers*2)
		results := pool.Run(ctx)

		for _, j := range batch {
			j := j
			pool.Submit(func(ctx context.Context) Result {
				start := time.Now()
				res := JobSkillExtractionResult{JobID: j.ID}
				defer func() { res.Duration = time.Since(start) }()

				reqs := extractJobRequirements(j, skillsByName)
				res.SkillCount = len(reqs)

				if err := p.reqs.UpsertForJob(ctx, j.ID, reqs); err != nil {
					res.Err = err
					p.log.Printf("pipeline=job_skill_extraction status=error job_id=%s skills=%d err=%v duration=%s", j.ID, res.SkillCount, err, res.Duration)
					return Result{Err: err}
				}

				p.log.Printf("pipeline=job_skill_extraction status=ok job_id=%s skills=%d duration=%s", j.ID, res.SkillCount, res.Duration)
				return Result{Err: nil}
			})
		}

		pool.Close()

		for r := range results {
			_ = r
		}

		offset += len(batch)
	}
}

func extractJobRequirements(j repository.JobForSkillExtraction, skillsByName map[string]uuid.UUID) []repository.JobRequiredSkillUpsert {
	text := strings.TrimSpace(j.Description)
	if text == "" {
		text = strings.TrimSpace(j.RawDescription)
	}
	if text == "" {
		text = strings.TrimSpace(j.Title)
	}
	if text == "" {
		return nil
	}
	lower := strings.ToLower(text)

	type hit struct {
		name  string
		id    uuid.UUID
		count int
	}

	hits := make([]hit, 0)
	for name, id := range skillsByName {
		n := strings.TrimSpace(name)
		if n == "" || id == uuid.Nil {
			continue
		}
		c := countSkillMention(lower, n)
		if c <= 0 {
			continue
		}
		hits = append(hits, hit{name: n, id: id, count: c})
	}

	if len(hits) == 0 {
		return nil
	}

	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].count == hits[j].count {
			return hits[i].name < hits[j].name
		}
		return hits[i].count > hits[j].count
	})

	out := make([]repository.JobRequiredSkillUpsert, 0, len(hits))
	seen := map[uuid.UUID]struct{}{}
	for _, h := range hits {
		if _, ok := seen[h.id]; ok {
			continue
		}
		seen[h.id] = struct{}{}

		lvl := importanceFromCount(h.count)
		mandatory := isMandatoryByContext(lower, h.name, lvl)
		years := yearsFromLevel(lvl)

		importance := lvl
		requiredLevel := lvl
		isMandatory := mandatory
		requiredYears := years

		out = append(out, repository.JobRequiredSkillUpsert{
			SkillID:          h.id,
			ImportanceWeight: importance,
			RequiredLevel:    &requiredLevel,
			IsMandatory:      &isMandatory,
			RequiredYears:    &requiredYears,
			SourceVersion:    int16(2),
		})
	}
	return out
}

func countSkillMention(textLower, skillName string) int {
	skillLower := strings.ToLower(strings.TrimSpace(skillName))
	if skillLower == "" {
		return 0
	}

	pat := `(?i)(^|[^a-z0-9])` + regexp.QuoteMeta(skillLower) + `([^a-z0-9]|$)`
	re := regexp.MustCompile(pat)
	m := re.FindAllStringIndex(textLower, -1)
	return len(m)
}

func importanceFromCount(count int) int {
	switch {
	case count >= 4:
		return 5
	case count == 3:
		return 4
	case count == 2:
		return 3
	default:
		return 2
	}
}

func yearsFromLevel(level int) int {
	if level < 1 {
		level = 1
	}
	if level > 5 {
		level = 5
	}
	switch level {
	case 5:
		return 3
	case 4:
		return 2
	case 3:
		return 1
	default:
		return 0
	}
}

func isMandatoryByContext(textLower, skillName string, level int) bool {
	if level >= 4 {
		return true
	}
	skillLower := strings.ToLower(strings.TrimSpace(skillName))
	if skillLower == "" {
		return false
	}

	idx := strings.Index(textLower, skillLower)
	if idx < 0 {
		return false
	}

	start := idx - 80
	if start < 0 {
		start = 0
	}
	end := idx + len(skillLower) + 80
	if end > len(textLower) {
		end = len(textLower)
	}
	window := textLower[start:end]

	markers := []string{"must", "required", "require", "mandatory", "need to", "needs", "minimum", "min."}
	for _, m := range markers {
		if strings.Contains(window, m) {
			return true
		}
	}
	return false
}
