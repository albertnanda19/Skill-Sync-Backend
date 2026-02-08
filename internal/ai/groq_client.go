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

type GroqClient struct {
	apiKey  string
	model   string
	baseURL string
	http    *http.Client
}

func NewGroqClient() *GroqClient {
	apiKey := strings.TrimSpace(os.Getenv("GROQ_API_KEY"))
	model := strings.TrimSpace(os.Getenv("GROQ_MODEL"))
	if model == "" {
		model = "llama-3.1-8b-instant"
	}
	baseURL := strings.TrimSpace(os.Getenv("GROQ_BASE_URL"))
	if baseURL == "" {
		baseURL = "https://api.groq.com/openai/v1"
	}

	timeout := 10 * time.Second
	if raw := strings.TrimSpace(os.Getenv("AI_TIMEOUT_SECONDS")); raw != "" {
		if sec, err := parsePositiveInt(raw); err == nil && sec > 0 {
			timeout = time.Duration(sec) * time.Second
		}
	}

	return &GroqClient{
		apiKey:  apiKey,
		model:   model,
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *GroqClient) Recommend(ctx context.Context, userProfile string, jobs []JobContext) ([]AIRecommendation, error) {
	if c.apiKey == "" {
		return nil, errors.New("missing GROQ_API_KEY")
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

	body := groqChatRequest{
		Model: c.model,
		Messages: []groqMessage{
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
		return nil, APIError{Provider: "groq", StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	var out groqChatResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}
	if len(out.Choices) == 0 {
		return nil, errors.New("groq: empty choices")
	}

	content := strings.TrimSpace(out.Choices[0].Message.Content)
	jsonPayload := extractJSONArray(content)
	if jsonPayload == "" {
		return nil, fmt.Errorf("groq: no JSON array in response: %s", content)
	}

	var recs []AIRecommendation
	if err := json.Unmarshal([]byte(jsonPayload), &recs); err != nil {
		return nil, err
	}
	return recs, nil
}

type groqChatRequest struct {
	Model    string        `json:"model"`
	Messages []groqMessage `json:"messages"`
}

type groqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type groqChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}
