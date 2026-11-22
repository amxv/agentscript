package transcript

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestRenderPageUsesFormattedCharacterCount(t *testing.T) {
	tr := Transcript{Path: "my & é session.jsonl", Provider: ProviderClaude}
	for i := range 11 {
		tr.Blocks = append(tr.Blocks, Block{
			Index: i, Kind: KindAssistant, Text: "é < & 😀\nfolded", Provider: ProviderClaude,
		})
	}
	for _, opts := range []RenderOptions{
		{MaxLines: 1},
		{Format: "md", MarkdownStyle: "audit", MaxLines: 1},
		{Format: "html", MaxLines: 1},
		{Format: "json"},
	} {
		t.Run(opts.normalizedFormat(), func(t *testing.T) {
			prefix := tr
			prefix.Blocks = tr.Blocks[:10]
			var expected bytes.Buffer
			if err := Render(&expected, prefix, opts); err != nil {
				t.Fatal(err)
			}
			budget := utf8.RuneCount(expected.Bytes())
			for _, limit := range []int{budget, budget - 1} {
				var page bytes.Buffer
				want := 10
				if limit < budget {
					want = 9
				}
				next := func(index, gotBudget int) string {
					if index != want || gotBudget != limit {
						t.Fatalf("continuation starts at %d with budget %d, want %d and %d", index, gotBudget, want, limit)
					}
					return fmt.Sprintf("resume %d", index)
				}
				if err := RenderPage(&page, tr, opts, limit, next); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(page.String(), fmt.Sprintf("resume %d", want)) {
					t.Fatalf("missing continuation for block %d", want)
				}
			}
		})
	}
}
