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
	MatchReason []string
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

	userSkills := uniqueStrings(skillKeywords)
	primarySkill := ""
	if len(userSkills) > 0 {
		primarySkill = userSkills[0]
	}
	patterns := buildSkillPatterns(userSkills)

	rows, err := u.jobs.ListJobsSkillGroundedCandidates(ctx, patterns, maxCandidates)
	if err != nil {
		return u.fallbackRecentJobs(ctx, skillKeywords)
	}
	if len(rows) == 0 {
		return []AIJobRecommendationItem{}, nil
	}

	cacheKey := ""
	useCache := u.cache != nil && IsAICacheEnabled() && len(rows) >= 10
	if useCache {
		hash := BuildUserProfileHash(userID, userSkills, skillsUpdatedAt)
		cacheKey = BuildAIRecommendationCacheKey(userID, hash)
		if cached, hit := u.cache.GetFromCache(ctx, cacheKey); hit {
			log.Printf("ai_cache_hit=true ai_cache_key=%s ai_called=false ai_jobs_returned=%d", cacheKey, len(cached))
			return cached, nil
		}
		log.Printf("ai_cache_hit=false ai_cache_key=%s", cacheKey)
	}

	backendProfile := isBackendProfile(userSkills)

	type scored struct {
		row             repository.JobListRow
		structuredScore int
		finalScore      int
		aiScore         int
		reasons         []string
	}

	jobCtx := make([]ai.JobContext, 0, len(rows))
	byID := make(map[string]scored, len(rows))
	structuredSum := 0
	for _, r := range rows {
		ss, reasons := structuredScoreJob(r, userSkills, primarySkill, backendProfile)
		if ss <= 0 {
			continue
		}
		structuredSum += ss
		id := r.ID.String()
		byID[id] = scored{row: r, structuredScore: ss, finalScore: ss, aiScore: 0, reasons: reasons}
		text := strings.TrimSpace(r.Description + "\n" + r.RawDescription)
		jobCtx = append(jobCtx, ai.JobContext{
			ID:          id,
			Title:       r.Title,
			Company:     r.Company,
			Location:    r.Location,
			Description: text,
			Source:      r.Source,
			URL:         r.SourceURL,
		})
	}

	if len(byID) == 0 {
		return []AIJobRecommendationItem{}, nil
	}

	avgStructured := 0
	if len(byID) > 0 {
		avgStructured = structuredSum / len(byID)
	}

	model := strings.TrimSpace(os.Getenv("OPENROUTER_MODEL"))
	if model == "" {
		model = "openrouter/auto"
	}

	scoredRows := make([]scored, 0, len(byID))
	for _, v := range byID {
		scoredRows = append(scoredRows, v)
	}
	sort.SliceStable(scoredRows, func(i, j int) bool {
		return scoredRows[i].structuredScore > scoredRows[j].structuredScore
	})

	aIEnabled := optBoolEnv("AI_RECOMMENDATION_ENABLED")
	rrLimit := optIntEnv("AI_RECOMMENDATION_RERANK_LIMIT", 20)
	if rrLimit <= 0 {
		rrLimit = 20
	}
	if rrLimit > 20 {
		rrLimit = 20
	}

	aiCalled := false
	aiSum := 0
	lat := time.Duration(0)
	if aIEnabled && u.provider != nil && len(jobCtx) > 0 {
		if len(scoredRows) > rrLimit {
			scoredRows = scoredRows[:rrLimit]
		}
		ctxSubset := make([]ai.JobContext, 0, len(scoredRows))
		for _, s := range scoredRows {
			ctxSubset = append(ctxSubset, ai.JobContext{
				ID:          s.row.ID.String(),
				Title:       s.row.Title,
				Company:     s.row.Company,
				Location:    s.row.Location,
				Description: strings.TrimSpace(s.row.Description + "\n" + s.row.RawDescription),
				Source:      s.row.Source,
				URL:         s.row.SourceURL,
			})
		}

		if useCache && cacheKey != "" {
			locked, lerr := u.cache.AcquireUserLock(ctx, userID)
			if lerr == nil && !locked {
				time.Sleep(AIRecommendationLockWait())
				if cached, hit := u.cache.GetFromCache(ctx, cacheKey); hit {
					log.Printf("ai_cache_hit=true ai_cache_key=%s ai_called=false ai_jobs_returned=%d", cacheKey, len(cached))
					return cached, nil
				}
				log.Printf("ai_cache_lock_contended=true ai_cache_key=%s ai_called=false", cacheKey)
			}
		}

		start := time.Now()
		recs, err := u.provider.Recommend(ctx, userProfile, ctxSubset)
		lat = time.Since(start)
		if err == nil {
			aiCalled = true
			for _, r := range recs {
				id := strings.TrimSpace(r.JobID)
				if id == "" {
					continue
				}
				cur, ok := byID[id]
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
				cur.aiScore = s
				aiSum += s
				fs := int(float64(cur.structuredScore)*0.7 + float64(s)*0.3 + 0.5)
				if fs < 0 {
					fs = 0
				}
				if fs > 100 {
					fs = 100
				}
				cur.finalScore = fs
				if rr := strings.TrimSpace(r.Reason); rr != "" {
					cur.reasons = append(cur.reasons, rr)
				}
				byID[id] = cur
			}
		} else {
			log.Printf("ai_recommendation=true ai_failed=true model=%s candidate_jobs=%d ai_error=%v latency=%s", model, len(ctxSubset), err, lat)
		}
	}

	final := make([]scored, 0, len(byID))
	for _, v := range byID {
		if v.finalScore < 30 {
			continue
		}
		final = append(final, v)
	}

	sort.SliceStable(final, func(i, j int) bool {
		return final[i].finalScore > final[j].finalScore
	})
	if len(final) > topK {
		final = final[:topK]
	}

	out := make([]AIJobRecommendationItem, 0, len(final))
	for _, s := range final {
		out = append(out, AIJobRecommendationItem{
			JobID:       s.row.ID,
			Title:       s.row.Title,
			CompanyName: s.row.Company,
			Location:    s.row.Location,
			JobURL:      s.row.SourceURL,
			Source:      s.row.Source,
			MatchScore:  s.finalScore,
			MatchReason: uniqueStrings(s.reasons),
		})
	}

	avgAI := 0
	if aiCalled && len(out) > 0 {
		avgAI = aiSum / len(out)
	}
	topTitle := ""
	if len(out) > 0 {
		topTitle = out[0].Title
	}
	log.Printf("user_id=%s total_jobs_scanned=%d jobs_after_filter=%d avg_structured_score=%d avg_ai_score=%d top_job_title=%q ai_called=%t latency=%s", userID.String(), maxCandidates, len(rows), avgStructured, avgAI, topTitle, aiCalled, lat)

	if useCache && cacheKey != "" && len(out) > 0 {
		_ = u.cache.SaveToCache(ctx, userID, cacheKey, out)
		log.Printf("ai_cache_hit=false ai_cache_key=%s ai_called=%t ai_jobs_returned=%d", cacheKey, aiCalled, len(out))
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
				MatchReason: []string{"fallback:recent_jobs"},
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
			MatchReason: []string{s.reason},
		})
	}
	return out, nil
}

