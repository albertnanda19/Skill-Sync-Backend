package handler

import (
	"encoding/json"
	"errors"
	"html"
	"strconv"
	"strings"
	"time"

	"skill-sync/internal/delivery/http/dto"
	"skill-sync/internal/delivery/http/middleware"
	"skill-sync/internal/pkg/response"
	"skill-sync/internal/usecase"

	"github.com/gofiber/fiber/v3"
)

type JobsHandler struct {
	uc usecase.JobListUsecase
}

func NewJobsHandler(uc usecase.JobListUsecase) *JobsHandler {
	return &JobsHandler{uc: uc}
}

func (h *JobsHandler) HandleListJobs(c fiber.Ctx) error {
	title := c.Query("title")
	companyName := c.Query("company_name")
	location := c.Query("location")
	skills := parseSkillsQuery(c.Query("skills"))

	limit, err := parseQueryIntStrict(c, "limit", 20)
	if err != nil {
		return middleware.NewAppError(fiber.StatusBadRequest, "Bad request", nil, err)
	}
	offset, err := parseQueryIntStrict(c, "offset", 0)
	if err != nil {
		return middleware.NewAppError(fiber.StatusBadRequest, "Bad request", nil, err)
	}

	items, partial, err := h.uc.ListJobs(c.Context(), usecase.JobListParams{
		Title:       title,
		CompanyName: companyName,
		Location:    location,
		Skills:      skills,
		Limit:       limit,
		Offset:      offset,
	})
	if err != nil {
		return mapJobListUsecaseError(err)
	}

	out := make([]dto.JobListResponse, 0, len(items))
	for _, it := range items {
		posted := ""
		if it.PostedAt != nil {
			t := time.Time(*it.PostedAt)
			if !t.IsZero() {
				posted = t.UTC().Format(time.RFC3339)
			}
		}

		jsonDesc, jsonCompany, jsonLocation := extractFromJobPostingJSONLD(it.Description)

		titleClean := sanitizeJobTitle(it.Title)
		companyClean := it.CompanyName
		if strings.TrimSpace(companyClean) == "" {
			companyClean = jsonCompany
		}
		locationClean := it.Location
		if strings.TrimSpace(locationClean) == "" {
			locationClean = jsonLocation
		}

		descSrc := it.Description
		if strings.TrimSpace(jsonDesc) != "" {
			descSrc = jsonDesc
		}
		descClean := sanitizeJobDescription(descSrc)

		out = append(out, dto.JobListResponse{
			JobID:       it.JobID,
			Title:       titleClean,
			CompanyName: strings.TrimSpace(companyClean),
			Location:    strings.TrimSpace(locationClean),
			Description: descClean,
			Skills:      it.Skills,
			PostedDate:  posted,
		})
	}

	msg := "ok"
	if partial {
		msg = "partial data returned"
	}
	return response.Success(c, fiber.StatusOK, msg, out)
}

func sanitizeJobTitle(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	if i := strings.Index(strings.ToLower(s), " | glints"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	s = strings.Join(strings.Fields(s), " ")
	s = strings.TrimRight(s, ",|-")
	s = strings.TrimSpace(s)
	return s
}

func sanitizeJobDescription(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	if looksLikeJSON(s) {
		s = stripJSONLDWrapperNoise(s)
	}

	// Strip HTML tags in a lightweight way.
	var b strings.Builder
	b.Grow(len(s))
	inTag := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '<':
			inTag = true
			continue
		case '>':
			inTag = false
			continue
		}
		if inTag {
			continue
		}
		b.WriteByte(c)
	}

	out := strings.Join(strings.Fields(b.String()), " ")
	out = html.UnescapeString(out)
	if len(out) > 600 {
		out = out[:600]
	}
	return strings.TrimSpace(out)
}

func looksLikeJSON(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "{") && strings.Contains(s, "\"@type\"")
}

func stripJSONLDWrapperNoise(s string) string {
	s = strings.ReplaceAll(s, "\\u003c", "<")
	s = strings.ReplaceAll(s, "\\u003e", ">")
	return s
}

func extractFromJobPostingJSONLD(s string) (desc string, company string, location string) {
	s = strings.TrimSpace(s)
	if !looksLikeJSON(s) {
		return "", "", ""
	}

	jsonObj := extractFirstJSONObject(s)
	if strings.TrimSpace(jsonObj) == "" {
		jsonObj = s
	}

	var m map[string]any
	if err := json.Unmarshal([]byte(jsonObj), &m); err != nil {
		return "", "", ""
	}

	if !isJobPostingType(m["@type"]) {
		return "", "", ""
	}

	if v, ok := m["description"].(string); ok {
		desc = strings.TrimSpace(v)
	}

	if orgAny, ok := m["hiringOrganization"].(map[string]any); ok {
		if v, ok := orgAny["name"].(string); ok {
			company = strings.TrimSpace(v)
		}
	}
	if company == "" {
		if identAny, ok := m["identifier"].(map[string]any); ok {
			if v, ok := identAny["name"].(string); ok {
				company = strings.TrimSpace(v)
			}
		}
	}

	if locAny, ok := m["jobLocation"].(map[string]any); ok {
		if addrAny, ok := locAny["address"].(map[string]any); ok {
			locality, _ := addrAny["addressLocality"].(string)
			region, _ := addrAny["addressRegion"].(string)
			locality = strings.TrimSpace(locality)
			region = strings.TrimSpace(region)
			switch {
			case locality != "" && region != "":
				location = locality + ", " + region
			case locality != "":
				location = locality
			case region != "":
				location = region
			}
		}
	}

	return desc, company, location
}

func isJobPostingType(v any) bool {
	switch x := v.(type) {
	case string:
		return strings.EqualFold(strings.TrimSpace(x), "JobPosting")
	case []any:
		for _, it := range x {
			if s, ok := it.(string); ok {
				if strings.EqualFold(strings.TrimSpace(s), "JobPosting") {
					return true
				}
			}
		}
		return false
	default:
		return false
	}
}

func extractFirstJSONObject(s string) string {
	s = strings.TrimSpace(s)
	start := strings.Index(s, "{")
	if start < 0 {
		return ""
	}
	depth := 0
	inStr := false
	esc := false
	for i := start; i < len(s); i++ {
		ch := s[i]
		if inStr {
			if esc {
				esc = false
				continue
			}
			switch ch {
			case '\\':
				esc = true
			case '"':
				inStr = false
			}
			continue
		}
		switch ch {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

func parseQueryIntStrict(c fiber.Ctx, key string, defaultVal int) (int, error) {
	s := c.Query(key)
	if s == "" {
		return defaultVal, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func parseSkillsQuery(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func mapJobListUsecaseError(err error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, usecase.ErrInvalidInput):
		return middleware.NewAppError(fiber.StatusBadRequest, "Bad request", nil, err)
	case errors.Is(err, usecase.ErrInternal):
		return middleware.NewAppError(fiber.StatusInternalServerError, response.MessageInternalServerError, nil, err)
	default:
		return middleware.NewAppError(fiber.StatusInternalServerError, response.MessageInternalServerError, nil, err)
	}
}
