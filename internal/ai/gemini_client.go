package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"google.golang.org/genai"
)

type GeminiClient struct {
	apiKey string
	model  string
}

func NewGeminiClient() *GeminiClient {
	apiKey := strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	model := strings.TrimSpace(os.Getenv("GEMINI_MODEL"))
	if model == "" {
		model = "gemini-2.5-flash"
	}
	return &GeminiClient{apiKey: apiKey, model: model}
}

func (c *GeminiClient) Recommend(ctx context.Context, userProfile string, jobs []JobContext) ([]AIRecommendation, error) {
	if c.apiKey == "" {
		return nil, errors.New("missing GEMINI_API_KEY")
	}
	if len(jobs) == 0 {
		return []AIRecommendation{}, nil
	}

	timeout := 10 * time.Second
	if raw := strings.TrimSpace(os.Getenv("AI_TIMEOUT_SECONDS")); raw != "" {
		if sec, err := parsePositiveInt(raw); err == nil && sec > 0 {
			timeout = time.Duration(sec) * time.Second
		}
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: c.apiKey, Backend: genai.BackendGeminiAPI})
	if err != nil {
		return nil, err
	}

	maxResults := 20
	if raw := strings.TrimSpace(os.Getenv("AI_RECOMMENDATION_TOP_K")); raw != "" {
		if n, err := parsePositiveInt(raw); err == nil && n > 0 {
			maxResults = n
		}
	}
	if maxResults > 20 {
		maxResults = 20
	}

	systemMsg := BuildSystemPrompt()
	userMsg := BuildUserPrompt(userProfile, jobs, maxResults)

	cfg := &genai.GenerateContentConfig{
		SystemInstruction: &genai.Content{Parts: []*genai.Part{{Text: systemMsg}}},
	}

	result, err := client.Models.GenerateContent(
		ctx,
		c.model,
		[]*genai.Content{{Parts: []*genai.Part{{Text: userMsg}}}},
		cfg,
	)
	if err != nil {
		var apiErr genai.APIError
		if errors.As(err, &apiErr) {
			return nil, APIError{Provider: "gemini", StatusCode: apiErr.Code, Body: apiErr.Message}
		}
		return nil, err
	}

	content := strings.TrimSpace(result.Text())
	jsonPayload := extractJSONArray(content)
	if jsonPayload == "" {
		return nil, fmt.Errorf("gemini: no JSON array in response: %s", content)
	}

	var recs []AIRecommendation
	if err := json.Unmarshal([]byte(jsonPayload), &recs); err != nil {
		return nil, err
	}
	return recs, nil
}
