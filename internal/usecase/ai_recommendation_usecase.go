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

	cacheKey := ""
	useCache := u.cache != nil && IsAICacheEnabled()
	if useCache {
		hash := BuildUserProfileHash(userID, userSkills, skillsUpdatedAt)
		cacheKey = BuildAIRecommendationCacheKey(userID, hash)
		if cached, lastSeen, lastSeenGlobal, generatedAt, hit := u.cache.GetFromCache(ctx, cacheKey); hit {
			if g := AICacheRefreshGrace(); g > 0 && !generatedAt.IsZero() && time.Since(generatedAt) <= g {
				log.Printf("ai_cache_hit=true ai_cache_key=%s ai_called=false ai_jobs_returned=%d ai_grace_hit=true", cacheKey, len(cached))
				return cached, nil
			}
			// Only run incremental AI if there are truly new jobs in table (global marker changed).
			maxGlobal, merr := u.jobs.GetMaxJobCreatedAt(ctx)
			if merr != nil {
				log.Printf("ai_cache_hit=true ai_cache_key=%s ai_called=false ai_jobs_returned=%d ai_global_check_failed=true err=%v", cacheKey, len(cached), merr)
				return cached, nil
			}
			if !maxGlobal.After(lastSeenGlobal) {
				log.Printf("ai_cache_hit=true ai_cache_key=%s ai_called=false ai_jobs_returned=%d", cacheKey, len(cached))
				return cached, nil
			}

			// There are global new jobs; only re-rank newly added relevant jobs and merge.
			newRows, nerr := u.jobs.ListJobsSkillGroundedCandidatesSince(ctx, patterns, lastSeen, maxCandidates)
			if nerr == nil && len(newRows) == 0 {
				// Global changed but no new relevant candidates; just advance global marker.
				_ = u.cache.SaveToCache(ctx, userID, cacheKey, cached, lastSeen, maxGlobal)
				log.Printf("ai_cache_hit=true ai_cache_key=%s ai_called=false ai_jobs_returned=%d ai_global_advanced=true", cacheKey, len(cached))
				return cached, nil
			}

			if useCache && cacheKey != "" {
				locked, lerr := u.cache.AcquireUserLock(ctx, userID)
				if lerr == nil && !locked {
					time.Sleep(AIRecommendationLockWait())
					if cached2, _, _, _, hit2 := u.cache.GetFromCache(ctx, cacheKey); hit2 {
						log.Printf("ai_cache_hit=true ai_cache_key=%s ai_called=false ai_jobs_returned=%d", cacheKey, len(cached2))
						return cached2, nil
					}
					log.Printf("ai_cache_lock_contended=true ai_cache_key=%s ai_called=false", cacheKey)
				}
			}

			if nerr != nil {
				// Can't check deltas; serve cached to avoid extra DB/AI load.
				log.Printf("ai_cache_hit=true ai_cache_key=%s ai_called=false ai_jobs_returned=%d ai_delta_check_failed=true", cacheKey, len(cached))
				return cached, nil
			}

			// We have new rows; rerank only them.
			backendProfile := isBackendProfile(userSkills)
			newItems, newLastSeen, aerr := u.rankRowsWithAI(ctx, userProfile, userSkills, primarySkill, backendProfile, newRows, topK)
			if aerr != nil {
				// If AI fails for deltas, still return cached.
				log.Printf("ai_incremental_failed=true ai_cache_key=%s err=%v", cacheKey, aerr)
				return cached, nil
			}

			merged := mergeAndSortRecommendations(cached, newItems)
			if len(merged) > topK {
				merged = merged[:topK]
			}

			// Advance marker conservatively.
			marker := lastSeen
			if newLastSeen.After(marker) {
				marker = newLastSeen
			}
			if len(merged) > 0 {
				_ = u.cache.SaveToCache(ctx, userID, cacheKey, merged, marker, maxGlobal)
			}
			log.Printf("ai_cache_hit=true ai_cache_key=%s ai_incremental=true ai_jobs_returned=%d", cacheKey, len(merged))
			return merged, nil
		}
		log.Printf("ai_cache_hit=false ai_cache_key=%s", cacheKey)
	}

	backendProfile := isBackendProfile(userSkills)
	rows, scanned, err := u.scanSkillGroundedCandidates(ctx, patterns, userSkills, primarySkill, backendProfile)
	if err != nil {
		return u.fallbackRecentJobs(ctx, skillKeywords)
	}
	if len(rows) == 0 {
		return []AIJobRecommendationItem{}, nil
	}
	log.Printf("ai_candidates_scan_complete=true total_jobs_scanned=%d candidate_pool=%d", scanned, len(rows))
	out, lastSeen, err := u.rankRowsWithAI(ctx, userProfile, userSkills, primarySkill, backendProfile, rows, topK)
	if err != nil {
		return u.fallbackRecentJobs(ctx, skillKeywords)
	}
	if useCache && cacheKey != "" && len(out) > 0 {
		maxGlobal, _ := u.jobs.GetMaxJobCreatedAt(ctx)
		_ = u.cache.SaveToCache(ctx, userID, cacheKey, out, lastSeen, maxGlobal)
		log.Printf("ai_cache_hit=false ai_cache_key=%s ai_jobs_returned=%d", cacheKey, len(out))
	}
	return out, nil
}

