package ai

import "context"

type Provider interface {
	Recommend(ctx context.Context, userProfile string, jobs []JobContext) ([]AIRecommendation, error)
}
