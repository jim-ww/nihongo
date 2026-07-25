package main

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/gojp/kana"
	"github.com/jim-ww/nihongo/store"
)

// tripleN matches gojp/kana's romanization bug where a geminate ん before a
// な-row kana (な/に/ぬ/ね/の) gets rendered with one extra spurious 'n'
// (e.g. "みんな" -> "minnna" instead of "minna"). Collapsing any run of 3+
// 'n' down to the correct 2 fixes romaji search for that whole class of
// words without touching the (correct) single-n cases.
var tripleN = regexp.MustCompile(`n{3,}`)

func normalizeRomaji(s string) string {
	return tripleN.ReplaceAllString(s, "nn")
}

// particleReplacer produces the alternate reading used when は/へ/を appear
// in their grammatical-particle pronunciation (wa/e/o) rather than their
// literal one (ha/he/wo) - e.g. こんにちは is pronounced/romanized
// "konnichiwa", not "konnichiha". There's no way to know from the reading
// alone which is intended, so rather than hardcoding a word list, this
// mechanically generates the alternate romanization for any reading
// containing one of these three kana and lets search match either; a
// reading where は/へ/を is never intended as a particle just gets one
// unused extra index entry, which is harmless.
var particleReplacer = strings.NewReplacer("は", "わ", "へ", "え", "を", "お")

