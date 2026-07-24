package main

import (
	"fmt"
	"strings"

	"github.com/gojp/kana"
	"github.com/jim-ww/nihongo/store"
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

// asSlice normalizes a JSON value that may be a single node or an array of
// nodes into a slice, since Yomitan's structured-content format allows both
// interchangeably wherever a "content" field is used.
func asSlice(v any) []any {
	if v == nil {
		return nil
	}
	if arr, ok := v.([]any); ok {
		return arr
	}
	return []any{v}
}

// dataContent returns a structured-content node's `data.content` field,
// which Yomitan dictionaries use as a semantic tag identifying what the
// node represents (glossary, part-of-speech-info, example-sentence, ...).
// This is the authoritative way to interpret the tree; raw tag names
// (div/span/ul) are purely presentational and carry no meaning on their own.
func dataContent(v map[string]any) string {
	d, ok := v["data"].(map[string]any)
	if !ok {
		return ""
	}
	return getString(d["content"])
}

// renderText flattens a structured-content node into plain text.
// Ruby annotations (furigana) are rendered as "base(reading)" so kanji
// readings inside examples/definitions survive instead of being dropped.
func renderText(node any) string {
	switch v := node.(type) {
	case string:
		return v
	case []any:
		var b strings.Builder
		for _, c := range v {
			b.WriteString(renderText(c))
		}
		return b.String()
	case map[string]any:
		switch getString(v["tag"]) {
		case "rt":
			// consumed by the enclosing ruby node
			return ""
		case "ruby":
			var base strings.Builder
			var reading string
			for _, c := range asSlice(v["content"]) {
				if m, ok := c.(map[string]any); ok && getString(m["tag"]) == "rt" {
					reading = renderText(m["content"])
					continue
				}
				base.WriteString(renderText(c))
			}
			if reading != "" {
				return base.String() + "(" + reading + ")"
			}
			return base.String()
		}
		if content, ok := v["content"]; ok {
			return renderText(content)
		}
		return ""
	}
	return ""
}

// collectListItems renders each <li> under a node (e.g. a glossary <ul>)
// into its own string.
func collectListItems(content any) []string {
	var out []string
	for _, it := range asSlice(content) {
		t := strings.TrimSpace(renderText(it))
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func appendUnique(list *[]string, s string) {
	for _, existing := range *list {
		if existing == s {
			return
		}
	}
	*list = append(*list, s)
}

// extractGlossary parses a Yomitan term-bank glossary field (term[5]) into
// part-of-speech tags, flat definitions and example sentences. It walks the
// structured-content tree by its semantic `data.content` markers rather than
// guessing intent from raw text, since that's how the format actually
// encodes meaning.
func extractGlossary(glossary any) (pos, defs, examples []string) {
	for _, item := range asSlice(glossary) {
		switch v := item.(type) {
		case string:
			if t := strings.TrimSpace(v); t != "" {
				defs = append(defs, t)
			}
		case map[string]any:
			switch getString(v["type"]) {
			case "structured-content":
				walkStructuredContent(v["content"], &pos, &defs, &examples)
			case "text":
				t := strings.TrimSpace(getString(v["text"]))
				if t == "" {
					t = strings.TrimSpace(renderText(v["content"]))
				}
				if t != "" {
					defs = append(defs, t)
				}
			case "image":
				// unsupported in a text-only CLI, skip
			default:
				if _, ok := v["content"]; ok {
					walkStructuredContent(v["content"], &pos, &defs, &examples)
				}
			}
		}
	}
	return
}

// walkStructuredContent recursively descends a structured-content tree,
// extracting information from nodes with a recognized `data.content`
// marker and transparently recursing through purely presentational
// wrappers (div/span/ul/ol/li with no marker) to find them. Containers
// known to hold only presentational or attribution text (links back to the
// source dictionary, alternate-spelling tables) are skipped entirely so
// their text never leaks into definitions.
func walkStructuredContent(node any, pos, defs, examples *[]string) {
	switch v := node.(type) {
	case []any:
		for _, child := range v {
			walkStructuredContent(child, pos, defs, examples)
		}
		return
	case map[string]any:
		switch dataContent(v) {
		case "part-of-speech-info":
			text := strings.TrimSpace(getString(v["title"]))
			if text == "" {
				text = strings.TrimSpace(renderText(v["content"]))
			}
			if text != "" {
				appendUnique(pos, text)
			}
			return

		case "glossary":
			items := collectListItems(v["content"])
			if joined := strings.Join(items, "; "); joined != "" {
				*defs = append(*defs, joined)
			}
			return

		case "sense-note", "info-gloss":
			var text string
			for _, c := range asSlice(v["content"]) {
				m, ok := c.(map[string]any)
				if !ok {
					continue
				}
				dc := dataContent(m)
				if strings.HasSuffix(dc, "-content") {
					text = strings.TrimSpace(renderText(m["content"]))
				}
			}
			if text != "" {
				*defs = append(*defs, "Note: "+text)
			}
			return

		case "xref":
			var link, gloss string
			for _, c := range asSlice(v["content"]) {
				m, ok := c.(map[string]any)
				if !ok {
					continue
				}
				switch dataContent(m) {
				case "xref-content":
					var parts []string
					for _, cc := range asSlice(m["content"]) {
						if cm, ok := cc.(map[string]any); ok && dataContent(cm) == "reference-label" {
							continue
						}
						if t := strings.TrimSpace(renderText(cc)); t != "" {
							parts = append(parts, t)
						}
					}
					link = strings.Join(parts, " ")
				case "xref-glossary":
					gloss = strings.TrimSpace(renderText(m["content"]))
				}
			}
			if link != "" {
				s := "See also: " + link
				if gloss != "" {
					s += " (" + gloss + ")"
				}
				*defs = append(*defs, s)
			}
			return

		case "example-sentence":
			var jp, en string
			for _, c := range asSlice(v["content"]) {
				m, ok := c.(map[string]any)
				if !ok {
					continue
				}
				switch dataContent(m) {
				case "example-sentence-a":
					jp = strings.TrimSpace(renderText(m["content"]))
				case "example-sentence-b":
					en = strings.TrimSpace(renderText(m["content"]))
				}
			}
			if jp != "" || en != "" {
				*examples = append(*examples, jp+"\n"+en)
			}
			return

		case "attribution", "forms":
			// source-dictionary link / alternate-spelling table: not useful as text
			return
		}

		// presentational wrapper (sense-group, sense, extra-info, ...): recurse
		if content, ok := v["content"]; ok {
			walkStructuredContent(content, pos, defs, examples)
		}
	}
}
