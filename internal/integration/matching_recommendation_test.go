package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"skill-sync/internal/config"
	"skill-sync/internal/database"
	"skill-sync/internal/database/migration"
	dbpostgres "skill-sync/internal/database/postgres"
	"skill-sync/internal/delivery/http/middleware"
	"skill-sync/internal/delivery/http/routes"
	"skill-sync/internal/repository"
	"skill-sync/internal/usecase"

	"skill-sync/internal/domain/matching"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type semanticResponse struct {
	Status  int             `json:"status"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type loginData struct {
	AccessToken string `json:"access_token"`
}

type recommendationItem struct {
	JobID            uuid.UUID                 `json:"job_id"`
	Title            string                    `json:"title"`
	CompanyName      string                    `json:"company_name"`
	Location         string                    `json:"location"`
	MatchScore       int                       `json:"match_score"`
	MandatoryMissing bool                      `json:"mandatory_missing"`
	MissingSkills    []recommendationSkillItem `json:"missing_skills"`
}

type recommendationSkillItem struct {
	SkillID     uuid.UUID `json:"skill_id"`
	SkillName   string    `json:"skill_name"`
	IsMandatory bool      `json:"is_mandatory"`
}

func TestIntegration_Login_MatchingV2_Recommendations(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	db := connectTestDB(t, ctx)
	defer func() { _ = db.Close() }()

	runMigrations(t, ctx, db)

	seed := seedDummyData(t, ctx, db)
	defer cleanupSeed(t, ctx, db, seed)

	app := newTestFiberApp(t, seed.cfg, db)

	tok := loginAndGetJWT(t, app)
	if tok == "" {
		t.Fatalf("login: empty access_token")
	}

	matchingV2UC := usecase.NewMatchingUsecaseV2(
		repository.NewPostgresJobRepository(db),
		repository.NewPostgresJobSkillV2Repository(db),
		repository.NewPostgresUserSkillRepository(db),
	)

	resV2, err := matchingV2UC.CalculateMatchV2(ctx, seed.userID, seed.jobV2ID)
	if err != nil {
		t.Fatalf("matching v2 error: %v", err)
	}
	if resV2.MatchScore < 0 || resV2.MatchScore > 100 {
		t.Fatalf("matching v2: expected match_score 0-100, got %d", resV2.MatchScore)
	}
	if len(resV2.MatchedSkills) == 0 {
		t.Fatalf("matching v2: expected matched_skills not empty")
	}
	if !containsSkillNameV2(resV2.MatchedSkills, "Go") {
		t.Fatalf("matching v2: expected matched_skills to contain Go")
	}
	if !resV2.MandatoryMissing {
		t.Fatalf("matching v2: expected mandatory_missing=true")
	}
	if !containsMissingSkillNameV2(resV2.MissingSkills, "Docker", true) {
		t.Fatalf("matching v2: expected missing_skills to contain mandatory Docker")
	}

	resFallback, err := matchingV2UC.CalculateMatchV2(ctx, seed.userID, seed.jobFallbackID)
	if err != nil {
		t.Fatalf("matching v2 fallback error: %v", err)
	}
	if resFallback.MatchScore < 0 || resFallback.MatchScore > 100 {
		t.Fatalf("matching v2 fallback: expected match_score 0-100, got %d", resFallback.MatchScore)
	}

	recs := callRecommendations(t, app, tok)
	if len(recs) == 0 {
		t.Fatalf("recommendations: expected non-empty array")
	}

	assertNoDuplicateJobs(t, recs)
	assertSortedByScoreDesc(t, recs)

	found := false
	for _, it := range recs {
		if it.JobID == seed.jobV2ID {
			found = true
			if !it.MandatoryMissing {
				t.Fatalf("recommendations: jobV2 expected mandatory_missing=true")
			}
			if !containsMissingSkillName(it.MissingSkills, "Docker", true) {
				t.Fatalf("recommendations: jobV2 expected missing_skills include mandatory Docker")
			}
			break
		}
	}
	if !found {
		t.Fatalf("recommendations: expected seeded job to appear in response")
	}
}

func connectTestDB(t *testing.T, ctx context.Context) database.DB {
	t.Helper()

	host := stringsOrDefault(os.Getenv("SKILLSYNC_TEST_DB_HOST"), os.Getenv("DB_HOST"))
	port := stringsOrDefault(os.Getenv("SKILLSYNC_TEST_DB_PORT"), os.Getenv("DB_PORT"))
	name := stringsOrDefault(os.Getenv("SKILLSYNC_TEST_DB_NAME"), os.Getenv("DB_NAME"))
	user := stringsOrDefault(os.Getenv("SKILLSYNC_TEST_DB_USER"), os.Getenv("DB_USER"))
	pass := stringsOrDefault(os.Getenv("SKILLSYNC_TEST_DB_PASSWORD"), os.Getenv("DB_PASSWORD"))
	ssl := stringsOrDefault(os.Getenv("SKILLSYNC_TEST_DB_SSL_MODE"), os.Getenv("DB_SSL_MODE"))

	if host == "" || port == "" || name == "" || user == "" {
		t.Skip("missing test DB env vars: set SKILLSYNC_TEST_DB_HOST/PORT/NAME/USER/PASSWORD (or DB_HOST/DB_PORT/DB_NAME/DB_USER/DB_PASSWORD)")
	}
	if ssl == "" {
		ssl = "disable"
	}

	dbcfg := config.DatabaseConfig{
		DBHost:     host,
		DBPort:     port,
		DBName:     name,
		DBUser:     user,
		DBPassword: pass,
		DBSSLMode:  ssl,
	}

	db, err := dbpostgres.Connect(ctx, dbcfg)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	return db
}

func runMigrations(t *testing.T, ctx context.Context, db database.DB) {
	t.Helper()

	migDir := resolveMigrationsDir(t)
	r := migration.Runner{Dir: migDir}
	if err := r.Run(ctx, db.SQLDB()); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
}

func resolveMigrationsDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("resolve migrations dir: runtime.Caller failed")
	}

	// this file: internal/integration/matching_recommendation_test.go
	// backend root: ../../
	backendRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	migDir := filepath.Join(backendRoot, "migrations")

	if st, err := os.Stat(migDir); err != nil || !st.IsDir() {
		t.Fatalf("resolve migrations dir: not found or not a dir: %s", migDir)
	}
	files, _ := filepath.Glob(filepath.Join(migDir, "V*__*.sql"))
	if len(files) == 0 {
		t.Fatalf("resolve migrations dir: no migration files found in %s", migDir)
	}

	return migDir
}

type seededIDs struct {
	cfg           config.Config
	userID        uuid.UUID
	jobV2ID       uuid.UUID
	jobFallbackID uuid.UUID
	sourceID      uuid.UUID
	skillIDs      map[string]uuid.UUID
}

func seedDummyData(t *testing.T, ctx context.Context, db database.DB) seededIDs {
	t.Helper()

	jwtAccessSecret := stringsOrDefault(os.Getenv("SKILLSYNC_TEST_JWT_ACCESS_SECRET"), "test-access-secret")
	jwtRefreshSecret := stringsOrDefault(os.Getenv("SKILLSYNC_TEST_JWT_REFRESH_SECRET"), "test-refresh-secret")

	out := seededIDs{
		cfg: config.Config{
			App:      config.AppConfig{AppName: "SkillSync", Environment: "test", HTTPPort: "0"},
			Database: config.DatabaseConfig{RunSeeders: false},
			JWT: config.JWTConfig{
				AccessSecret:     jwtAccessSecret,
				RefreshSecret:    jwtRefreshSecret,
				AccessExpiresIn:  15 * time.Minute,
				RefreshExpiresIn: 24 * time.Hour,
			},
		},
		skillIDs: map[string]uuid.UUID{},
	}

	out.sourceID = ensureJobSource(t, ctx, db, "IT-Test")

	out.skillIDs["Go"] = ensureSkill(t, ctx, db, "Go")
	out.skillIDs["PostgreSQL"] = ensureSkill(t, ctx, db, "PostgreSQL")
	out.skillIDs["Docker"] = ensureSkill(t, ctx, db, "Docker")
	out.skillIDs["Redis"] = ensureSkill(t, ctx, db, "Redis")

	out.jobV2ID = ensureJob(t, ctx, db, out.sourceID, "it-test-job-v2", "Backend Engineer (Go) - IT", "IT Co", "Jakarta")
	out.jobFallbackID = ensureJob(t, ctx, db, out.sourceID, "it-test-job-fallback", "Backend Engineer (Fallback) - IT", "IT Co", "Remote")

	ensureJobSkillV2(t, ctx, db, out.jobV2ID, out.skillIDs["Go"], 5, ptrInt(4), ptrBool(true), ptrInt(2), int16(1))
	ensureJobSkillV2(t, ctx, db, out.jobV2ID, out.skillIDs["PostgreSQL"], 4, ptrInt(3), ptrBool(true), ptrInt(1), int16(1))
	ensureJobSkillV2(t, ctx, db, out.jobV2ID, out.skillIDs["Docker"], 4, ptrInt(4), ptrBool(true), ptrInt(1), int16(1))
	ensureJobSkillV2(t, ctx, db, out.jobV2ID, out.skillIDs["Redis"], 3, ptrInt(2), ptrBool(false), ptrInt(0), int16(1))

	ensureJobSkillV2(t, ctx, db, out.jobFallbackID, out.skillIDs["Go"], 5, nil, nil, nil, int16(1))
	ensureJobSkillV2(t, ctx, db, out.jobFallbackID, out.skillIDs["Docker"], 4, nil, nil, nil, int16(1))

	out.userID = ensureUser(t, ctx, db, "user@example.com", "password")

	ensureUserSkill(t, ctx, db, out.userID, out.skillIDs["Go"], 5, 3)
	ensureUserSkill(t, ctx, db, out.userID, out.skillIDs["PostgreSQL"], 4, 2)
	ensureUserSkill(t, ctx, db, out.userID, out.skillIDs["Redis"], 3, 1)

	return out
}

func cleanupSeed(t *testing.T, ctx context.Context, db database.DB, seed seededIDs) {
	t.Helper()

	_, _ = db.Exec(ctx, `DELETE FROM user_skills WHERE user_id = $1`, seed.userID)
	_, _ = db.Exec(ctx, `DELETE FROM users WHERE id = $1`, seed.userID)
	_, _ = db.Exec(ctx, `DELETE FROM job_skills WHERE job_id = $1 OR job_id = $2`, seed.jobV2ID, seed.jobFallbackID)
	_, _ = db.Exec(ctx, `DELETE FROM jobs WHERE id = $1 OR id = $2`, seed.jobV2ID, seed.jobFallbackID)
	_, _ = db.Exec(ctx, `DELETE FROM job_sources WHERE id = $1`, seed.sourceID)
}

func newTestFiberApp(t *testing.T, cfg config.Config, db database.DB) *fiber.App {
	t.Helper()

	app := fiber.New(fiber.Config{})
	errMw := middleware.NewErrorMiddleware()
	app.Use(errMw.Middleware())

	routes.NewRegistry(cfg, db).Register(app)
	return app
}

func loginAndGetJWT(t *testing.T, app *fiber.App) string {
	t.Helper()

	body := map[string]string{"email": "user@example.com", "password": "password"}
	b, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("login request error: %v", err)
	}
	defer resp.Body.Close()

	var sr semanticResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		t.Fatalf("login decode error: %v", err)
	}
	if sr.Status != 200 {
		t.Fatalf("login: expected status=200, got %d (message=%s)", sr.Status, sr.Message)
	}
	if sr.Message != "ok" {
		t.Fatalf("login: expected message=ok, got %s", sr.Message)
	}
	if len(sr.Data) == 0 {
		t.Fatalf("login: expected non-empty data")
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(sr.Data, &m); err != nil {
		t.Fatalf("login: data unmarshal error: %v", err)
	}
	var ld loginData
	if raw, ok := m["access_token"]; ok {
		_ = json.Unmarshal(raw, &ld.AccessToken)
	}
	if ld.AccessToken == "" {
		t.Fatalf("login: missing access_token")
	}
	return ld.AccessToken
}

func callRecommendations(t *testing.T, app *fiber.App, jwt string) []recommendationItem {
	t.Helper()

	req := httptest.NewRequest("GET", "/api/v1/jobs/recommendations?limit=5&offset=0", nil)
	req.Header.Set("Authorization", "Bearer "+jwt)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("recommendations request error: %v", err)
	}
	defer resp.Body.Close()

	var sr semanticResponse
	if err := json.NewDecoder(resp.Body).Decode(&sr); err != nil {
		t.Fatalf("recommendations decode error: %v", err)
	}
	if sr.Status != 200 {
		t.Fatalf("recommendations: expected status=200, got %d (message=%s)", sr.Status, sr.Message)
	}
	if sr.Message != "ok" {
		t.Fatalf("recommendations: expected message=ok, got %s", sr.Message)
	}

	var items []recommendationItem
	if err := json.Unmarshal(sr.Data, &items); err != nil {
		t.Fatalf("recommendations: data unmarshal error: %v", err)
	}
	return items
}

func assertSortedByScoreDesc(t *testing.T, items []recommendationItem) {
	t.Helper()

	for i := 1; i < len(items); i++ {
		if items[i].MatchScore > items[i-1].MatchScore {
			t.Fatalf("recommendations: expected match_score descending at idx=%d: prev=%d cur=%d", i, items[i-1].MatchScore, items[i].MatchScore)
		}
	}
}

func assertNoDuplicateJobs(t *testing.T, items []recommendationItem) {
	t.Helper()

	seen := map[uuid.UUID]struct{}{}
	for i, it := range items {
		if it.JobID == uuid.Nil {
			t.Fatalf("recommendations: idx=%d has nil job_id", i)
		}
		if _, ok := seen[it.JobID]; ok {
			t.Fatalf("recommendations: duplicate job_id=%s", it.JobID)
		}
		seen[it.JobID] = struct{}{}
	}
}

func containsMissingSkillName(items []recommendationSkillItem, name string, mandatory bool) bool {
	for _, it := range items {
		if it.SkillName == name && it.IsMandatory == mandatory {
			return true
		}
	}
	return false
}

func containsSkillNameV2(items []matching.MatchedSkillV2, name string) bool {
	for _, it := range items {
		if it.SkillName == name {
			return true
		}
	}
	return false
}

func containsMissingSkillNameV2(items []matching.MissingSkillV2, name string, mandatory bool) bool {
	for _, it := range items {
		if it.SkillName == name && it.IsMandatory == mandatory {
			return true
		}
	}
	return false
}

func ensureJobSource(t *testing.T, ctx context.Context, db database.DB, name string) uuid.UUID {
	t.Helper()

	id := uuid.New()
	_, err := db.Exec(ctx,
		`INSERT INTO job_sources (id, name, base_url) VALUES ($1,$2,$3)
		 ON CONFLICT (name) DO NOTHING`,
		id, name, "https://example.test",
	)
	if err != nil {
		t.Fatalf("seed job_source: %v", err)
	}

	row := db.QueryRow(ctx, `SELECT id FROM job_sources WHERE name = $1 LIMIT 1`, name)
	var got uuid.UUID
	if err := row.Scan(&got); err != nil {
		t.Fatalf("seed job_source select: %v", err)
	}
	return got
}

func ensureSkill(t *testing.T, ctx context.Context, db database.DB, name string) uuid.UUID {
	t.Helper()

	id := uuid.New()
	_, err := db.Exec(ctx,
		`INSERT INTO skills (id, name, category) VALUES ($1,$2,$3)
		 ON CONFLICT (name) DO NOTHING`,
		id, name, "IT",
	)
	if err != nil {
		t.Fatalf("seed skill %s: %v", name, err)
	}

	row := db.QueryRow(ctx, `SELECT id FROM skills WHERE name = $1 LIMIT 1`, name)
	var got uuid.UUID
	if err := row.Scan(&got); err != nil {
		t.Fatalf("seed skill select %s: %v", name, err)
	}
	return got
}

func ensureJob(t *testing.T, ctx context.Context, db database.DB, sourceID uuid.UUID, externalID, title, company, location string) uuid.UUID {
	t.Helper()

	id := uuid.New()
	_, err := db.Exec(ctx,
		`INSERT INTO jobs (id, source_id, external_job_id, title, company, location, description, raw_description, posted_at, scraped_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,now(),now())
		 ON CONFLICT (source_id, external_job_id) DO NOTHING`,
		id, sourceID, externalID, title, company, location, "desc", "desc",
	)
	if err != nil {
		t.Fatalf("seed job %s: %v", externalID, err)
	}

	row := db.QueryRow(ctx, `SELECT id FROM jobs WHERE source_id = $1 AND external_job_id = $2 LIMIT 1`, sourceID, externalID)
	var got uuid.UUID
	if err := row.Scan(&got); err != nil {
		t.Fatalf("seed job select %s: %v", externalID, err)
	}
	return got
}

func ensureJobSkillV2(t *testing.T, ctx context.Context, db database.DB, jobID, skillID uuid.UUID, importance int, requiredLevel *int, isMandatory *bool, requiredYears *int, sourceVersion int16) {
	t.Helper()

	_, err := db.Exec(ctx,
		`INSERT INTO job_skills (id, job_id, skill_id, importance_weight, required_level, is_mandatory, required_years, source_version)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 ON CONFLICT (job_id, skill_id) DO UPDATE SET
			importance_weight = EXCLUDED.importance_weight,
			required_level = EXCLUDED.required_level,
			is_mandatory = EXCLUDED.is_mandatory,
			required_years = EXCLUDED.required_years,
			source_version = EXCLUDED.source_version`,
		uuid.New(), jobID, skillID, importance, requiredLevel, isMandatory, requiredYears, sourceVersion,
	)
	if err != nil {
		t.Fatalf("seed job_skill: %v", err)
	}
}

func ensureUser(t *testing.T, ctx context.Context, db database.DB, email, password string) uuid.UUID {
	t.Helper()

	pwHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("seed user: bcrypt error: %v", err)
	}

	id := uuid.New()
	_, err = db.Exec(ctx,
		`INSERT INTO users (id, email, password_hash) VALUES ($1,$2,$3)
		 ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash`,
		id, email, string(pwHash),
	)
	if err != nil {
		t.Fatalf("seed user insert: %v", err)
	}

	row := db.QueryRow(ctx, `SELECT id FROM users WHERE email = $1 LIMIT 1`, email)
	var got uuid.UUID
	if err := row.Scan(&got); err != nil {
		t.Fatalf("seed user select: %v", err)
	}
	return got
}

func ensureUserSkill(t *testing.T, ctx context.Context, db database.DB, userID, skillID uuid.UUID, level, years int) {
	t.Helper()

	id := uuid.New()
	_, err := db.Exec(ctx,
		`INSERT INTO user_skills (id, user_id, skill_id, proficiency_level, years_experience)
		 VALUES ($1,$2,$3,$4,$5)
		 ON CONFLICT (user_id, skill_id) DO UPDATE SET
			proficiency_level = EXCLUDED.proficiency_level,
			years_experience = EXCLUDED.years_experience`,
		id, userID, skillID, level, years,
	)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("seed user_skill timeout")
		}
		t.Fatalf("seed user_skill: %v", err)
	}
}

func ptrInt(v int) *int    { return &v }
func ptrBool(v bool) *bool { return &v }

func stringsOrDefault(v, def string) string {
	if v != "" {
		return v
	}
	return def
}
