package search

var Synonyms = map[string][]string{
	"office boy": {"office", "helper", "staff"},
	"frontend":   {"front end", "frontend developer", "ui developer"},
	"backend":    {"back end", "server developer"},
	"designer":   {"graphic designer", "ui designer", "visual designer"},
	"admin":      {"administration", "staff admin"},
}

func GetSynonyms(query string) []string {
	if query == "" {
		return []string{}
	}
	if v, ok := Synonyms[query]; ok {
		out := make([]string, 0, len(v))
		out = append(out, v...)
		return out
	}
	return []string{}
}
