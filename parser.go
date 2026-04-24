package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gojp/kana"
	"github.com/jim-ww/nihongo/store"
)

var (
	numberPrefix  = regexp.MustCompile(`^[①②③④⑤⑥⑦⑧⑨⑩0-9]+\.?\s*`)
	seeAlso       = regexp.MustCompile(`(?i)^see also[:.]?\s*`)
	bracketNoise  = regexp.MustCompile(`\s*\([^)]+\)`)
	formsNoise    = regexp.MustCompile(`(?i)\b(forms?|tatoeba|\[\d+\]|see also)\b`)
	japaneseNoise = regexp.MustCompile(`[\p{Han}\p{Hiragana}\p{Katakana}]{3,}`)
)

func getString(v any) string {
	if v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func getFloat(v any) float64 {
	if v == nil {
		return 0
	}
	if f, ok := v.(float64); ok {
		return f
	}
	return 0
}

func convertToFtsDict(term []any) store.FtsDict {
	if len(term) < 8 {
		return store.FtsDict{}
	}

	expression := getString(term[0])
	reading := getString(term[1])
	romaji := kana.KanaToRomaji(reading)

	pos, defs, examples := extractGlossary(term[5])

	return store.FtsDict{
		Expression:     expression,
		Reading:        reading,
		ReadingRomaji:  romaji,
		DefinitionTags: getString(term[2]),
		TermTags:       getString(term[7]),
		Sequence:       fmt.Sprintf("%.0f", getFloat(term[6])),
		Score:          getFloat(term[4]),
		Definitions:    defs,
		Examples:       examples,
		Pos:            pos,
	}
}

func extractGlossary(glossary any) (pos, defs, examples []string) {
	walkGlossary(glossary, &pos, &defs, &examples)

	cleanPos := make([]string, 0, len(pos))
	for _, p := range pos {
		if !isPosOrMeta(p) {
			cleanPos = append(cleanPos, p)
		}
	}

	cleanDefs := make([]string, 0, len(defs))
	for _, d := range defs {
		d = cleanDefinition(d)
		if d != "" && !isPosOrMeta(d) && len(d) > 3 && !japaneseNoise.MatchString(d) {
			cleanDefs = append(cleanDefs, d)
		}
	}

	return cleanPos, cleanDefs, examples
}

func walkGlossary(node any, pos, defs, examples *[]string) {
	switch v := node.(type) {
	case []any:
		for _, child := range v {
			walkGlossary(child, pos, defs, examples)
		}

	case map[string]any:
		sc := getString(v["data-sc-content"])
		tag := getString(v["tag"])
		text := getString(v["text"])

		if tag == "ruby" {
			return
		}

		if sc == "part-of-speech-info" {
			collectText(v, pos)
			return
		}

		if sc == "example-sentence" || sc == "example" || sc == "example-sentence-b" || tag == "example-sentence" {
			collectExample(v, examples)
			return
		}

		if text != "" {
			trimmed := strings.TrimSpace(text)
			if trimmed != "" {
				if isPosOrMeta(trimmed) {
					*pos = append(*pos, trimmed)
				} else if isLikelyExampleText(trimmed) {
					*examples = append(*examples, trimmed)
				} else {
					*defs = append(*defs, trimmed)
				}
			}
		}

		if content, ok := v["content"]; ok {
			walkGlossary(content, pos, defs, examples)
		}

	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return
		}
		if isPosOrMeta(trimmed) {
			*pos = append(*pos, trimmed)
		} else if isLikelyExampleText(trimmed) {
			*examples = append(*examples, trimmed)
		} else {
			*defs = append(*defs, trimmed)
		}
	}
}

func collectText(node any, target *[]string) {
	switch v := node.(type) {
	case []any:
		for _, child := range v {
			collectText(child, target)
		}
	case map[string]any:
		if text := getString(v["text"]); text != "" {
			*target = append(*target, strings.TrimSpace(text))
		}
		if content, ok := v["content"]; ok {
			collectText(content, target)
		}
	}
}

func collectExample(node any, examples *[]string) {
	switch v := node.(type) {
	case []any:
		for _, child := range v {
			collectExample(child, examples)
		}
	case map[string]any:
		if text := getString(v["text"]); text != "" {
			*examples = append(*examples, strings.TrimSpace(text))
		}
		if content, ok := v["content"]; ok {
			collectExample(content, examples)
		}
	}
}

func cleanDefinition(s string) string {
	s = numberPrefix.ReplaceAllString(s, "")
	s = seeAlso.ReplaceAllString(s, "")
	s = bracketNoise.ReplaceAllString(s, "")
	s = formsNoise.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)
	return s
}

func isPosOrMeta(s string) bool {
	lower := strings.ToLower(strings.TrimSpace(s))
	switch lower {
	case "noun", "suru", "intransitive", "transitive", "na-adj", "adj-na", "i-adj", "adj-i",
		"exp", "v1", "v5", "v5u", "v5k", "v5g", "v5s", "v5t", "v5n", "v5b", "v5m", "v5r",
		"no-adj", "1-dan", "adjective", "adverb", "to-adverb", "5-dan", "proverb",
		"derogatory", "polite", "colloquial", "archaic", "physics", "kana", "mimetic",
		"idiom", "auxiliary", "suffix", "prefix", "particle", "formal", "buddhism",
		"bardo", "adjectival", "forms", "tatoeba", "see also",
		"grand", "fragment":
		return true
	}

	if strings.HasPrefix(lower, "note") || strings.Contains(lower, "old kanji") ||
		strings.Contains(lower, "redirected from") ||
		lower == "jm dict" || lower == "jmdict" || lower == "tatoeba" ||
		lower == "|" || lower == "—" || lower == "⟶" || lower == "→" || lower == "＊" ||
		lower == "〔" || lower == "〕" {
		return true
	}
	return false
}

func isLikelyExampleText(s string) bool {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return false
	}
	if strings.ContainsAny(trimmed, "?!！？") {
		return true
	}
	hasCJK := false
	hasLatin := false
	for _, r := range trimmed {
		if r >= 0x3000 && r <= 0x9FFF {
			hasCJK = true
		}
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			hasLatin = true
		}
	}
	if hasCJK && hasLatin && len(trimmed) > 15 {
		return true
	}
	return hasCJK && hasLatin && len(trimmed) > 12
}