func (u *AIRecommendation) scanSkillGroundedCandidates(ctx context.Context, patterns []string, userSkills []string, primarySkill string, backendProfile bool) ([]repository.JobListRow, int, error) {
	if u == nil || u.jobs == nil {
		return nil, 0, ErrInternal
	}
	if len(patterns) == 0 {
		return []repository.JobListRow{}, 0, nil
	}

	pageSize := optIntEnv("AI_RECOMMENDATION_SCAN_PAGE_SIZE", 1000)
	if pageSize <= 0 {
		pageSize = 1000
	}
	if pageSize > 1000 {
		pageSize = 1000
	}

	poolSize := optIntEnv("AI_RECOMMENDATION_STRUCTURED_POOL_SIZE", 200)
	if poolSize <= 0 {
		poolSize = 200
	}
	if poolSize > 2000 {
		poolSize = 2000
	}

	// Hard guard to avoid runaway scans in case patterns are too broad.
	scanLimit := optIntEnv("AI_RECOMMENDATION_SCAN_LIMIT", 20000)
	if scanLimit <= 0 {
		scanLimit = 20000
	}
	if scanLimit > 200000 {
		scanLimit = 200000
	}

	type scoredRow struct {
		row   repository.JobListRow
		score int
	}

	cursorCreatedAt := time.Time{}
	cursorID := uuid.Nil

	kept := make([]scoredRow, 0, poolSize)
	scanned := 0

	for {
		if scanned >= scanLimit {
			break
		}
		limit := pageSize
		if remain := scanLimit - scanned; remain < limit {
			limit = remain
		}

		page, err := u.jobs.ListJobsSkillGroundedCandidatesPage(ctx, patterns, cursorCreatedAt, cursorID, limit)
		if err != nil {
			return nil, scanned, err
		}
		if len(page) == 0 {
			break
		}
		scanned += len(page)

		for _, r := range page {
			ss, _ := structuredScoreJob(r, userSkills, primarySkill, backendProfile)
			if ss <= 0 {
				continue
			}
			kept = append(kept, scoredRow{row: r, score: ss})
		}

		// Periodically trim to keep memory bounded.
		if len(kept) > poolSize*2 {
			sort.SliceStable(kept, func(i, j int) bool {
				return kept[i].score > kept[j].score
			})
			kept = kept[:poolSize]
		}

		last := page[len(page)-1]
		cursorCreatedAt = last.CreatedAt
		cursorID = last.ID
	}

	if len(kept) == 0 {
		return []repository.JobListRow{}, scanned, nil
	}
	sort.SliceStable(kept, func(i, j int) bool {
		return kept[i].score > kept[j].score
	})
	if len(kept) > poolSize {
		kept = kept[:poolSize]
	}

	out := make([]repository.JobListRow, 0, len(kept))
	for _, it := range kept {
		out = append(out, it.row)
	}
	return out, scanned, nil
}

