package languages

import (
	"context"
	"fmt"
	"github.com/rivo/uniseg"
	"unicode/utf16"
	"unicode/utf8"
)

type GraphemeCluster struct {
	Index      int      `json:"index"`
	Text       string   `json:"text"`
	ByteStart  int      `json:"byteStart"`
	ByteEnd    int      `json:"byteEnd"`
	UTF16Start int      `json:"utf16Start"`
	UTF16End   int      `json:"utf16End"`
	CodePoints []string `json:"codePoints"`
}

func SegmentGraphemes(ctx context.Context, text string) ([]GraphemeCluster, error) {
	if !utf8.ValidString(text) {
		return nil, productiveError("invalid_unicode", "", "text", "Input must be valid UTF-8.")
	}
	if len(text) > 4096 {
		return nil, productiveError("execution_limit", "", "text", "Text exceeds 4 KiB.")
	}
	out := []GraphemeCluster{}
	iterator := uniseg.NewGraphemes(text)
	u16 := 0
	for iterator.Next() {
		if err := ctx.Err(); err != nil {
			return nil, productiveError("cancelled", "", "text", err.Error())
		}
		start, end := iterator.Positions()
		cluster := GraphemeCluster{Index: len(out), Text: iterator.Str(), ByteStart: start, ByteEnd: end, UTF16Start: u16, CodePoints: []string{}}
		for _, r := range cluster.Text {
			if err := ctx.Err(); err != nil {
				return nil, productiveError("cancelled", "", "text", err.Error())
			}
			cluster.CodePoints = append(cluster.CodePoints, fmt.Sprintf("U+%04X", r))
			u16 += utf16.RuneLen(r)
		}
		cluster.UTF16End = u16
		out = append(out, cluster)
		if len(out) > 512 {
			return nil, productiveError("execution_limit", "", "text", "Text exceeds 512 grapheme clusters.")
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, productiveError("cancelled", "", "text", err.Error())
	}
	return out, nil
}
