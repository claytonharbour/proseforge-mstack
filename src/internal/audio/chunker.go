package audio

import (
	"strings"

	"github.com/claytonharbour/proseforge-mstack/src/internal/types"
)

const (
	// DefaultMaxChunkBytes is a conservative limit below Cloud TTS's 4000-byte limit.
	DefaultMaxChunkBytes = 3000
	// DefaultMaxChunkWords keeps chunks at roughly 500 words (~3000 bytes of English prose).
	DefaultMaxChunkWords = 500
)

// ChunkOpts controls the chunking limits.
type ChunkOpts struct {
	MaxBytes int
	MaxWords int
}

func (o ChunkOpts) withDefaults() ChunkOpts {
	if o.MaxBytes <= 0 {
		o.MaxBytes = DefaultMaxChunkBytes
	}
	if o.MaxWords <= 0 {
		o.MaxWords = DefaultMaxChunkWords
	}
	return o
}

// ChunkChapter splits a chapter body into segments that respect byte and word limits.
// startIndex is the segment index for the first chunk (allows sequential numbering
// across chapters). Returns segments with Index set sequentially from startIndex.
func ChunkChapter(body string, startIndex int, opts ChunkOpts) []types.Segment {
	opts = opts.withDefaults()
	body = strings.TrimSpace(body)
	if body == "" {
		return nil
	}

	paragraphs := splitParagraphs(body)
	var segments []types.Segment
	idx := startIndex

	var accum []string
	accumBytes := 0
	accumWords := 0

	flush := func() {
		if len(accum) == 0 {
			return
		}
		text := strings.Join(accum, "\n\n")
		segments = append(segments, types.Segment{
			Index: idx,
			Text:  text,
		})
		idx++
		accum = nil
		accumBytes = 0
		accumWords = 0
	}

	for _, para := range paragraphs {
		paraBytes := len([]byte(para))
		paraWords := len(strings.Fields(para))

		// If this single paragraph exceeds the limit, flush current accumulator
		// then split the paragraph by sentences
		if paraBytes > opts.MaxBytes || paraWords > opts.MaxWords {
			flush()
			sentenceSegments := splitLargeParagraph(para, idx, opts)
			segments = append(segments, sentenceSegments...)
			idx += len(sentenceSegments)
			continue
		}

		// Check if adding this paragraph would exceed limits
		newBytes := accumBytes + paraBytes
		newWords := accumWords + paraWords
		if len(accum) > 0 {
			newBytes += 2 // for "\n\n" separator
		}

		if newBytes > opts.MaxBytes || newWords > opts.MaxWords {
			flush()
		}

		accum = append(accum, para)
		if accumBytes > 0 {
			accumBytes += 2 // "\n\n" separator
		}
		accumBytes += paraBytes
		accumWords += paraWords
	}

	flush()
	return segments
}

// splitParagraphs splits text on double newlines, trimming each paragraph.
func splitParagraphs(text string) []string {
	raw := strings.Split(text, "\n\n")
	var out []string
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// splitLargeParagraph splits an oversized paragraph at sentence boundaries.
func splitLargeParagraph(para string, startIndex int, opts ChunkOpts) []types.Segment {
	sentences := splitSentences(para)
	var segments []types.Segment
	idx := startIndex

	var accum []string
	accumBytes := 0
	accumWords := 0

	flush := func() {
		if len(accum) == 0 {
			return
		}
		text := strings.Join(accum, " ")
		segments = append(segments, types.Segment{
			Index: idx,
			Text:  text,
		})
		idx++
		accum = nil
		accumBytes = 0
		accumWords = 0
	}

	for _, sent := range sentences {
		sentBytes := len([]byte(sent))
		sentWords := len(strings.Fields(sent))

		// If a single sentence exceeds limits, split at word boundaries
		if sentBytes > opts.MaxBytes || sentWords > opts.MaxWords {
			flush()
			wordSegments := splitAtWords(sent, idx, opts)
			segments = append(segments, wordSegments...)
			idx += len(wordSegments)
			continue
		}

		newBytes := accumBytes + sentBytes
		newWords := accumWords + sentWords
		if len(accum) > 0 {
			newBytes++ // space separator
		}

		if newBytes > opts.MaxBytes || newWords > opts.MaxWords {
			flush()
		}

		accum = append(accum, sent)
		if accumBytes > 0 {
			accumBytes++ // space separator
		}
		accumBytes += sentBytes
		accumWords += sentWords
	}

	flush()
	return segments
}

// splitSentences splits text at sentence boundaries (. ! ?), preserving the
// punctuation with the sentence.
func splitSentences(text string) []string {
	var sentences []string
	var current strings.Builder

	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		current.WriteRune(runes[i])

		// Check for sentence-ending punctuation followed by space or end of text
		if runes[i] == '.' || runes[i] == '!' || runes[i] == '?' {
			// Look ahead: is it end of text or followed by a space?
			if i+1 >= len(runes) || runes[i+1] == ' ' {
				s := strings.TrimSpace(current.String())
				if s != "" {
					sentences = append(sentences, s)
				}
				current.Reset()
				// Skip the space after punctuation
				if i+1 < len(runes) && runes[i+1] == ' ' {
					i++
				}
			}
		}
	}

	// Remaining text
	s := strings.TrimSpace(current.String())
	if s != "" {
		sentences = append(sentences, s)
	}

	return sentences
}

// splitAtWords splits text at word boundaries to fit within limits.
func splitAtWords(text string, startIndex int, opts ChunkOpts) []types.Segment {
	words := strings.Fields(text)
	var segments []types.Segment
	idx := startIndex

	var accum []string
	accumBytes := 0

	flush := func() {
		if len(accum) == 0 {
			return
		}
		t := strings.Join(accum, " ")
		segments = append(segments, types.Segment{
			Index: idx,
			Text:  t,
		})
		idx++
		accum = nil
		accumBytes = 0
	}

	for _, w := range words {
		wBytes := len([]byte(w))
		newBytes := accumBytes + wBytes
		if len(accum) > 0 {
			newBytes++ // space
		}

		if len(accum) > 0 && (newBytes > opts.MaxBytes || len(accum) >= opts.MaxWords) {
			flush()
		}

		accum = append(accum, w)
		if accumBytes > 0 {
			accumBytes++ // space
		}
		accumBytes += wBytes
	}

	flush()
	return segments
}
