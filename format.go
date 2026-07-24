package main

import (
	"fmt"
	"strings"

	"github.com/gojp/kana"
	"github.com/jim-ww/nihongo/store"
)

func formatList(r store.FtsDict) string {
	b := new(strings.Builder)

	fmt.Fprintf(b, "%s", r.Expression)

	if r.Reading != "" && r.Reading != r.Expression {
		fmt.Fprintf(b, " [%s]", r.Reading)
	} else if r.Reading == "" && (kana.IsKatakana(r.Expression) || kana.IsHiragana(r.Expression)) {
		// fallback
		fmt.Fprintf(b, " [%s]", r.Expression)
	} else if r.Reading == "" && r.ReadingRomaji != "" {
		// another fallback
		fmt.Fprintf(b, " [%s]", kana.RomajiToKatakana(r.ReadingRomaji))
	}

	if *romajiFlag && r.ReadingRomaji != "" {
		fmt.Fprintf(b, " %s", r.ReadingRomaji)
	}

	if len(r.Definitions) > 0 {
		first := strings.TrimSpace(r.Definitions[0])
		if first != "" && len(first) < 70 {
			fmt.Fprintf(b, " → %s", first)
		}
	}

	if r.TermTags != "" {
		fmt.Fprintf(b, " [%s]", r.TermTags)
	}

	return b.String()
}

func formatEntry(r store.FtsDict) string {
	b := new(strings.Builder)

	fmt.Fprintf(b, "%s", r.Expression)

	if r.Reading != "" && r.Reading != r.Expression {
		fmt.Fprintf(b, " [%s]", r.Reading)
	} else if r.Reading == "" && (kana.IsKatakana(r.Expression) || kana.IsHiragana(r.Expression)) {
		// fallback
		fmt.Fprintf(b, " [%s]", r.Expression)
	} else if r.Reading == "" && r.ReadingRomaji != "" {
		// another fallback
		fmt.Fprintf(b, " [%s]", kana.RomajiToKatakana(r.ReadingRomaji))
	}

	if r.ReadingRomaji != "" {
		fmt.Fprintf(b, "  %s", r.ReadingRomaji)
	}
	fmt.Fprintln(b)

	if r.TermTags != "" || r.DefinitionTags != "" {
		tags := r.TermTags
		if r.DefinitionTags != "" {
			if tags != "" {
				tags += " | "
			}
			tags += r.DefinitionTags
		}
		fmt.Fprintf(b, "Tags: %s\n", tags)
	}

	if len(r.Forms) > 0 {
		fmt.Fprintf(b, "Also written/read: %s\n", strings.Join(r.Forms, ", "))
	}

	fmt.Fprintln(b)

	if len(r.Groups) > 0 {
		formatGroups(b, r.Groups)
	} else {
		formatFlatDefinitions(b, r)
	}

	return b.String()
}

// formatGroups renders senses under the part-of-speech heading they share,
// mirroring how the dictionary itself groups meanings, with each sense's
// notes/cross-references/examples nested beneath it instead of flattened
// into the definition list.
func formatGroups(b *strings.Builder, groups []store.SenseGroup) {
	n := 0
	for _, g := range groups {
		if len(g.Senses) == 0 {
			continue
		}
		if len(g.Pos) > 0 {
			fmt.Fprintf(b, "%s\n", strings.Join(g.Pos, ", "))
		}
		for _, s := range g.Senses {
			n++
			fmt.Fprintf(b, "  %d. %s\n", n, s.Gloss)
			for _, note := range s.Notes {
				fmt.Fprintf(b, "       %s\n", note)
			}
			for _, xref := range s.Xrefs {
				fmt.Fprintf(b, "       %s\n", xref)
			}
			for _, ex := range s.Examples {
				if ex.JP != "" {
					fmt.Fprintf(b, "       e.g. %s\n", ex.JP)
				}
				if ex.EN != "" {
					fmt.Fprintf(b, "            %s\n", ex.EN)
				}
			}
		}
		fmt.Fprintln(b)
	}
}

// formatFlatDefinitions is the fallback for glossaries that never had
// structured-content grouping to begin with (plain-string glossaries).
func formatFlatDefinitions(b *strings.Builder, r store.FtsDict) {
	if len(r.Pos) > 0 {
		fmt.Fprintf(b, "Part of Speech: %s\n\n", strings.Join(r.Pos, ", "))
	}

	if len(r.Definitions) > 0 {
		fmt.Fprintln(b, "Definitions:")
		for i, def := range r.Definitions {
			trimmed := strings.TrimSpace(def)
			if trimmed != "" {
				fmt.Fprintf(b, "  %d. %s\n", i+1, trimmed)
			}
		}
		fmt.Fprintln(b)
	}

	if len(r.Examples) > 0 {
		fmt.Fprintln(b, "Examples:")
		for i, ex := range r.Examples {
			lines := strings.Split(ex, "\n")
			for j, line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}
				if j == 0 {
					fmt.Fprintf(b, "  %d. %s\n", i+1, trimmed)
				} else {
					fmt.Fprintf(b, "     %s\n", trimmed)
				}
			}
			fmt.Fprintln(b)
		}
	}
}
