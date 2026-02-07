package search

import (
	"strings"
	"unicode"
)

type QueryContext struct {
	Original   string
	Normalized string
	Variants   []string
}

func NormalizeQuery(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}
	input = strings.ToLower(input)

	b := strings.Builder{}
	b.Grow(len(input))
	lastWasSpace := false

	for _, r := range input {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			b.WriteRune(r)
			lastWasSpace = false
			continue
		}
		if unicode.IsSpace(r) {
			if b.Len() == 0 || lastWasSpace {
				continue
			}
			b.WriteByte(' ')
			lastWasSpace = true
			continue
		}
		// drop all other characters
	}

	out := strings.TrimSpace(b.String())
	out = strings.Join(strings.Fields(out), " ")
	return out
}

func ExpandQuery(normalized string) []string {
	normalized = strings.TrimSpace(normalized)
	if normalized == "" {
		return []string{}
	}

	out := make([]string, 0, 10)
	seen := make(map[string]struct{}, 10)
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}

	add(normalized)

	// 1) Synonyms for exact full query
	for _, syn := range GetSynonyms(normalized) {
		add(syn)
	}

	words := strings.Fields(normalized)

	// 2) If query is a single token, try joining-with-space heuristic based on synonym keys.
	//    e.g. officeboy -> office boy
	if len(words) == 1 {
		single := words[0]
		for k, syns := range Synonyms {
			if !strings.Contains(k, " ") {
				continue
			}
			kNoSpace := strings.ReplaceAll(k, " ", "")
			if kNoSpace != single {
				continue
			}
			add(k)
			for _, syn := range syns {
				add(syn)
			}
			break
		}
	}

	// 3) Prefix-based expansion: if the query begins with a term that has synonyms, replace only that prefix.
	//    Works for multi-word queries like "officeboy jakarta" once the heuristic adds "office boy jakarta".
	tryPrefix := func(phrase string, rest []string) {
		phrase = strings.TrimSpace(phrase)
		if phrase == "" {
			return
		}
		syns := GetSynonyms(phrase)
		if len(syns) == 0 {
			return
		}
		restStr := strings.Join(rest, " ")
		for _, syn := range syns {
			if restStr == "" {
				add(syn)
				continue
			}
			add(strings.TrimSpace(syn + " " + restStr))
		}
	}

	if len(words) >= 1 {
		tryPrefix(words[0], words[1:])
	}
	if len(words) >= 2 {
		tryPrefix(words[0]+" "+words[1], words[2:])
	}

	// 4) If the first token is a compact form of a spaced synonym key, generate spaced prefix variants.
	//    e.g. officeboy jakarta -> office boy jakarta -> (office|helper|staff) jakarta
	if len(words) >= 1 && !strings.Contains(words[0], " ") {
		first := words[0]
		rest := words[1:]
		for k, syns := range Synonyms {
			if !strings.Contains(k, " ") {
				continue
			}
			kNoSpace := strings.ReplaceAll(k, " ", "")
			if kNoSpace != first {
				continue
			}
			base := k
			if len(rest) > 0 {
				base = strings.TrimSpace(base + " " + strings.Join(rest, " "))
			}
			add(base)
			restStr := strings.Join(rest, " ")
			for _, syn := range syns {
				if restStr == "" {
					add(syn)
					continue
				}
				add(strings.TrimSpace(syn + " " + restStr))
			}
			break
		}
	}

	if len(out) > 10 {
		out = out[:10]
	}
	return out
}

func ProcessQuery(input string) QueryContext {
	ctx := QueryContext{Original: input}
	ctx.Normalized = NormalizeQuery(input)
	if ctx.Normalized == "" {
		ctx.Variants = []string{}
		return ctx
	}
	ctx.Variants = ExpandQuery(ctx.Normalized)
	if len(ctx.Variants) > 10 {
		ctx.Variants = ctx.Variants[:10]
	}
	return ctx
}

func FallbackFirstWord(normalized string) string {
	normalized = strings.TrimSpace(normalized)
	if normalized == "" {
		return ""
	}
	words := strings.Fields(normalized)
	if len(words) == 0 {
		return ""
	}
	return words[0]
}
