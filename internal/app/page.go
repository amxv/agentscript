package app

import (
	"errors"
	"flag"
	"fmt"
	"runtime"
	"strings"

	"github.com/amxv/agentscript/internal/transcript"
)

type blockSelection struct {
	slice, turnSlice      string
	from, to, last        int
	around, before, after int
	lastTurns             int
}

func newBlockSelection() blockSelection {
	return blockSelection{from: -1, to: -1, around: -1}
}

func (s blockSelection) selectBlocks(blocks []transcript.Block) ([]transcript.Block, error) {
	if s.lastTurns > 0 {
		return transcript.LastTurnBlocks(blocks, s.lastTurns), nil
	}
	if s.turnSlice != "" {
		spec, err := transcript.ParseSliceSpec(s.turnSlice)
		if err != nil {
			return nil, err
		}
		blocks = transcript.SliceBlocksByTurn(blocks, spec)
	}
	spec, err := transcript.ParseSliceSpec(s.slice)
	if err != nil {
		return nil, err
	}
	if s.slice == "" {
		if s.from >= 0 {
			spec.From = &s.from
		}
		if s.to >= 0 {
			spec.To = &s.to
		}
	}
	return transcript.SliceBlocks(blocks, spec, s.last, s.around, s.before, s.after), nil
}

type pageFlags struct {
	maxChars int
	// Record rendering flags before adding selection and discovery flags.
	renderFlags map[string]bool
}

func addPageFlags(fs *flag.FlagSet, page *pageFlags, selection *blockSelection) {
	page.renderFlags = make(map[string]bool)
	fs.VisitAll(func(f *flag.Flag) {
		page.renderFlags[f.Name] = f.Name != "out"
	})
	fs.IntVar(&selection.last, "last", 0, "show the last N blocks before display filters")
	fs.IntVar(&selection.lastTurns, "last-turns", 0, "show the last N user turns before display filters")
	fs.IntVar(&page.maxChars, "max-chars", 0, "limit rendered characters at whole-block boundaries; 0 disables")
}

func pageValueFlags() map[string]bool {
	flags := commonValueFlags()
	flags["last"] = true
	flags["last-turns"] = true
	flags["max-chars"] = true
	return flags
}

func (p pageFlags) validate(s blockSelection) error {
	if p.maxChars < 0 || s.last < 0 || s.lastTurns < 0 {
		return errors.New("--last, --last-turns, and --max-chars must be nonnegative")
	}
	if s.lastTurns > 0 && (s.last > 0 || s.around >= 0 || s.from >= 0 || s.to >= 0 || s.slice != "" || s.turnSlice != "") {
		return errors.New("--last-turns cannot be combined with another block or turn selection")
	}
	return nil
}

func (p pageFlags) nextCommand(tr transcript.Transcript, fs *flag.FlagSet) func(int, int) string {
	var flags []string
	fs.Visit(func(f *flag.Flag) {
		if p.renderFlags[f.Name] {
			_, isBool := f.Value.(interface{ IsBoolFlag() bool })
			if isBool {
				flags = append(flags, "--"+f.Name+"="+f.Value.String())
			} else {
				flags = append(flags, "--"+f.Name+" "+commandQuote(f.Value.String()))
			}
		}
	})
	// Interleaved ACP sessions can give a selected turn gaps in block space.
	// Carry its turn bounds into the continuation to preserve those gaps.
	for i := 1; i < len(tr.Blocks); i++ {
		if tr.Blocks[i].Index == tr.Blocks[i-1].Index+1 {
			continue
		}
		minTurn, maxTurn := tr.Blocks[0].Turn, tr.Blocks[0].Turn
		for _, block := range tr.Blocks {
			minTurn = min(minTurn, block.Turn)
			maxTurn = max(maxTurn, block.Turn)
		}
		flags = append(flags, fmt.Sprintf("--turn-slice %d:%d", minTurn, maxTurn))
		break
	}
	return func(index, budget int) string {
		end := tr.Blocks[len(tr.Blocks)-1].Index
		command := fmt.Sprintf("%s open --path %s --slice %d:%d --max-chars %d",
			commandName, commandQuote(tr.Path), index, end, budget)
		if len(flags) > 0 {
			command += " " + strings.Join(flags, " ")
		}
		return command
	}
}

func commandQuote(value string) string {
	escape := `'"'"'`
	if runtime.GOOS == "windows" {
		escape = "''" // PowerShell
	}
	return "'" + strings.ReplaceAll(value, "'", escape) + "'"
}
