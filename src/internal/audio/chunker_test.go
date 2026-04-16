package audio

import (
	"strings"
	"testing"
)

func TestChunkChapter_ShortText(t *testing.T) {
	body := "This is a short paragraph."
	segments := ChunkChapter(body, 1, ChunkOpts{})

	if len(segments) != 1 {
		t.Fatalf("segments = %d, want 1", len(segments))
	}
	if segments[0].Text != body {
		t.Errorf("text = %q, want %q", segments[0].Text, body)
	}
	if segments[0].Index != 1 {
		t.Errorf("index = %d, want 1", segments[0].Index)
	}
}

func TestChunkChapter_ParagraphBoundarySplit(t *testing.T) {
	// Create two paragraphs, each under limits but combined over limit
	para1 := strings.Repeat("word ", 300) // 300 words
	para2 := strings.Repeat("word ", 300) // 300 words
	body := para1 + "\n\n" + para2

	segments := ChunkChapter(body, 1, ChunkOpts{MaxWords: 400, MaxBytes: 10000})

	if len(segments) != 2 {
		t.Fatalf("segments = %d, want 2", len(segments))
	}
	if segments[0].Index != 1 {
		t.Errorf("first index = %d, want 1", segments[0].Index)
	}
	if segments[1].Index != 2 {
		t.Errorf("second index = %d, want 2", segments[1].Index)
	}
}

func TestChunkChapter_LongParagraph_SentenceSplit(t *testing.T) {
	// Build a single paragraph that exceeds word limit with multiple sentences
	sentences := make([]string, 10)
	for i := range sentences {
		sentences[i] = strings.Repeat("word ", 60) + "end."
	}
	body := strings.Join(sentences, " ") // Single paragraph, ~610 words

	segments := ChunkChapter(body, 1, ChunkOpts{MaxWords: 200, MaxBytes: 10000})

	if len(segments) < 2 {
		t.Fatalf("segments = %d, want >= 2", len(segments))
	}

	// Verify all segments are within limits
	for i, seg := range segments {
		words := len(strings.Fields(seg.Text))
		if words > 200 {
			t.Errorf("segment %d has %d words, exceeds 200", i, words)
		}
	}
}

func TestChunkChapter_ByteLimit(t *testing.T) {
	// "Short text." = 11 bytes, "Another short text." = 19 bytes
	// With MaxBytes=25, first para fits, second para exceeds so splits at paragraph boundary
	body := "Short text.\n\nAnother short text."
	segments := ChunkChapter(body, 1, ChunkOpts{MaxBytes: 25, MaxWords: 1000})

	if len(segments) != 2 {
		t.Fatalf("segments = %d, want 2", len(segments))
	}
	if segments[0].Text != "Short text." {
		t.Errorf("segment 0 = %q", segments[0].Text)
	}
	if segments[1].Text != "Another short text." {
		t.Errorf("segment 1 = %q", segments[1].Text)
	}
}

func TestChunkChapter_UTF8ByteCounting(t *testing.T) {
	// \u00e9 (é) is 2 bytes in UTF-8, so 1000 chars = 2000 bytes
	unicodeText := strings.Repeat("\u00e9", 1000)
	body := unicodeText + "\n\n" + "short"

	// 2000 + 2 (\n\n) + 5 = 2007 bytes total, fits in 2500
	segments := ChunkChapter(body, 1, ChunkOpts{MaxBytes: 2500, MaxWords: 10000})
	if len(segments) != 1 {
		t.Fatalf("segments = %d, want 1 (total bytes: 2000 + 2 + 5 = 2007)", len(segments))
	}

	// With a tighter limit, should split at paragraph boundary
	segments = ChunkChapter(body, 1, ChunkOpts{MaxBytes: 2001, MaxWords: 10000})
	if len(segments) != 2 {
		t.Fatalf("segments = %d, want 2", len(segments))
	}
}

func TestChunkChapter_SequentialIndexing(t *testing.T) {
	para := strings.Repeat("word ", 100)
	body := para + "\n\n" + para + "\n\n" + para

	// Start at index 5
	segments := ChunkChapter(body, 5, ChunkOpts{MaxWords: 150, MaxBytes: 10000})

	for i, seg := range segments {
		expected := 5 + i
		if seg.Index != expected {
			t.Errorf("segment %d: index = %d, want %d", i, seg.Index, expected)
		}
	}
}

func TestChunkChapter_TextReconstruction(t *testing.T) {
	body := "First paragraph here.\n\nSecond paragraph here.\n\nThird paragraph here."

	segments := ChunkChapter(body, 1, ChunkOpts{MaxBytes: 30, MaxWords: 1000})

	// Reconstruct and compare (chunks split at paragraph boundaries should
	// reconstruct to the original when joined with \n\n)
	var parts []string
	for _, seg := range segments {
		parts = append(parts, seg.Text)
	}
	reconstructed := strings.Join(parts, "\n\n")

	if reconstructed != body {
		t.Errorf("reconstructed text does not match original\ngot:  %q\nwant: %q", reconstructed, body)
	}
}

func TestChunkChapter_Empty(t *testing.T) {
	segments := ChunkChapter("", 1, ChunkOpts{})
	if len(segments) != 0 {
		t.Errorf("segments = %d, want 0", len(segments))
	}
}

func TestChunkChapter_WhitespaceOnly(t *testing.T) {
	segments := ChunkChapter("   \n\n   ", 1, ChunkOpts{})
	if len(segments) != 0 {
		t.Errorf("segments = %d, want 0", len(segments))
	}
}

func TestSplitSentences(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"Hello world.", 1},
		{"Hello. World.", 2},
		{"Hello! World? Yes.", 3},
		{"Dr. Smith went home.", 2}, // "Dr." followed by space triggers split
		{"No punctuation at end", 1},
	}

	for _, tt := range tests {
		got := splitSentences(tt.input)
		if len(got) != tt.want {
			t.Errorf("splitSentences(%q) = %d sentences %v, want %d", tt.input, len(got), got, tt.want)
		}
	}
}
