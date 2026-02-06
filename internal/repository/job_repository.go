package repository

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"
	"time"

	"skill-sync/internal/database"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	ErrJobNotFound = errors.New("job not found")
)

type JobRepository interface {
	ExistsByID(ctx context.Context, jobID uuid.UUID) (bool, error)
	ListJobs(ctx context.Context, limit, offset int) ([]Job, error)
	ListJobsForListing(ctx context.Context, f JobListFilter) ([]JobListRow, error)
	ListActiveJobsWithoutSkills(ctx context.Context, limit, offset int) ([]JobForSkillExtraction, error)
}

type Job struct {
	ID       uuid.UUID
	Title    string
	Company  string
	Location string
}

type JobForSkillExtraction struct {
	ID             uuid.UUID
	Title          string
	Description    string
	RawDescription string
}

type JobListFilter struct {
	Title       string
	CompanyName string
	Location    string
	Skills      []string
	Limit       int
	Offset      int
}

type JobListRow struct {
	ID          uuid.UUID
	Title       string
	Company     string
	Location    string
	Description string
	PostedAt    *time.Time
}

type PostgresJobRepository struct {
	db database.DB
}

func NewPostgresJobRepository(db database.DB) *PostgresJobRepository {
	return &PostgresJobRepository{db: db}
}

func (r *PostgresJobRepository) ExistsByID(ctx context.Context, jobID uuid.UUID) (bool, error) {
	var exists bool
	row := r.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM jobs WHERE id = $1)`, jobID)
	if err := row.Scan(&exists); err != nil {
		if err == sql.ErrNoRows || errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return exists, nil
}

func (r *PostgresJobRepository) ListJobs(ctx context.Context, limit, offset int) ([]Job, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.db.Query(ctx,
		`SELECT id, COALESCE(title, ''), COALESCE(company, ''), COALESCE(location, '')
		 FROM jobs
		 ORDER BY created_at DESC
		 LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]Job, 0)
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.ID, &j.Title, &j.Company, &j.Location); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *PostgresJobRepository) ListJobsForListing(ctx context.Context, f JobListFilter) ([]JobListRow, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	base := strings.Builder{}
	base.WriteString(`SELECT j.id,
		COALESCE(j.title, ''),
		COALESCE(j.company, ''),
		COALESCE(j.location, ''),
		COALESCE(j.description, ''),
		j.posted_at
		FROM jobs j
		WHERE 1=1`)

	args := make([]any, 0, 6)
	argN := 1

	if strings.TrimSpace(f.Title) != "" {
		base.WriteString(" AND j.title ILIKE $" + itoa(argN))
		args = append(args, "%"+strings.TrimSpace(f.Title)+"%")
		argN++
	}
	if strings.TrimSpace(f.CompanyName) != "" {
		base.WriteString(" AND j.company ILIKE $" + itoa(argN))
		args = append(args, "%"+strings.TrimSpace(f.CompanyName)+"%")
		argN++
	}
	if strings.TrimSpace(f.Location) != "" {
		base.WriteString(" AND j.location ILIKE $" + itoa(argN))
		args = append(args, "%"+strings.TrimSpace(f.Location)+"%")
		argN++
	}
	if len(f.Skills) > 0 {
		patterns := make([]string, 0, len(f.Skills))
		for _, s := range f.Skills {
			s = strings.TrimSpace(s)
			if s == "" {
				continue
			}
			patterns = append(patterns, "%"+s+"%")
		}
		if len(patterns) > 0 {
			base.WriteString(" AND EXISTS (")
			base.WriteString("SELECT 1 FROM job_skills js JOIN skills s ON s.id = js.skill_id ")
			base.WriteString("WHERE js.job_id = j.id AND s.name ILIKE ANY($" + itoa(argN) + ")")
			base.WriteString(")")
			args = append(args, patterns)
			argN++
		}
	}

	base.WriteString(" ORDER BY j.posted_at DESC NULLS LAST, j.created_at DESC")
	base.WriteString(" LIMIT $" + itoa(argN) + " OFFSET $" + itoa(argN+1))
	args = append(args, limit, offset)

	rows, err := r.db.Query(ctx, base.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]JobListRow, 0)
	for rows.Next() {
		var it JobListRow
		var posted sql.NullTime
		if err := rows.Scan(&it.ID, &it.Title, &it.Company, &it.Location, &it.Description, &posted); err != nil {
			return nil, err
		}
		if posted.Valid {
			t := posted.Time
			it.PostedAt = &t
		}
		out = append(out, it)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func itoa(i int) string {
	return strconv.Itoa(i)
}

func (r *PostgresJobRepository) ListActiveJobsWithoutSkills(ctx context.Context, limit, offset int) ([]JobForSkillExtraction, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 500 {
		limit = 500
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.db.Query(ctx,
		`SELECT j.id,
		        COALESCE(j.title, ''),
		        COALESCE(j.description, ''),
		        COALESCE(j.raw_description, '')
		 FROM jobs j
		 WHERE j.is_active = true
		   AND NOT EXISTS (SELECT 1 FROM job_skills js WHERE js.job_id = j.id)
		 ORDER BY j.scraped_at DESC NULLS LAST, j.created_at DESC
		 LIMIT $1 OFFSET $2`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]JobForSkillExtraction, 0)
	for rows.Next() {
		var j JobForSkillExtraction
		if err := rows.Scan(&j.ID, &j.Title, &j.Description, &j.RawDescription); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
