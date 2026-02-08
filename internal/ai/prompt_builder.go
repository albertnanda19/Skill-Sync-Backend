package ai

import (
	"fmt"
	"strings"
)

const maxDescriptionChars = 500

func BuildSystemPrompt() string {
	return "You are an intelligent job recommendation engine.\nYour task is to rank job postings based on how relevant they are to the user profile.\nUse semantic understanding of skills, experience, job title, and description.\nReturn only valid JSON."
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
	b.WriteString("- Higher score = more relevant\n")
	b.WriteString("- Only include jobs that are truly relevant to the user profile\n")
	b.WriteString(fmt.Sprintf("- Maximum %d results\n", maxResults))
	b.WriteString("- If none are relevant, return []\n")
	b.WriteString("- job_id MUST match exactly one of the IDs provided above\n")
	b.WriteString("- Return JSON only (no markdown, no prose)\n")

	return b.String()
}
