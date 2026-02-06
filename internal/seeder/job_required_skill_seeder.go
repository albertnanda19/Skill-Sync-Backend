package seeder

import (
	"context"
	"math/rand"
	"strings"
	"time"

	"skill-sync/internal/database"

	"github.com/google/uuid"
)

type JobRequiredSkillSeeder struct{}

func (JobRequiredSkillSeeder) Name() string { return "job_required_skills_v2" }

func (JobRequiredSkillSeeder) Run(ctx context.Context, db database.DB) error {
	if err := ensureTableColumns(ctx, db, "skills", "id", "name", "category", "created_at"); err != nil {
		return err
	}
	if err := ensureTableColumns(ctx, db, "jobs",
		"id",
		"source_id",
		"external_job_id",
		"title",
		"company",
		"location",
		"employment_type",
		"description",
		"raw_description",
		"posted_at",
		"scraped_at",
		"created_at",
	); err != nil {
		return err
	}
	if err := ensureTableColumns(ctx, db, "job_skills",
		"id",
		"job_id",
		"skill_id",
		"importance_weight",
		"required_level",
		"is_mandatory",
		"required_years",
		"source_version",
	); err != nil {
		return err
	}

	skillsByName, err := loadSkillsByName(ctx, db)
	if err != nil {
		return err
	}

	jobs, err := loadSeededJobs(ctx, db)
	if err != nil {
		return err
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	_ = time.Now().UTC()

	for _, j := range jobs {
		hasReq, err := jobHasAnyRequirements(ctx, db, j.ID)
		if err != nil {
			continue
		}
		if hasReq {
			continue
		}

		reqs := buildRequirementsForJob(j.Title, skillsByName, rng)
		if len(reqs) == 0 {
			continue
		}

		for _, r := range reqs {
			_, err := db.Exec(ctx,
				`INSERT INTO job_skills (
					id, job_id, skill_id, importance_weight, required_level, is_mandatory, required_years, source_version
				)
				VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
				ON CONFLICT (job_id, skill_id) DO NOTHING`,
				uuid.New(),
				j.ID,
				r.SkillID,
				r.ImportanceWeight,
				r.RequiredLevel,
				r.IsMandatory,
				r.RequiredYears,
				int16(1),
			)
			if err != nil {
				continue
			}
		}
	}

	return nil
}

type seededJob struct {
	ID    uuid.UUID
	Title string
}

type jobRequirementSeed struct {
	SkillID          uuid.UUID
	RequiredLevel    int
	IsMandatory      bool
	RequiredYears    int
	ImportanceWeight int
}

func loadSkillsByName(ctx context.Context, db database.DB) (map[string]uuid.UUID, error) {
	rows, err := db.Query(ctx, `SELECT id, name FROM skills ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[name] = id
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func loadSeededJobs(ctx context.Context, db database.DB) ([]seededJob, error) {
	rows, err := db.Query(ctx, `SELECT id, title FROM jobs WHERE external_job_id LIKE 'seed-%' ORDER BY title ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]seededJob, 0)
	for rows.Next() {
		var j seededJob
		if err := rows.Scan(&j.ID, &j.Title); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func jobHasAnyRequirements(ctx context.Context, db database.DB, jobID uuid.UUID) (bool, error) {
	row := db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM job_skills WHERE job_id = $1)`, jobID)
	var exists bool
	if err := row.Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

func buildRequirementsForJob(title string, skills map[string]uuid.UUID, rng *rand.Rand) []jobRequirementSeed {
	add := func(out []jobRequirementSeed, name string, level int, mandatory bool, years int, weight int) []jobRequirementSeed {
		id, ok := skills[name]
		if !ok {
			return out
		}
		for _, it := range out {
			if it.SkillID == id {
				return out
			}
		}
		return append(out, jobRequirementSeed{
			SkillID:          id,
			RequiredLevel:    clamp(level, 1, 5),
			IsMandatory:      mandatory,
			RequiredYears:    clamp(years, 0, 5),
			ImportanceWeight: clamp(weight, 1, 5),
		})
	}

	pickLevel := func(min, max int) int { return min + rng.Intn(max-min+1) }
	pickYears := func(min, max int) int { return min + rng.Intn(max-min+1) }

	out := make([]jobRequirementSeed, 0, 6)

	// fallback pool for optional additions from existing seeded skills
	optionalPool := []string{"Redis", "Kubernetes", "AWS", "GCP", "Docker", "PostgreSQL", "Go", "JavaScript", "TypeScript"}
	addRandomOptional := func(out []jobRequirementSeed, n int) []jobRequirementSeed {
		if n <= 0 {
			return out
		}
		for tries := 0; tries < 20 && n > 0; tries++ {
			name := optionalPool[rng.Intn(len(optionalPool))]
			before := len(out)
			out = add(out, name, pickLevel(1, 4), false, pickYears(0, 3), pickLevel(1, 4))
			if len(out) > before {
				n--
			}
		}
		return out
	}

	switch {
	case hasAny(title, "backend", "go"):
		out = add(out, "Go", 4, true, 2, 5)
		out = add(out, "PostgreSQL", 3, true, 1, 4)
		out = add(out, "Docker", 3, true, 1, 4)
		out = add(out, "Redis", pickLevel(2, 4), false, pickYears(0, 2), 3)
		out = add(out, "Kubernetes", pickLevel(2, 4), false, pickYears(0, 2), 3)
		out = add(out, "AWS", pickLevel(2, 4), false, pickYears(0, 2), 3)
		out = addRandomOptional(out, 1)

	case hasAny(title, "fullstack"):
		out = add(out, "JavaScript", 4, true, 2, 5)
		out = add(out, "TypeScript", 3, true, 1, 4)
		out = add(out, "Go", 3, false, 1, 3)
		out = add(out, "PostgreSQL", 3, true, 1, 4)
		out = add(out, "Docker", 2, false, 0, 2)
		out = add(out, "Redis", 2, false, 0, 2)

	case hasAny(title, "devops"):
		out = add(out, "Docker", 4, true, 2, 5)
		out = add(out, "Kubernetes", 4, true, 2, 5)
		out = add(out, "AWS", 3, true, 1, 4)
		out = add(out, "GCP", 2, false, 0, 2)
		out = add(out, "PostgreSQL", 2, false, 0, 2)

	case hasAny(title, "data"):
		out = add(out, "PostgreSQL", 4, true, 2, 5)
		out = add(out, "AWS", 3, false, 1, 3)
		out = add(out, "GCP", 2, false, 0, 2)
		out = addRandomOptional(out, 1)

	case hasAny(title, "mobile", "react native"):
		out = add(out, "JavaScript", 4, true, 2, 5)
		out = add(out, "TypeScript", 3, true, 1, 4)
		out = add(out, "Docker", 2, false, 0, 2)
		out = addRandomOptional(out, 1)

	case hasAny(title, "qa"):
		out = add(out, "JavaScript", 3, true, 1, 4)
		out = add(out, "PostgreSQL", 2, false, 0, 2)
		out = add(out, "Docker", 2, false, 0, 2)
		out = addRandomOptional(out, 1)

	case hasAny(title, "sre"):
		out = add(out, "Kubernetes", 4, true, 2, 5)
		out = add(out, "Docker", 4, true, 2, 5)
		out = add(out, "AWS", 3, true, 1, 4)
		out = add(out, "Redis", 2, false, 0, 2)
		out = addRandomOptional(out, 1)

	case hasAny(title, "typescript"):
		out = add(out, "TypeScript", 4, true, 2, 5)
		out = add(out, "JavaScript", 3, true, 1, 4)
		out = add(out, "PostgreSQL", 2, false, 0, 2)
		out = add(out, "Docker", 2, false, 0, 2)
		out = addRandomOptional(out, 1)

	default:
		out = addRandomOptional(out, 3)
	}

	if len(out) < 2 {
		out = addRandomOptional(out, 2)
	}
	return out
}

func hasAny(s string, subs ...string) bool {
	s = strings.ToLower(s)
	for _, sub := range subs {
		if sub != "" && strings.Contains(s, strings.ToLower(sub)) {
			return true
		}
	}
	return false
}

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
