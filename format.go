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
		fmt.Fprintf(b, "Tags: %s\n\n", tags)
	}

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
			for line := range strings.SplitSeq(ex, "\n") {
				trimmed := strings.TrimSpace(line)
				if trimmed != "" {
					fmt.Fprintf(b, "  %d. %s\n", i+1, trimmed)
				}
			}
			fmt.Fprintln(b)
		}
	}

	return b.String()
}
