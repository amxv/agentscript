package transcript

import (
	"fmt"
	"strconv"
	"strings"
)

type SliceSpec struct {
	From *int
	To   *int
}

func ParseSliceSpec(spec string) (SliceSpec, error) {
	if strings.TrimSpace(spec) == "" {
		return SliceSpec{}, nil
	}
	parts := strings.Split(spec, ":")
	if len(parts) != 2 {
		return SliceSpec{}, fmt.Errorf("slice must look like 0:100, 100:, or :50")
	}
	var out SliceSpec
	if strings.TrimSpace(parts[0]) != "" {
		v, err := strconv.Atoi(strings.TrimSpace(parts[0]))
		if err != nil || v < 0 {
			return SliceSpec{}, fmt.Errorf("invalid slice start %q", parts[0])
		}
		out.From = &v
	}
	if strings.TrimSpace(parts[1]) != "" {
		v, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || v < 0 {
			return SliceSpec{}, fmt.Errorf("invalid slice end %q", parts[1])
		}
		out.To = &v
	}
	return out, nil
}

func SliceBlocks(blocks []Block, spec SliceSpec, last, around, before, after int) []Block {
	if last > 0 {
		start := len(blocks) - last
		if start < 0 {
			start = 0
		}
		return blocks[start:]
	}
	if around >= 0 {
		start := around - before
		end := around + after
		if start < 0 {
			start = 0
		}
		if end >= len(blocks) {
			end = len(blocks) - 1
		}
		if start > end || len(blocks) == 0 {
			return nil
		}
		return blocks[start : end+1]
	}
	start := 0
	end := len(blocks)
	if spec.From != nil {
		start = *spec.From
	}
	if spec.To != nil {
		end = *spec.To + 1
	}
	if start < 0 {
		start = 0
	}
	if end > len(blocks) {
		end = len(blocks)
	}
	if start > end {
		return nil
	}
	return blocks[start:end]
}

func SliceBlocksByTurn(blocks []Block, spec SliceSpec) []Block {
	start := 0
	end := int(^uint(0) >> 1)
	if spec.From != nil {
		start = *spec.From
	}
	if spec.To != nil {
		end = *spec.To
	}
	out := make([]Block, 0, len(blocks))
	for _, b := range blocks {
		if b.Turn >= start && b.Turn <= end {
			out = append(out, b)
		}
	}
	return out
}
