package transcript

import (
	"encoding/json"
	"io"
	"strconv"
	"unicode/utf8"
)

type continuation struct {
	Index   int    `json:"index"`
	Command string `json:"command"`
}

// RenderPage caps rendered transcript content at a whole-block boundary.
// Fixed framing and continuation metadata are always emitted. If the first
// block alone exceeds the budget, nextCommand receives the minimum budget
// needed to include that block on the next page.
func RenderPage(w io.Writer, tr Transcript, opts RenderOptions, budget int, nextCommand func(index, budget int) string) error {
	blocks := FilterBlocks(tr.Blocks, opts)
	var framing charCount
	if err := renderPage(&framing, tr, blocks[:0], opts, nil); err != nil {
		return err
	}
	size := int(framing)
	format := opts.normalizedFormat()
	for i, block := range blocks {
		count, err := renderedBlockSize(tr, block, opts, i == 0)
		if err != nil {
			return err
		}
		size += count
		required := size
		if format == "html" || (format == "md" || format == "markdown") && markdownStyle(opts) == "audit" {
			// The empty framing includes a single digit for the block count.
			required += len(strconv.Itoa(i+1)) - 1
		}
		if required > budget {
			if i == 0 {
				budget = required
			}
			next := continuation{Index: block.Index, Command: nextCommand(block.Index, budget)}
			return renderPage(w, tr, blocks[:i], opts, &next)
		}
	}
	return renderPage(w, tr, blocks, opts, nil)
}

func renderedBlockSize(tr Transcript, block Block, opts RenderOptions, first bool) (int, error) {
	var count charCount
	switch opts.normalizedFormat() {
	case "json":
		data, err := json.MarshalIndent(block, "    ", "  ")
		if err != nil {
			return 0, err
		}
		// Each block adds a newline and four spaces before the object.
		// The first also adds whitespace before the closing bracket.
		// Subsequent blocks add a comma after the previous object.
		separator := 6
		if first {
			separator = 8
		}
		return utf8.RuneCount(data) + separator, nil
	case "md", "markdown":
		renderMarkdownBlock(&count, tr, block, opts, markdownStyle(opts))
	case "html":
		renderHTMLBlock(&count, tr, block, opts)
	default:
		renderTextBlock(&count, tr, block, opts)
	}
	return int(count), nil
}

type charCount int

func (c *charCount) Write(p []byte) (int, error) {
	*c += charCount(utf8.RuneCount(p))
	return len(p), nil
}
