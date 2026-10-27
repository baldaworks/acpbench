package acpbench

import (
	"testing"
	"time"

	acp "github.com/coder/acp-go-sdk"
)

func TestResolveTokenCounts(t *testing.T) {
	t.Parallel()

	thoughtTokens := 40
	tests := []struct {
		name         string
		usage        *acp.Usage
		messageChars int
		thoughtChars int
		wantOutput   int
		wantThought  int
	}{
		{
			name:         "reported output and thought tokens",
			usage:        &acp.Usage{OutputTokens: 100, ThoughtTokens: &thoughtTokens},
			messageChars: 8,
			thoughtChars: 8,
			wantOutput:   100,
			wantThought:  40,
		},
		{
			name:         "estimate output and thought separately",
			messageChars: 9,
			thoughtChars: 5,
			wantOutput:   3,
			wantThought:  2,
		},
		{
			name:         "reported output with streamed thought fallback",
			usage:        &acp.Usage{OutputTokens: 12},
			messageChars: 40,
			thoughtChars: 9,
			wantOutput:   12,
			wantThought:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			output, thought := resolveTokenCounts(tt.usage, tt.messageChars, tt.thoughtChars)
			if output != tt.wantOutput || thought != tt.wantThought {
				t.Fatalf("resolveTokenCounts() = (%d, %d), want (%d, %d)", output, thought, tt.wantOutput, tt.wantThought)
			}
		})
	}
}

func TestResolveGenerationStart(t *testing.T) {
	t.Parallel()

	promptStart := time.Unix(100, 0)
	messageStart := promptStart.Add(3 * time.Second)
	thoughtStart := promptStart.Add(time.Second)

	tests := []struct {
		name              string
		firstMessageChunk time.Time
		firstThoughtChunk time.Time
		thoughtTokens     int
		want              time.Time
	}{
		{
			name:              "visible reasoning starts generation timer",
			firstMessageChunk: messageStart,
			firstThoughtChunk: thoughtStart,
			thoughtTokens:     40,
			want:              thoughtStart,
		},
		{
			name:              "hidden reasoning starts timer at prompt dispatch",
			firstMessageChunk: messageStart,
			thoughtTokens:     40,
			want:              promptStart,
		},
		{
			name:              "output only starts timer at first message",
			firstMessageChunk: messageStart,
			want:              messageStart,
		},
		{
			name:              "earliest generated chunk wins",
			firstMessageChunk: thoughtStart,
			firstThoughtChunk: messageStart,
			thoughtTokens:     40,
			want:              thoughtStart,
		},
		{
			name: "no chunks falls back to prompt dispatch",
			want: promptStart,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := resolveGenerationStart(promptStart, tt.firstMessageChunk, tt.firstThoughtChunk, tt.thoughtTokens)
			if !got.Equal(tt.want) {
				t.Fatalf("resolveGenerationStart() = %v, want %v", got, tt.want)
			}
		})
	}
}
