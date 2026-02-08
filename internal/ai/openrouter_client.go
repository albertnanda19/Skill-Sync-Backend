package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type OpenRouterClient struct {
	apiKey  string
	model   string
	baseURL string
	http    *http.Client
}

func NewOpenRouterClient() *OpenRouterClient {
	apiKey := strings.TrimSpace(os.Getenv("OPENROUTER_API_KEY"))
	model := strings.TrimSpace(os.Getenv("OPENROUTER_MODEL"))
	if model == "" {
		model = "openrouter/auto"
	}
	baseURL := strings.TrimSpace(os.Getenv("OPENROUTER_BASE_URL"))
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}

	timeout := 10 * time.Second
	if raw := strings.TrimSpace(os.Getenv("AI_TIMEOUT_SECONDS")); raw != "" {
		if sec, err := parsePositiveInt(raw); err == nil && sec > 0 {
			timeout = time.Duration(sec) * time.Second
		}
	}

	return &OpenRouterClient{
		apiKey:  apiKey,
		model:   model,
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *OpenRouterClient) Recommend(ctx context.Context, userProfile string, jobs []JobContext) ([]AIRecommendation, error) {
	if c.apiKey == "" {
		return nil, errors.New("missing OPENROUTER_API_KEY")
	}
	if len(jobs) == 0 {
		return []AIRecommendation{}, nil
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

	body := openRouterChatRequest{
		Model: c.model,
		Messages: []openRouterMessage{
			{Role: "system", Content: systemMsg},
			{Role: "user", Content: userMsg},
		},
	}

	b, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("openrouter error: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var out openRouterChatResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}
	if len(out.Choices) == 0 {
		return nil, errors.New("openrouter: empty choices")
	}

	content := strings.TrimSpace(out.Choices[0].Message.Content)
	jsonPayload := extractJSONArray(content)
	if jsonPayload == "" {
		return nil, fmt.Errorf("openrouter: no JSON array in response: %s", content)
	}

	var recs []AIRecommendation
	if err := json.Unmarshal([]byte(jsonPayload), &recs); err != nil {
		return nil, err
	}
	return recs, nil
}

type openRouterChatRequest struct {
	Model    string              `json:"model"`
	Messages []openRouterMessage `json:"messages"`
}

type openRouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openRouterChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func extractJSONArray(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	start := strings.Index(s, "[")
	end := strings.LastIndex(s, "]")
	if start == -1 || end == -1 || end < start {
		return ""
	}
	return strings.TrimSpace(s[start : end+1])
}

func parsePositiveInt(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, errors.New("empty")
	}
	n := 0
	for _, ch := range raw {
		if ch < '0' || ch > '9' {
			return 0, errors.New("not int")
		}
		n = n*10 + int(ch-'0')
		if n > 86400 {
			return n, nil
		}
	}
	if n <= 0 {
		return 0, errors.New("non-positive")
	}
	return n, nil
}
