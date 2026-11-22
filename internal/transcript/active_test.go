package transcript

import (
	"strings"
	"testing"
)

func TestParseActiveTranscript(t *testing.T) {
	tests := []struct {
		provider Provider
		prefix   string
		first    string
		second   string
	}{
		{
			provider: ProviderClaude,
			first:    `{"type":"user","message":{"content":"first"}}`,
			second:   `{"type":"assistant","message":{"content":[{"type":"text","text":"second"}]}}`,
		},
		{
			provider: ProviderCodex,
			prefix:   "{\"type\":\"session_meta\",\"payload\":{}}\n",
			first:    `{"type":"event_msg","payload":{"type":"user_message","message":"first"}}`,
			second:   `{"type":"event_msg","payload":{"type":"user_message","message":"second"}}`,
		},
	}
	for _, tt := range tests {
		t.Run(string(tt.provider), func(t *testing.T) {
			prefix := tt.prefix + tt.first + "\nmalformed interior line\n"
			partial, err := Parse([]byte(prefix + tt.second[:len(tt.second)-1]))
			if err != nil || len(partial.Blocks) != 1 || partial.Blocks[0].Text != "first" {
				t.Fatalf("partial transcript = %+v, %v", partial, err)
			}
			complete, err := Parse([]byte(prefix + tt.second))
			if err != nil || len(complete.Blocks) != 2 || complete.Blocks[1].Index != 1 || complete.Blocks[1].Text != "second" {
				t.Fatalf("completed transcript = %+v, %v", complete, err)
			}
		})
	}
}

func TestParseSkipsOversizeUnterminatedLine(t *testing.T) {
	first := `{"type":"user","message":{"content":"first"}}` + "\n"
	tail := `{"unfinished":"` + strings.Repeat("x", maxJSONLineSize)
	tr, err := Parse([]byte(first + tail))
	if err != nil || len(tr.Blocks) != 1 {
		t.Fatalf("oversized partial transcript = %+v, %v", tr, err)
	}
	if _, err := Parse([]byte(first + tail + "\n")); err == nil {
		t.Fatal("oversized terminated line should retain the scanner error")
	}
}
