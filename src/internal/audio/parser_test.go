package audio

import (
	"testing"
)

func TestParseMarkdown_MultiChapter(t *testing.T) {
	content := `# The Great Story
*A tale of wonder*
By Jane Doe
---
## Chapter One
First paragraph of chapter one.

Second paragraph of chapter one.

## Chapter Two
First paragraph of chapter two.

## Chapter Three
Third chapter content here.
`

	result, err := ParseMarkdownString(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Meta.Title != "The Great Story" {
		t.Errorf("title = %q, want %q", result.Meta.Title, "The Great Story")
	}
	if result.Meta.Tagline != "A tale of wonder" {
		t.Errorf("tagline = %q, want %q", result.Meta.Tagline, "A tale of wonder")
	}
	if result.Meta.Author != "Jane Doe" {
		t.Errorf("author = %q, want %q", result.Meta.Author, "Jane Doe")
	}

	if len(result.Chapters) != 3 {
		t.Fatalf("chapters = %d, want 3", len(result.Chapters))
	}

	ch1 := result.Chapters[0]
	if ch1.Index != 1 {
		t.Errorf("chapter 1 index = %d, want 1", ch1.Index)
	}
	if ch1.Title != "Chapter One" {
		t.Errorf("chapter 1 title = %q, want %q", ch1.Title, "Chapter One")
	}
	if ch1.Body != "First paragraph of chapter one.\n\nSecond paragraph of chapter one." {
		t.Errorf("chapter 1 body = %q", ch1.Body)
	}

	ch2 := result.Chapters[1]
	if ch2.Title != "Chapter Two" {
		t.Errorf("chapter 2 title = %q, want %q", ch2.Title, "Chapter Two")
	}
	if ch2.WordCount != 5 {
		t.Errorf("chapter 2 word count = %d, want 5", ch2.WordCount)
	}
}

func TestParseMarkdown_FrontmatterExtraction(t *testing.T) {
	content := `# My Novel
*An epic journey*
By John Smith
---
## First Chapter
Some text.
`

	result, err := ParseMarkdownString(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Meta.Title != "My Novel" {
		t.Errorf("title = %q, want %q", result.Meta.Title, "My Novel")
	}
	if result.Meta.Tagline != "An epic journey" {
		t.Errorf("tagline = %q, want %q", result.Meta.Tagline, "An epic journey")
	}
	if result.Meta.Author != "John Smith" {
		t.Errorf("author = %q, want %q", result.Meta.Author, "John Smith")
	}
}

func TestParseMarkdown_SingleChapter(t *testing.T) {
	content := `# Title
## Only Chapter
Hello world.
`

	result, err := ParseMarkdownString(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Chapters) != 1 {
		t.Fatalf("chapters = %d, want 1", len(result.Chapters))
	}
	if result.Chapters[0].Title != "Only Chapter" {
		t.Errorf("title = %q, want %q", result.Chapters[0].Title, "Only Chapter")
	}
}

func TestParseMarkdown_NoChapters(t *testing.T) {
	content := `# Title
Just some text without chapters.
`

	result, err := ParseMarkdownString(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result.Chapters) != 0 {
		t.Errorf("chapters = %d, want 0", len(result.Chapters))
	}
}

func TestParseMarkdown_Empty(t *testing.T) {
	result, err := ParseMarkdownString("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Meta.Title != "" {
		t.Errorf("title = %q, want empty", result.Meta.Title)
	}
	if len(result.Chapters) != 0 {
		t.Errorf("chapters = %d, want 0", len(result.Chapters))
	}
}

func TestParseMarkdown_NoTaglineOrAuthor(t *testing.T) {
	content := `# Simple Story
---
## Chapter One
Content.
`

	result, err := ParseMarkdownString(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Meta.Tagline != "" {
		t.Errorf("tagline = %q, want empty", result.Meta.Tagline)
	}
	if result.Meta.Author != "" {
		t.Errorf("author = %q, want empty", result.Meta.Author)
	}
}

func TestParseMarkdown_WordCount(t *testing.T) {
	content := `# Title
## Chapter
One two three four five six seven eight nine ten.
`

	result, err := ParseMarkdownString(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Chapters[0].WordCount != 10 {
		t.Errorf("word count = %d, want 10", result.Chapters[0].WordCount)
	}
}

func TestSlug(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"The Scourge of Aethelgard", "the-scourge-of-aethelgard"},
		{"Visions and Visions", "visions-and-visions"},
		{"A New Dawn, A Lingering Shadow", "a-new-dawn-a-lingering-shadow"},
		{"Soul's Burden", "souls-burden"},
		{"  Multiple   Spaces  ", "multiple-spaces"},
		{"Already-Slugged", "already-slugged"},
	}

	for _, tt := range tests {
		got := Slug(tt.input)
		if got != tt.want {
			t.Errorf("Slug(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