// altReadingRomaji returns the particle-pronunciation romanization of
// reading, or "" if は/へ/を don't appear in it (nothing to disambiguate).
func altReadingRomaji(reading string) string {
	if !strings.ContainsAny(reading, "はへを") {
		return ""
	}
	alt := particleReplacer.Replace(reading)
	if alt == reading {
		return ""
	}
	return normalizeRomaji(kana.KanaToRomaji(alt))
}

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
	romaji := normalizeRomaji(kana.KanaToRomaji(reading))
	altRomaji := altReadingRomaji(reading)

	groups, pos, defs, examples, forms := extractEntry(term[5])

	var altForms []string
	for _, f := range forms {
		if f != expression && f != reading {
			appendUnique(&altForms, f)
		}
	}

	return store.FtsDict{
		Expression:       expression,
		Reading:          reading,
		ReadingRomaji:    romaji,
		ReadingRomajiAlt: altRomaji,
		DefinitionTags:   getString(term[2]),
		TermTags:         getString(term[7]),
		Sequence:         fmt.Sprintf("%.0f", getFloat(term[6])),
		Score:            getFloat(term[4]),
		Definitions:      defs,
		Examples:         examples,
		Pos:              pos,
		Groups:           groups,
		Forms:            altForms,
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

// findAllByMarker collects every node under root carrying the given
// `data.content` marker. It does not recurse into a matched node, since
// none of the markers this parser cares about nest inside themselves
// (a "sense" never contains another "sense", a "sense-group" never
// contains another "sense-group").
func findAllByMarker(root any, marker string) []map[string]any {
	var out []map[string]any
	var walk func(n any)
	walk = func(n any) {
		switch v := n.(type) {
		case []any:
			for _, c := range v {
				walk(c)
			}
		case map[string]any:
			if dataContent(v) == marker {
				out = append(out, v)
				return
			}
			if content, ok := v["content"]; ok {
				walk(content)
			}
		}
	}
	walk(root)
	return out
}

// extractEntry parses a Yomitan term-bank glossary field (term[5]) by
// walking the structured-content tree via its semantic `data.content`
// markers rather than guessing intent from raw text. It returns the entry
// grouped the way the dictionary itself groups it - senses sharing a
// part-of-speech under one heading - plus flattened pos/definitions/examples
// for search indexing and list-view previews.
func extractEntry(glossary any) (groups []store.SenseGroup, pos, defs, examples, forms []string) {
	for _, item := range asSlice(glossary) {
		switch v := item.(type) {
		case string:
			if t := strings.TrimSpace(v); t != "" {
				groups = append(groups, store.SenseGroup{Senses: []store.Sense{{Gloss: t}}})
			}
		case map[string]any:
			switch getString(v["type"]) {
			case "structured-content":
				groups = append(groups, extractSenseGroups(v["content"])...)
				for _, f := range extractForms(v["content"]) {
					appendUnique(&forms, f)
				}
			case "text":
				t := strings.TrimSpace(getString(v["text"]))
				if t == "" {
					t = strings.TrimSpace(renderText(v["content"]))
				}
				if t != "" {
					groups = append(groups, store.SenseGroup{Senses: []store.Sense{{Gloss: t}}})
				}
			case "image":
				// unsupported in a text-only CLI, skip
			default:
				if _, ok := v["content"]; ok {
					groups = append(groups, extractSenseGroups(v["content"])...)
				}
			}
		}
	}

	for _, g := range groups {
		for _, p := range g.Pos {
			appendUnique(&pos, p)
		}
		for _, s := range g.Senses {
			// Split back into individual glosses (rather than keeping the
			// "; "-joined sense) so each one becomes its own space-bounded
			// token in the search index - that's what lets exact/boundary
			// matching in Search() actually work.
			for _, gl := range strings.Split(s.Gloss, "; ") {
				if gl = strings.TrimSpace(gl); gl != "" {
					defs = append(defs, gl)
				}
			}
			for _, ex := range s.Examples {
				examples = append(examples, ex.JP+"\n"+ex.EN)
			}
		}
	}
	return
}

// extractSenseGroups finds every sense-group in a structured-content tree
// and builds it into a store.SenseGroup. Dictionaries occasionally omit the
// sense-group wrapper for a single ungrouped sense, so as a fallback it
// looks for bare "sense" nodes too.
func extractSenseGroups(node any) []store.SenseGroup {
	groupNodes := findAllByMarker(node, "sense-group")
	if len(groupNodes) == 0 {
		if senseNodes := findAllByMarker(node, "sense"); len(senseNodes) > 0 {
			return []store.SenseGroup{buildSenseGroup(nil, senseNodes)}
		}
		return nil
	}

	var groups []store.SenseGroup
	for _, gn := range groupNodes {
		pos := collectPOS(gn["content"])
		senseNodes := findAllByMarker(gn["content"], "sense")
		groups = append(groups, buildSenseGroup(pos, senseNodes))
	}
	return groups
}

// extractForms reads a "forms" table (alternate spellings/readings for the
// entry, e.g. ＣＤプレーヤー vs ＣＤプレイヤー vs シーディープレーヤー) and
// returns every distinct spelling/reading it lists. The table's per-cell
// priority/validity markers aren't reconstructed here - for a text CLI just
// knowing the alternate forms exist is the useful part.
func extractForms(node any) []string {
	var out []string
	for _, formsNode := range findAllByMarker(node, "forms") {
		table := findTag(formsNode["content"], "table")
		if table == nil {
			continue
		}
		for _, row := range asSlice(table["content"]) {
			rm, ok := row.(map[string]any)
			if !ok {
				continue
			}
			for _, cell := range asSlice(rm["content"]) {
				cm, ok := cell.(map[string]any)
				if !ok || getString(cm["tag"]) != "th" {
					continue
				}
				if t := strings.TrimSpace(renderText(cm["content"])); t != "" {
					appendUnique(&out, t)
				}
			}
		}
	}
	return out
}

func findTag(content any, tag string) map[string]any {
	for _, it := range asSlice(content) {
		if m, ok := it.(map[string]any); ok && getString(m["tag"]) == tag {
			return m
		}
	}
	return nil
}

func collectPOS(node any) []string {
	var pos []string
	for _, pn := range findAllByMarker(node, "part-of-speech-info") {
		text := strings.TrimSpace(getString(pn["title"]))
		if text == "" {
			text = strings.TrimSpace(renderText(pn["content"]))
		}
		if text != "" {
			appendUnique(&pos, text)
		}
	}
	return pos
}

func buildSenseGroup(pos []string, senseNodes []map[string]any) store.SenseGroup {
	var senses []store.Sense
	for _, sn := range senseNodes {
		var glossParts, notes, xrefs []string
		var examples []store.Example
		walkSense(sn["content"], &glossParts, &notes, &xrefs, &examples)

		gloss := strings.Join(glossParts, "; ")
		if gloss == "" && len(notes) == 0 && len(xrefs) == 0 {
			continue
		}
		senses = append(senses, store.Sense{
			Gloss:    gloss,
			Notes:    notes,
			Xrefs:    xrefs,
			Examples: examples,
		})
	}
	return store.SenseGroup{Pos: pos, Senses: senses}
}

// walkSense descends a single sense's subtree, splitting its glossary text
// from the notes/xrefs that qualify it and the examples that illustrate it.
func walkSense(node any, gloss, notes, xrefs *[]string, examples *[]store.Example) {
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
				*examples = append(*examples, store.Example{JP: jp, EN: en})
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
