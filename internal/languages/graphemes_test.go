package languages

import (
	"context"
	"strings"
	"testing"
)

func TestPinnedExtendedGraphemeBoundaries(t *testing.T) {
	for _, text := range []string{"\u00e9", "e\u0301", "e\u0301\u0323", "\U0001f600", "\U0001f1fa\U0001f1e6", "\U0001f469\u200d\U0001f4bb", "\U0001f44d\U0001f3fd"} {
		g, err := SegmentGraphemes(context.Background(), text)
		if err != nil || len(g) != 1 || g[0].Text != text || g[0].ByteStart != 0 || g[0].ByteEnd != len(text) {
			t.Fatalf("incorrect cluster %q: %+v %v", text, g, err)
		}
	}
	g, err := SegmentGraphemes(context.Background(), "A\U0001f469\u200d\U0001f4bbe\u0301")
	if err != nil || len(g) != 3 || g[1].UTF16Start != 1 || g[1].UTF16End != 6 || g[2].UTF16Start != 6 || g[2].UTF16End != 8 {
		t.Fatal("UTF-16 boundaries conflated", g, err)
	}
	if g, err = SegmentGraphemes(context.Background(), ""); err != nil || len(g) != 0 {
		t.Fatal(g, err)
	}
	for _, bad := range []string{string([]byte{0xff}), strings.Repeat("a", 513), "e" + strings.Repeat("\u0301", 2048)} {
		if _, err := SegmentGraphemes(context.Background(), bad); err == nil {
			t.Fatal("invalid/oversized text accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := SegmentGraphemes(ctx, ""); err == nil {
		t.Fatal("empty input ignored cancellation")
	}
}
