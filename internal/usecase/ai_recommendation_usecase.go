package usecase

import (
	"context"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"skill-sync/internal/ai"
	"skill-sync/internal/domain/user"
	"skill-sync/internal/repository"

	"github.com/google/uuid"
)

type AIRecommendationUsecase interface {
	GetAIRecommendations(ctx context.Context, userID uuid.UUID) ([]AIJobRecommendationItem, error)
}

type AIJobRecommendationItem struct {
	JobID       uuid.UUID
	Title       string
	CompanyName string
	Location    string
	JobURL      string
	Source      string
	MatchScore  int
	MatchReason string
}

type AIRecommendation struct {
	jobs       repository.JobRepository
	userSkills repository.UserSkillRepository
	users      user.Repository
	provider   ai.Provider
	cache      *AIRecommendationCache
}

func NewAIRecommendationUsecase(jobs repository.JobRepository, userSkills repository.UserSkillRepository, users user.Repository, provider ai.Provider, cache *AIRecommendationCache) *AIRecommendation {
	return &AIRecommendation{jobs: jobs, userSkills: userSkills, users: users, provider: provider, cache: cache}
}

func (u *AIRecommendation) GetAIRecommendations(ctx context.Context, userID uuid.UUID) ([]AIJobRecommendationItem, error) {
	if userID == uuid.Nil {
		return nil, ErrUnauthorized
	}

	if !optBoolEnv("AI_RECOMMENDATION_ENABLED") {
		// feature disabled -> fallback recent jobs
		return u.fallbackRecentJobs(ctx, nil)
	}

	maxCandidates := optIntEnv("AI_RECOMMENDATION_MAX_CANDIDATES", 50)
	if maxCandidates <= 0 {
		maxCandidates = 50
	}
	if maxCandidates > 50 {
		maxCandidates = 50
	}
	topK := optIntEnv("AI_RECOMMENDATION_TOP_K", 20)
	if topK <= 0 {
		topK = 20
	}
	if topK > 20 {
		topK = 20
	}

	// Load user profile context
	email := ""
	if u.users != nil {
		usr, err := u.users.GetUserByID(ctx, userID)
		if err == nil {
			email = strings.TrimSpace(usr.Email)
		}
	}

	profileBits := make([]string, 0, 3)
	if email != "" {
		profileBits = append(profileBits, "Email: "+email)
	}

	skills, err := u.userSkills.FindByUserID(ctx, userID)
	if err != nil {
		return u.fallbackRecentJobs(ctx, nil)
	}

	skillsUpdatedAt, err := u.userSkills.GetSkillsUpdatedAt(ctx, userID)
	if err != nil {
		skillsUpdatedAt = time.Time{}
	}
	skillKeywords := make([]string, 0, len(skills))
	if len(skills) > 0 {
		names := make([]string, 0, len(skills))
		for _, s := range skills {
			if strings.TrimSpace(s.SkillName) == "" {
				continue
			}
			n := strings.TrimSpace(s.SkillName)
			names = append(names, n)
			skillKeywords = append(skillKeywords, n)
		}
		if len(names) > 0 {
			profileBits = append(profileBits, "Skills: "+strings.Join(uniqueStrings(names), ", "))
		}
	}

	userProfile := strings.Join(profileBits, "\n")
	if strings.TrimSpace(userProfile) == "" {
		userProfile = "Skills: (unknown)"
	}

	// Cache layer (between endpoint and AI provider)
	cacheKey := ""
	if u.cache != nil && IsAICacheEnabled() {
		hash := BuildUserProfileHash(userID, uniqueStrings(skillKeywords), skillsUpdatedAt)
		cacheKey = BuildAIRecommendationCacheKey(userID, hash)
		if cached, hit := u.cache.GetFromCache(ctx, cacheKey); hit {
			log.Printf("ai_cache_hit=true ai_cache_key=%s ai_called=false ai_jobs_returned=%d", cacheKey, len(cached))
			return cached, nil
		}
		log.Printf("ai_cache_hit=false ai_cache_key=%s", cacheKey)
	}

	// Candidate jobs (recent)
	rows, err := u.jobs.ListJobsForListing(ctx, repository.JobListFilter{Limit: maxCandidates, Offset: 0})
	if err != nil || len(rows) == 0 {
		return u.fallbackRecentJobs(ctx, skillKeywords)
	}

	jobCtx := make([]ai.JobContext, 0, len(rows))
	byID := make(map[string]repository.JobListRow, len(rows))
	for _, r := range rows {
		id := r.ID.String()
		byID[id] = r
		jobCtx = append(jobCtx, ai.JobContext{
			ID:          id,
			Title:       r.Title,
			Company:     r.Company,
			Location:    r.Location,
			Description: r.Description,
			Source:      r.Source,
			URL:         r.SourceURL,
		})
	}

	model := strings.TrimSpace(os.Getenv("OPENROUTER_MODEL"))
	if model == "" {
		model = "openrouter/auto"
	}

	// Stampede protection: ensure only one AI call per user at a time.
	if cacheKey != "" {
		locked, lerr := u.cache.AcquireUserLock(ctx, userID)
		if lerr == nil && !locked {
			time.Sleep(AIRecommendationLockWait())
			if cached, hit := u.cache.GetFromCache(ctx, cacheKey); hit {
				log.Printf("ai_cache_hit=true ai_cache_key=%s ai_called=false ai_jobs_returned=%d", cacheKey, len(cached))
				return cached, nil
			}
			log.Printf("ai_cache_lock_contended=true ai_cache_key=%s ai_called=false", cacheKey)
			return u.fallbackRecentJobs(ctx, skillKeywords)
		}
	}

	start := time.Now()
	recs, err := u.provider.Recommend(ctx, userProfile, jobCtx)
	lat := time.Since(start)
	if err != nil {
		log.Printf("ai_recommendation=true ai_failed=true model=%s candidate_jobs=%d ai_error=%v latency=%s", model, len(jobCtx), err, lat)
		return u.fallbackRecentJobs(ctx, skillKeywords)
	}
	log.Printf("ai_recommendation=true ai_failed=false model=%s candidate_jobs=%d ai_returned=%d latency=%s", model, len(jobCtx), len(recs), lat)

	// Map AI recommendations -> jobs
	type scored struct {
		row    repository.JobListRow
		score  int
		reason string
	}

	scoredRows := make([]scored, 0, len(recs))
	seen := make(map[string]struct{})
	for _, r := range recs {
		id := strings.TrimSpace(r.JobID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		row, ok := byID[id]
		if !ok {
			continue
		}
		s := int(r.Score + 0.5)
		if s < 0 {
			s = 0
		}
		if s > 100 {
			s = 100
		}
		if s <= 0 {
			continue
		}
		scoredRows = append(scoredRows, scored{row: row, score: s, reason: strings.TrimSpace(r.Reason)})
		seen[id] = struct{}{}
	}

	sort.SliceStable(scoredRows, func(i, j int) bool {
		return scoredRows[i].score > scoredRows[j].score
	})

	if len(scoredRows) > topK {
		scoredRows = scoredRows[:topK]
	}

	out := make([]AIJobRecommendationItem, 0, len(scoredRows))
	for _, s := range scoredRows {
		out = append(out, AIJobRecommendationItem{
			JobID:       s.row.ID,
			Title:       s.row.Title,
			CompanyName: s.row.Company,
			Location:    s.row.Location,
			JobURL:      s.row.SourceURL,
			Source:      s.row.Source,
			MatchScore:  s.score,
			MatchReason: s.reason,
		})
	}

	if len(out) == 0 {
		return u.fallbackRecentJobs(ctx, skillKeywords)
	}

	if cacheKey != "" {
		_ = u.cache.SaveToCache(ctx, userID, cacheKey, out)
		log.Printf("ai_cache_hit=false ai_cache_key=%s ai_called=true ai_jobs_returned=%d", cacheKey, len(out))
	} else {
		log.Printf("ai_called=true ai_jobs_returned=%d", len(out))
	}
	return out, nil
}

func (u *AIRecommendation) fallbackRecentJobs(ctx context.Context, skillKeywords []string) ([]AIJobRecommendationItem, error) {
	maxCandidates := optIntEnv("AI_RECOMMENDATION_MAX_CANDIDATES", 50)
	if maxCandidates <= 0 {
		maxCandidates = 50
	}
	if maxCandidates > 50 {
		maxCandidates = 50
	}
	topK := optIntEnv("AI_RECOMMENDATION_TOP_K", 20)
	if topK <= 0 {
		topK = 20
	}
	if topK > 20 {
		topK = 20
	}

	rows, err := u.jobs.ListJobsForListing(ctx, repository.JobListFilter{Limit: maxCandidates, Offset: 0})
	if err != nil {
		return nil, ErrInternal
	}

	// If we have user skills, produce a lightweight relevance score from keyword overlap.
	kw := uniqueStrings(skillKeywords)
	if len(kw) == 0 {
		if len(rows) > topK {
			rows = rows[:topK]
		}
		out := make([]AIJobRecommendationItem, 0, len(rows))
		for _, r := range rows {
			out = append(out, AIJobRecommendationItem{
				JobID:       r.ID,
				Title:       r.Title,
				CompanyName: r.Company,
				Location:    r.Location,
				JobURL:      r.SourceURL,
				Source:      r.Source,
				MatchScore:  0,
				MatchReason: "fallback:recent_jobs",
			})
		}
		return out, nil
	}

	type scoredRow struct {
		row    repository.JobListRow
		score  int
		reason string
	}

	all := make([]scoredRow, 0, len(rows))
	for _, r := range rows {
		s, hits := keywordOverlapScore(r, kw)
		reason := "fallback:keyword_overlap"
		if len(hits) > 0 {
			reason = reason + " matched=" + strings.Join(hits, ",")
		}
		all = append(all, scoredRow{row: r, score: s, reason: reason})
	}

	sort.SliceStable(all, func(i, j int) bool {
		return all[i].score > all[j].score
	})
	if len(all) > topK {
		all = all[:topK]
	}

	out := make([]AIJobRecommendationItem, 0, len(all))
	for _, s := range all {
		out = append(out, AIJobRecommendationItem{
			JobID:       s.row.ID,
			Title:       s.row.Title,
			CompanyName: s.row.Company,
			Location:    s.row.Location,
			JobURL:      s.row.SourceURL,
			Source:      s.row.Source,
			MatchScore:  s.score,
			MatchReason: s.reason,
		})
	}
	return out, nil
}

func keywordOverlapScore(r repository.JobListRow, keywords []string) (int, []string) {
	text := strings.ToLower(strings.TrimSpace(r.Title + " " + r.Company + " " + r.Location + " " + r.Description))
	if text == "" || len(keywords) == 0 {
		return 0, nil
	}

	hits := make([]string, 0, 3)
	count := 0
	for _, kw := range keywords {
		k := strings.ToLower(strings.TrimSpace(kw))
		if k == "" {
			continue
		}
		if strings.Contains(text, k) {
			count++
			if len(hits) < 3 {
				hits = append(hits, kw)
			}
		}
	}

	// Score heuristic: each hit adds 25 points, cap 100.
	score := count * 25
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}
	return score, hits
}

func optBoolEnv(key string) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return false
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false
	}
	return v
}

func optIntEnv(key string, defaultVal int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return defaultVal
	}
	if v <= 0 {
		return defaultVal
	}
	return v
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}
