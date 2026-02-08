package ai

import (
	"context"
	"log"
)

type FallbackProvider struct {
	providers []Provider
}

func NewFallbackProvider(providers ...Provider) *FallbackProvider {
	return &FallbackProvider{providers: providers}
}

func (p *FallbackProvider) Recommend(ctx context.Context, userProfile string, jobs []JobContext) ([]AIRecommendation, error) {
	var lastErr error
	for i, pr := range p.providers {
		if pr == nil {
			continue
		}
		recs, err := pr.Recommend(ctx, userProfile, jobs)
		if err == nil {
			if i > 0 {
				log.Printf("ai_provider_fallback=true used_provider_index=%d", i)
			}
			return recs, nil
		}
		lastErr = err
		if IsRetryableAPIError(err) {
			continue
		}
		// Non-retryable: stop early.
		break
	}
	return nil, lastErr
}
