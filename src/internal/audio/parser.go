package audio

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// StoryMeta contains metadata extracted from the markdown frontmatter.
type StoryMeta struct {
	Title   string // H1 heading
	Tagline string // italic line, if any
	Author  string // "By <name>", if any
}

// Chapter represents a single chapter extracted from the markdown.
type Chapter struct {
	Index     int
	Title     string // H2 heading text
	Body      string // prose paragraphs joined by \n\n
	WordCount int
}

// ParseResult contains the full parsed output of a markdown story.
type ParseResult struct {
	Meta     StoryMeta
	Chapters []Chapter
}

// ParseMarkdown reads a markdown file and extracts story metadata and chapters.
// It expects H1 for the title and H2 for chapter boundaries.
func ParseMarkdown(filePath string) (*ParseResult, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	return parseMarkdownReader(bufio.NewScanner(f))
}

// ParseMarkdownString parses markdown content from a string (for testing).
func ParseMarkdownString(content string) (*ParseResult, error) {
	return parseMarkdownReader(bufio.NewScanner(strings.NewReader(content)))
}

func parseMarkdownReader(scanner *bufio.Scanner) (*ParseResult, error) {
	result := &ParseResult{}

	var (
		inFrontmatter  = true
		currentChapter *Chapter
		chapterIndex   = 0
		bodyLines      []string
	)

	flushChapter := func() {
		if currentChapter == nil {
			return
		}
		body := strings.TrimSpace(strings.Join(bodyLines, "\n"))
		currentChapter.Body = body
		currentChapter.WordCount = countWords(body)
		result.Chapters = append(result.Chapters, *currentChapter)
		currentChapter = nil
		bodyLines = nil
	}

	for scanner.Scan() {
		line := scanner.Text()

		// H1 title (only the first one)
		if strings.HasPrefix(line, "# ") && !strings.HasPrefix(line, "## ") {
			if result.Meta.Title == "" {
				result.Meta.Title = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			}
			continue
		}

		// H2 chapter heading
		if strings.HasPrefix(line, "## ") {
			flushChapter()
			inFrontmatter = false
			chapterIndex++
			currentChapter = &Chapter{
				Index: chapterIndex,
				Title: strings.TrimSpace(strings.TrimPrefix(line, "## ")),
			}
			bodyLines = nil
			continue
		}

		// Frontmatter lines (between title and first chapter)
		if inFrontmatter {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || trimmed == "---" {
				continue
			}
			// Italic tagline: *text*
			if strings.HasPrefix(trimmed, "*") && strings.HasSuffix(trimmed, "*") && !strings.HasPrefix(trimmed, "**") {
				result.Meta.Tagline = strings.Trim(trimmed, "*")
				continue
			}
			// Author line: "By Name"
			if strings.HasPrefix(trimmed, "By ") {
				result.Meta.Author = strings.TrimPrefix(trimmed, "By ")
				continue
			}
			continue
		}

		// Chapter body lines
		if currentChapter != nil {
			bodyLines = append(bodyLines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading markdown: %w", err)
	}

	flushChapter()

	return result, nil
}

// countWords counts words in a string by splitting on whitespace.
func countWords(s string) int {
	return len(strings.Fields(s))
}

// Slug converts a title to a URL-friendly slug.
func Slug(s string) string {
	s = strings.ToLower(s)
	s = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		if r == ' ' || r == '-' || r == '_' {
			return '-'
		}
		return -1
	}, s)
	// Collapse multiple dashes
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return strings.Trim(s, "-")
}
