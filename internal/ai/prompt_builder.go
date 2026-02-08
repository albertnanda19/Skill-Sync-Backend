package ai

import (
	"fmt"
	"strings"
)

const maxDescriptionChars = 500

func BuildSystemPrompt() string {
	return "You are a job-to-skill matching engine.\nYour task is to score each job ONLY by direct, explicit skill overlap and role/domain alignment with the user profile.\nDo NOT use generic semantic similarity.\nIf a job does not explicitly mention relevant skills, it must receive a very low score or be excluded.\nReturn only valid JSON."
}

func BuildUserPrompt(userProfile string, jobs []JobContext, maxResults int) string {
	b := strings.Builder{}
	b.WriteString("User Profile:\n")
	b.WriteString(strings.TrimSpace(userProfile))
	b.WriteString("\n\nJobs:\n")

	for i, j := range jobs {
		desc := strings.TrimSpace(j.Description)
		if len(desc) > maxDescriptionChars {
			desc = desc[:maxDescriptionChars]
		}
		b.WriteString(fmt.Sprintf("%d.\nID: %s\nTitle: %s\nCompany: %s\nLocation: %s\nDescription: %s\n\n", i+1, j.ID, j.Title, j.Company, j.Location, desc))
	}

	if maxResults <= 0 {
		maxResults = 20
	}

	b.WriteString("Instruction:\nReturn JSON array:\n\n[")
	b.WriteString("\n  { \"job_id\": \"<one of the provided IDs>\", \"score\": 0-100, \"reason\": \"short explanation\" }")
	b.WriteString("\n]\n\nRules:\n")
	b.WriteString("- Score range: 0–100\n")
	b.WriteString("- Higher score = more direct skill overlap and better domain alignment\n")
	b.WriteString("- Focus ONLY on: explicit skill mentions (title/description), backend vs frontend alignment, and seniority if available\n")
	b.WriteString("- Ignore company reputation, compensation, and generic similarity\n")
	b.WriteString("- Exclude jobs with weak or no explicit overlap by returning [] or omitting them\n")
	b.WriteString(fmt.Sprintf("- Maximum %d results\n", maxResults))
	b.WriteString("- If none are relevant, return []\n")
	b.WriteString("- job_id MUST match exactly one of the IDs provided above\n")
	b.WriteString("- Return JSON only (no markdown, no prose)\n")

	return b.String()
}
