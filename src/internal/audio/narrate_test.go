package audio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunNarrate_DryRun(t *testing.T) {
	// Create a temp markdown file
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "story.md")

	content := `# Test Story
*A test tagline*
By Test Author
---
## Chapter One
First paragraph of chapter one. This has some words to count.

Second paragraph of chapter one.

## Chapter Two
Content of chapter two goes here with more words.
`

	if err := os.WriteFile(mdPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	result, err := RunNarrate(NarrateOpts{
		InputFile: mdPath,
		OutputDir: filepath.Join(dir, "output"),
		DryRun:    true,
		Engine:    "say",
	})
	if err != nil {
		t.Fatalf("RunNarrate dry run failed: %v", err)
	}

	if result.StoryTitle != "Test Story" {
		t.Errorf("title = %q, want %q", result.StoryTitle, "Test Story")
	}
	if result.ChapterCount != 2 {
		t.Errorf("chapter count = %d, want 2", result.ChapterCount)
	}
	if result.TotalWords == 0 {
		t.Error("total words should be > 0")
	}
	if result.TotalChunks == 0 {
		t.Error("total chunks should be > 0")
	}

	// Dry run should not create any files
	if _, err := os.Stat(filepath.Join(dir, "output")); !os.IsNotExist(err) {
		t.Error("dry run should not create output directory")
	}
}

func TestRunNarrate_NoChapters(t *testing.T) {
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "story.md")

	content := `# Title Only
No chapters here.
`

	if err := os.WriteFile(mdPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := RunNarrate(NarrateOpts{
		InputFile: mdPath,
		DryRun:    true,
		Engine:    "say",
	})
	if err == nil {
		t.Fatal("expected error for no chapters")
	}
}

func TestRunNarrate_DefaultOutputDir(t *testing.T) {
	dir := t.TempDir()
	mdPath := filepath.Join(dir, "my-story.md")

	content := `# My Great Story
## Chapter One
Content.
`

	if err := os.WriteFile(mdPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Change to temp dir so default output dir is created there
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	result, err := RunNarrate(NarrateOpts{
		InputFile: mdPath,
		DryRun:    true,
		Engine:    "say",
	})
	if err != nil {
		t.Fatalf("RunNarrate dry run failed: %v", err)
	}

	if result.StoryTitle != "My Great Story" {
		t.Errorf("title = %q, want %q", result.StoryTitle, "My Great Story")
	}
}