func (u *AIRecommendation) rankRowsWithAI(ctx context.Context, userProfile string, userSkills []string, primarySkill string, backendProfile bool, rows []repository.JobListRow, topK int) ([]AIJobRecommendationItem, time.Time, error) {
	type scored struct {
		row             repository.JobListRow
		structuredScore int
		finalScore      int
		reasons         []string
	}

	byID := make(map[string]scored, len(rows))
	structuredSum := 0
	lastSeen := time.Time{}
	for _, r := range rows {
		if r.CreatedAt.After(lastSeen) {
			lastSeen = r.CreatedAt
		}
		ss, reasons := structuredScoreJob(r, userSkills, primarySkill, backendProfile)
		if ss <= 0 {
			continue
		}
		structuredSum += ss
		id := r.ID.String()
		byID[id] = scored{row: r, structuredScore: ss, finalScore: ss, reasons: reasons}
	}
	if len(byID) == 0 {
		return []AIJobRecommendationItem{}, lastSeen, nil
	}

	avgStructured := structuredSum / len(byID)

	scoredRows := make([]scored, 0, len(byID))
	for _, v := range byID {
		scoredRows = append(scoredRows, v)
	}
	sort.SliceStable(scoredRows, func(i, j int) bool {
		return scoredRows[i].structuredScore > scoredRows[j].structuredScore
	})

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
	if u.provider != nil && len(scoredRows) > 0 {
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
				score := int(r.Score + 0.5)
				if score < 0 {
					score = 0
				}
				if score > 100 {
					score = 100
				}
				aiSum += score
				fs := int(float64(cur.structuredScore)*0.7 + float64(score)*0.3 + 0.5)
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
			model := strings.TrimSpace(os.Getenv("OPENROUTER_MODEL"))
			if model == "" {
				model = "openrouter/auto"
			}
			log.Printf("ai_recommendation=true ai_failed=true model=%s candidate_jobs=%d ai_error=%v latency=%s", model, len(ctxSubset), err, lat)
			return nil, lastSeen, err
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
	if topK > 0 && len(final) > topK {
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
	log.Printf("total_jobs_scanned=%d jobs_after_filter=%d avg_structured_score=%d avg_ai_score=%d top_job_title=%q ai_called=%t latency=%s", len(rows), len(final), avgStructured, avgAI, topTitle, aiCalled, lat)
	return out, lastSeen, nil
}

func mergeAndSortRecommendations(oldItems []AIJobRecommendationItem, newItems []AIJobRecommendationItem) []AIJobRecommendationItem {
	if len(oldItems) == 0 {
		return newItems
	}
	if len(newItems) == 0 {
		return oldItems
	}

	byID := make(map[uuid.UUID]AIJobRecommendationItem, len(oldItems)+len(newItems))
	for _, it := range oldItems {
		byID[it.JobID] = it
	}
	for _, it := range newItems {
		// Prefer newly analyzed item for same job.
		byID[it.JobID] = it
	}

	out := make([]AIJobRecommendationItem, 0, len(byID))
	for _, it := range byID {
		out = append(out, it)
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].MatchScore > out[j].MatchScore
	})
	return out
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
	rawText := strings.ToLower(strings.TrimSpace(r.Title + "\n" + r.Description + "\n" + r.RawDescription))
	if rawText == "" || len(userSkills) == 0 {
		return 0, nil
	}
	// Normalized token text helps avoid false positives for short skills
	// (e.g. "go" matching "google").
	tokenText := normalizeTokenText(rawText)

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
			// For very short tokens, require word boundaries.
			if len(v) <= 2 {
				if strings.Contains(tokenText, " "+v+" ") {
					matched = append(matched, s)
					break
				}
				continue
			}
			if strings.Contains(rawText, v) {
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

func normalizeTokenText(s string) string {
	// Lowercase expected.
	// Replace non [a-z0-9] with spaces so we can do simple boundary checks.
	// Add leading/trailing space to simplify contains(" token ").
	b := strings.Builder{}
	b.Grow(len(s) + 2)
	b.WriteByte(' ')
	lastSpace := true
	for i := 0; i < len(s); i++ {
		ch := s[i]
		isAZ := ch >= 'a' && ch <= 'z'
		is09 := ch >= '0' && ch <= '9'
		if isAZ || is09 {
			b.WriteByte(ch)
			lastSpace = false
			continue
		}
		if !lastSpace {
			b.WriteByte(' ')
			lastSpace = true
		}
	}
	if !lastSpace {
		b.WriteByte(' ')
	}
	return b.String()
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
