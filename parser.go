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

		case "sense":
			// A sense's glossary and any note/xref that qualify it live
			// together under this node. Fold them into one definition
			// string instead of flattening them into separate list items,
			// so a note like "usu. 小父さん or おじさん" reads as
			// qualifying its own sense rather than appearing to be an
			// unrelated definition of its own.
			var gloss, notes, xrefs []string
			walkSense(v["content"], &gloss, &notes, &xrefs, examples)

			def := strings.Join(gloss, "; ")
			for _, n := range notes {
				def += " (" + n + ")"
			}
			for _, x := range xrefs {
				def += " (" + x + ")"
			}
			def = strings.TrimSpace(def)
			if def != "" {
				*defs = append(*defs, def)
			}
			return

		case "glossary":
			items := collectListItems(v["content"])
			if joined := strings.Join(items, "; "); joined != "" {
				*defs = append(*defs, joined)
			}
			return

		case "sense-note", "info-gloss":
			if label, text := noteLabelText(v); text != "" {
				*defs = append(*defs, label+": "+text)
			}
			return

		case "xref":
			if x := xrefText(v); x != "" {
				*defs = append(*defs, x)
			}
			return

		case "example-sentence":
			if jp, en := exampleSentenceText(v); jp != "" || en != "" {
				*examples = append(*examples, jp+"\n"+en)
			}
			return

		case "attribution", "forms":
			// source-dictionary link / alternate-spelling table: not useful as text
			return
		}

		// presentational wrapper (sense-group, extra-info, ...): recurse
		if content, ok := v["content"]; ok {
			walkStructuredContent(content, pos, defs, examples)
		}
	}
}

// walkSense descends a single sense's subtree, splitting its glossary text
// from the notes/xrefs that qualify it, so the caller can fold them into one
// definition. Example sentences found along the way still flow to the
// entry's shared examples list.
func walkSense(node any, gloss, notes, xrefs *[]string, examples *[]string) {
	switch v := node.(type) {
	case []any:
		for _, c := range v {
			walkSense(c, gloss, notes, xrefs, examples)
		}
		return
	case map[string]any:
		switch dataContent(v) {
		case "glossary":
			*gloss = append(*gloss, collectListItems(v["content"])...)
			return
		case "sense-note", "info-gloss":
			if label, text := noteLabelText(v); text != "" {
				*notes = append(*notes, label+": "+text)
			}
			return
		case "xref":
			if x := xrefText(v); x != "" {
				*xrefs = append(*xrefs, x)
			}
			return
		case "example-sentence":
			if jp, en := exampleSentenceText(v); jp != "" || en != "" {
				*examples = append(*examples, jp+"\n"+en)
			}
			return
		}
		if content, ok := v["content"]; ok {
			walkSense(content, gloss, notes, xrefs, examples)
		}
	}
}

func noteLabelText(v map[string]any) (label, text string) {
	for _, c := range asSlice(v["content"]) {
		m, ok := c.(map[string]any)
		if !ok {
			continue
		}
		switch {
		case strings.HasSuffix(dataContent(m), "-label"):
			label = strings.TrimSpace(renderText(m["content"]))
		case strings.HasSuffix(dataContent(m), "-content"):
			text = strings.TrimSpace(renderText(m["content"]))
		}
	}
	if label == "" {
		label = "Note"
	}
	return label, text
}

func xrefText(v map[string]any) string {
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
	if link == "" {
		return ""
	}
	s := "See also: " + link
	if gloss != "" {
		s += " (" + gloss + ")"
	}
	return s
}

func exampleSentenceText(v map[string]any) (jp, en string) {
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
	return jp, en
}