func buildSkillPatterns(skills []string) []string {
	clean := make([]string, 0, len(skills)*2)
	for _, s := range skills {
		k := strings.ToLower(strings.TrimSpace(s))
		if k == "" {
			continue
		}
		clean = append(clean, "%"+k+"%")
		switch k {
		case "go":
			clean = append(clean, "%golang%")
		case "postgres":
			clean = append(clean, "%postgresql%")
		case "postgresql":
			clean = append(clean, "%postgres%")
		}
	}
	return uniqueStrings(clean)
}

func isBackendProfile(skills []string) bool {
	for _, s := range skills {
		k := strings.ToLower(strings.TrimSpace(s))
		switch k {
		case "go", "golang", "node", "node.js", "nodejs", "java", "spring", "c#", "dotnet", "python", "django", "flask", "api", "rest", "microservices", "grpc", "postgres", "postgresql", "redis", "docker", "kubernetes":
			return true
		}
	}
	return false
}

func structuredScoreJob(r repository.JobListRow, userSkills []string, primarySkill string, backendProfile bool) (int, []string) {
	text := strings.ToLower(strings.TrimSpace(r.Title + "\n" + r.Description + "\n" + r.RawDescription))
	if text == "" || len(userSkills) == 0 {
		return 0, nil
	}

	matched := make([]string, 0, len(userSkills))
	for _, s := range userSkills {
		k := strings.ToLower(strings.TrimSpace(s))
		if k == "" {
			continue
		}
		variants := []string{k}
		if k == "go" {
			variants = append(variants, "golang")
		}
		if k == "postgres" {
			variants = append(variants, "postgresql")
		}
		if k == "postgresql" {
			variants = append(variants, "postgres")
		}
		for _, v := range variants {
			if strings.Contains(text, v) {
				matched = append(matched, s)
				break
			}
		}
	}
	matched = uniqueStrings(matched)
	if len(matched) == 0 {
		return 0, nil
	}

	skillScore := float64(len(matched)) / float64(len(userSkills))
	base := int(skillScore*70.0 + 0.5)
	bonus := 0
	penalty := 0

	reasons := make([]string, 0, 6)
	for _, m := range matched {
		reasons = append(reasons, m+" skill match")
	}

	titleLower := strings.ToLower(strings.TrimSpace(r.Title))
	primaryLower := strings.ToLower(strings.TrimSpace(primarySkill))
	if primaryLower != "" && strings.Contains(titleLower, primaryLower) {
		bonus += 20
	}
	if backendProfile {
		backendHits := 0
		for _, kw := range []string{"backend", "api", "microservice", "server", "golang", "rest"} {
			if strings.Contains(titleLower, kw) {
				backendHits++
			}
		}
		if backendHits > 0 {
			bonus += 10
			reasons = append(reasons, "Backend role alignment")
		}
		for _, bad := range []string{"frontend", "react", "ui", "ux", "designer", "data analyst", "marketing"} {
			if strings.Contains(titleLower, bad) {
				penalty += 30
				break
			}
		}
	}

	score := base + bonus - penalty
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	return score, reasons
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
