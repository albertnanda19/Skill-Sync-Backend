package usecase

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strings"
)

type jobSearchCacheKeyInput struct {
	Title       string   `json:"title"`
	CompanyName string   `json:"company_name"`
	Location    string   `json:"location"`
	Skills      []string `json:"skills"`
	SourceIDs   []string `json:"source_ids"`
	Limit       int      `json:"limit"`
	Offset      int      `json:"offset"`
}

func normalizeSearchValue(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	s = strings.Join(strings.Fields(s), " ")
	return s
}

func JobsSearchCacheKey(params JobListParams) string {
	skills := make([]string, 0, len(params.Skills))
	for _, s := range params.Skills {
		s = normalizeSearchValue(s)
		if s == "" {
			continue
		}
		skills = append(skills, s)
	}

	sourceIDs := make([]string, 0, len(params.SourceIDs))
	for _, id := range params.SourceIDs {
		if id.String() == "00000000-0000-0000-0000-000000000000" {
			continue
		}
		sourceIDs = append(sourceIDs, id.String())
	}
	sort.Strings(sourceIDs)

	in := jobSearchCacheKeyInput{
		Title:       normalizeSearchValue(params.Title),
		CompanyName: normalizeSearchValue(params.CompanyName),
		Location:    normalizeSearchValue(params.Location),
		Skills:      skills,
		SourceIDs:   sourceIDs,
		Limit:       params.Limit,
		Offset:      params.Offset,
	}

	b, _ := json.Marshal(in)
	sum := sha256.Sum256(b)
	h := hex.EncodeToString(sum[:])
	return "jobs:search:" + h
}

func JobsSearchLockKey(searchKey string) string {
	searchKey = strings.TrimSpace(searchKey)
	if strings.HasPrefix(searchKey, "jobs:search:") {
		return "jobs:lock:" + strings.TrimPrefix(searchKey, "jobs:search:")
	}
	return "jobs:lock:" + searchKey
}
