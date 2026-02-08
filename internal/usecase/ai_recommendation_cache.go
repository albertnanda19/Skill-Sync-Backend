package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	icache "skill-sync/internal/cache"

	"github.com/google/uuid"
)

type CachedAIRecommendations struct {
	UserID                     string                    `json:"user_id"`
	GeneratedAt                time.Time                 `json:"generated_at"`
	LastSeenJobCreatedAt       time.Time                 `json:"last_seen_job_created_at"`
	LastSeenGlobalJobCreatedAt time.Time                 `json:"last_seen_global_job_created_at"`
	Jobs                       []AIJobRecommendationItem `json:"jobs"`
}

type AIRecommendationCache struct {
	cache icache.Cache
}

func NewAIRecommendationCache(cache icache.Cache) *AIRecommendationCache {
	return &AIRecommendationCache{cache: cache}
}

func BuildUserProfileHash(userID uuid.UUID, skills []string, updatedAt time.Time) string {
	return BuildUserProfileHashWithRoles(userID, skills, nil, updatedAt)
}

func BuildUserProfileHashWithRoles(userID uuid.UUID, skills []string, roles []string, updatedAt time.Time) string {
	clean := make([]string, 0, len(skills))
	for _, s := range skills {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		clean = append(clean, s)
	}
	sort.Strings(clean)
	joined := strings.Join(clean, ",")

	roleClean := make([]string, 0, len(roles))
	for _, r := range roles {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		roleClean = append(roleClean, r)
	}
	sort.Strings(roleClean)
	rolesJoined := strings.Join(roleClean, ",")

	payload := userID.String() + "|" + joined + "|" + rolesJoined + "|" + updatedAt.UTC().Format(time.RFC3339Nano)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func BuildUserProfileHashWithContext(userID uuid.UUID, skillSignals []string, roles []string, experienceLevel string, preferenceLocation string, updatedAt time.Time) string {
	cleanSignals := make([]string, 0, len(skillSignals))
	for _, s := range skillSignals {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		cleanSignals = append(cleanSignals, s)
	}
	sort.Strings(cleanSignals)
	signalsJoined := strings.Join(cleanSignals, ",")

	roleClean := make([]string, 0, len(roles))
	for _, r := range roles {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		roleClean = append(roleClean, r)
	}
	sort.Strings(roleClean)
	rolesJoined := strings.Join(roleClean, ",")

	experienceLevel = strings.TrimSpace(experienceLevel)
	preferenceLocation = strings.TrimSpace(preferenceLocation)
	payload := userID.String() + "|" + signalsJoined + "|" + rolesJoined + "|" + experienceLevel + "|" + preferenceLocation + "|" + updatedAt.UTC().Format(time.RFC3339Nano)
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:])
}

func BuildAIRecommendationCacheKey(userID uuid.UUID, profileHash string) string {
	prefix := strings.TrimSpace(os.Getenv("AI_CACHE_PREFIX"))
	if prefix == "" {
		prefix = "recommendations"
	}
	profileHash = strings.TrimSpace(profileHash)
	if profileHash == "" {
		return prefix + ":user:" + userID.String()
	}
	return prefix + ":user:" + userID.String() + ":" + profileHash
}

func IsAICacheEnabled() bool {
	raw := strings.TrimSpace(os.Getenv("AI_CACHE_ENABLED"))
	if raw == "" {
		return false
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false
	}
	return v
}

func AICacheTTL() time.Duration {
	raw := strings.TrimSpace(os.Getenv("AI_CACHE_TTL_SECONDS"))
	if raw == "" {
		return 900 * time.Second
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v <= 0 {
		return 900 * time.Second
	}
	return time.Duration(v) * time.Second
}

func AICacheRefreshGrace() time.Duration {
	raw := strings.TrimSpace(os.Getenv("AI_CACHE_REFRESH_GRACE_SECONDS"))
	if raw == "" {
		return 5 * time.Second
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return 5 * time.Second
	}
	if v > 300 {
		v = 300
	}
	return time.Duration(v) * time.Second
}

func AIRecommendationLockKey(userID uuid.UUID) string {
	return "ai_reco_lock:" + userID.String()
}

func AIRecommendationLockTTL() time.Duration {
	return 15 * time.Second
}

func AIRecommendationLockWait() time.Duration {
	return 200 * time.Millisecond
}

func (c *AIRecommendationCache) GetFromCache(ctx context.Context, key string) ([]AIJobRecommendationItem, time.Time, time.Time, time.Time, bool) {
	if c == nil || c.cache == nil {
		return nil, time.Time{}, time.Time{}, time.Time{}, false
	}
	raw, err := c.cache.Get(ctx, key)
	if err != nil {
		return nil, time.Time{}, time.Time{}, time.Time{}, false
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, time.Time{}, time.Time{}, time.Time{}, false
	}
	var cached CachedAIRecommendations
	if err := json.Unmarshal([]byte(raw), &cached); err != nil {
		return nil, time.Time{}, time.Time{}, time.Time{}, false
	}
	if len(cached.Jobs) == 0 {
		return nil, time.Time{}, time.Time{}, time.Time{}, false
	}
	return cached.Jobs, cached.LastSeenJobCreatedAt, cached.LastSeenGlobalJobCreatedAt, cached.GeneratedAt, true
}

func (c *AIRecommendationCache) SaveToCache(ctx context.Context, userID uuid.UUID, key string, jobs []AIJobRecommendationItem, lastSeenJobCreatedAt time.Time, lastSeenGlobalJobCreatedAt time.Time) error {
	if c == nil || c.cache == nil {
		return nil
	}
	if len(jobs) == 0 {
		return nil
	}
	payload := CachedAIRecommendations{
		UserID:                     userID.String(),
		GeneratedAt:                time.Now().UTC(),
		LastSeenJobCreatedAt:       lastSeenJobCreatedAt.UTC(),
		LastSeenGlobalJobCreatedAt: lastSeenGlobalJobCreatedAt.UTC(),
		Jobs:                       jobs,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	ttl := AICacheTTL()
	log.Printf("ai_cache_save=true ai_cache_key=%s ai_cache_ttl=%d", key, int(ttl.Seconds()))
	return c.cache.Set(ctx, key, string(b), ttl)
}

func (c *AIRecommendationCache) AcquireUserLock(ctx context.Context, userID uuid.UUID) (bool, error) {
	lc, ok := c.cache.(icache.LockingCache)
	if !ok {
		return false, nil
	}
	return lc.SetIfNotExists(ctx, AIRecommendationLockKey(userID), "1", AIRecommendationLockTTL())
}

func (c *AIRecommendationCache) InvalidateUser(ctx context.Context, userID uuid.UUID) {
	lc, ok := c.cache.(icache.LockingCache)
	if !ok {
		return
	}
	prefix := strings.TrimSpace(os.Getenv("AI_CACHE_PREFIX"))
	if prefix == "" {
		prefix = "recommendations"
	}
	pattern := prefix + ":user:" + userID.String() + ":*"
	_ = lc.DeleteByPattern(ctx, pattern)
}
