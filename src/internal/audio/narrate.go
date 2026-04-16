package audio

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/claytonharbour/proseforge-mstack/src/internal/types"
	"github.com/claytonharbour/proseforge-mstack/src/internal/video"
)

// NarrateOpts configures the long-form narration pipeline.
type NarrateOpts struct {
	InputFile     string
	OutputDir     string        // default: ./<story-slug>/
	Engine        string        // say, gemini, cloudtts, vertex, auto
	Voice         string
	Model         string
	WordsPerMinute int
	ChapterGapSec float64       // silence between chapters (default: 2.0)
	KeepChunks    bool          // keep intermediate chunk audio
	ChapterTitles bool          // speak chapter titles (default: true)
	Timeout       time.Duration // TTS API timeout (default: 30m)
	Verbose       bool
	Project       string
	DryRun        bool          // parse + estimate only
}

// ChapterOutput contains metadata about a generated chapter audio file.
type ChapterOutput struct {
	Index    int    `json:"index"`
	Title    string `json:"title"`
	File     string `json:"file"`
	Words    int    `json:"words"`
	Chunks   int    `json:"chunks"`
}

// NarrateResult contains the result of a narration run.
type NarrateResult struct {
	StoryTitle    string          `json:"story_title"`
	ChapterCount  int             `json:"chapter_count"`
	TotalWords    int             `json:"total_words"`
	TotalChunks   int             `json:"total_chunks"`
	ChapterFiles  []ChapterOutput `json:"chapter_files"`
	AudiobookFile string          `json:"audiobook_file,omitempty"`
}

// RunNarrate executes the full long-form narration pipeline.
func RunNarrate(opts NarrateOpts) (*NarrateResult, error) {
	// Apply defaults
	if opts.ChapterGapSec <= 0 {
		opts.ChapterGapSec = 2.0
	}
	if opts.Timeout <= 0 {
		opts.Timeout = 30 * time.Minute
	}
	if opts.Project == "" {
		opts.Project = "proseforge"
	}

	// Resolve engine
	engine, err := video.ResolveEngine(opts.Engine, opts.Project)
	if err != nil {
		return nil, err
	}
	opts.Engine = engine

	// Step 1: Parse markdown
	parsed, err := ParseMarkdown(opts.InputFile)
	if err != nil {
		return nil, fmt.Errorf("failed to parse markdown: %w", err)
	}

	if len(parsed.Chapters) == 0 {
		return nil, fmt.Errorf("no chapters found in %s (expected ## headings)", opts.InputFile)
	}

	// Derive output directory from story title if not specified
	if opts.OutputDir == "" {
		slug := Slug(parsed.Meta.Title)
		if slug == "" {
			slug = strings.TrimSuffix(filepath.Base(opts.InputFile), filepath.Ext(opts.InputFile))
			slug = Slug(slug)
		}
		opts.OutputDir = slug
	}

	// Calculate totals
	totalWords := 0
	chunkOpts := ChunkOpts{MaxBytes: DefaultMaxChunkBytes, MaxWords: DefaultMaxChunkWords}
	totalChunks := 0
	for _, ch := range parsed.Chapters {
		totalWords += ch.WordCount
		chunks := ChunkChapter(ch.Body, 1, chunkOpts)
		totalChunks += len(chunks)
		if opts.ChapterTitles {
			totalChunks++ // title segment
		}
	}

	// Print summary
	fmt.Printf("Story: %s\n", parsed.Meta.Title)
	if parsed.Meta.Author != "" {
		fmt.Printf("Author: %s\n", parsed.Meta.Author)
	}
	fmt.Printf("Chapters: %d\n", len(parsed.Chapters))
	fmt.Printf("Total words: %d\n", totalWords)
	fmt.Printf("Total chunks: %d\n", totalChunks)
	fmt.Printf("Engine: %s\n", opts.Engine)
	fmt.Println()

	for _, ch := range parsed.Chapters {
		chunks := ChunkChapter(ch.Body, 1, chunkOpts)
		fmt.Printf("  %d. %s (%d words, %d chunks)\n", ch.Index, ch.Title, ch.WordCount, len(chunks))
	}
	fmt.Println()

	result := &NarrateResult{
		StoryTitle:   parsed.Meta.Title,
		ChapterCount: len(parsed.Chapters),
		TotalWords:   totalWords,
		TotalChunks:  totalChunks,
	}

	// Dry run: return early with estimates
	if opts.DryRun {
		fmt.Println("Dry run complete.")
		return result, nil
	}

	// Step 2: Create output directories
	chunksDir := filepath.Join(opts.OutputDir, "chunks")
	chaptersDir := filepath.Join(opts.OutputDir, "chapters")
	if err := os.MkdirAll(chunksDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create chunks dir: %w", err)
	}
	if err := os.MkdirAll(chaptersDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create chapters dir: %w", err)
	}

	videoSvc := video.NewService()
	segmentIndex := 1
	var chapterAudioFiles []string

	// Step 3: Generate TTS for each chapter
	for _, ch := range parsed.Chapters {
		chapterSlug := Slug(ch.Title)
		chapterChunkDir := filepath.Join(chunksDir, fmt.Sprintf("chapter_%02d", ch.Index))
		if err := os.MkdirAll(chapterChunkDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create chapter chunk dir: %w", err)
		}

		fmt.Printf("Chapter %d: %s\n", ch.Index, ch.Title)

		// Build segments for this chapter
		var segments []types.Segment

		// Optionally add chapter title as first segment
		if opts.ChapterTitles {
			titleText := fmt.Sprintf("Chapter %d. %s", ch.Index, ch.Title)
			segments = append(segments, types.Segment{
				Index: segmentIndex,
				Text:  titleText,
			})
			segmentIndex++
		}

		// Chunk the chapter body
		bodySegments := ChunkChapter(ch.Body, segmentIndex, chunkOpts)
		segments = append(segments, bodySegments...)
		segmentIndex += len(bodySegments)

		// Generate TTS
		err := videoSvc.GenerateTTS(video.TTSOpts{
			Segments:       segments,
			OutputDir:      chapterChunkDir,
			Engine:         opts.Engine,
			Voice:          opts.Voice,
			Model:          opts.Model,
			WordsPerMinute: opts.WordsPerMinute,
			Timeout:        opts.Timeout,
			Verbose:        opts.Verbose,
			Project:        opts.Project,
		})
		if err != nil {
			return nil, fmt.Errorf("TTS failed for chapter %d (%s): %w", ch.Index, ch.Title, err)
		}

		// Collect generated audio files in order
		var chunkFiles []string
		for _, seg := range segments {
			// Try common extensions
			var found string
			for _, ext := range []string{".m4a", ".mp3", ".wav"} {
				candidate := filepath.Join(chapterChunkDir, fmt.Sprintf("segment_%03d%s", seg.Index, ext))
				if _, err := os.Stat(candidate); err == nil {
					found = candidate
					break
				}
			}
			if found == "" {
				return nil, fmt.Errorf("audio file not found for segment %d in %s", seg.Index, chapterChunkDir)
			}
			chunkFiles = append(chunkFiles, found)
		}

		// Concatenate chunks into chapter file
		chapterExt := filepath.Ext(chunkFiles[0])
		chapterFile := filepath.Join(chaptersDir, fmt.Sprintf("%02d_%s%s", ch.Index, chapterSlug, chapterExt))
		fmt.Printf("  Concatenating %d chunks -> %s\n", len(chunkFiles), filepath.Base(chapterFile))

		if err := ConcatAudioFiles(chunkFiles, chapterFile); err != nil {
			return nil, fmt.Errorf("failed to concatenate chapter %d: %w", ch.Index, err)
		}

		chapterAudioFiles = append(chapterAudioFiles, chapterFile)
		result.ChapterFiles = append(result.ChapterFiles, ChapterOutput{
			Index:  ch.Index,
			Title:  ch.Title,
			File:   chapterFile,
			Words:  ch.WordCount,
			Chunks: len(segments),
		})

		fmt.Println()
	}

	// Step 4: Generate combined audiobook
	storySlug := Slug(parsed.Meta.Title)
	if storySlug == "" {
		storySlug = strings.TrimSuffix(filepath.Base(opts.InputFile), filepath.Ext(opts.InputFile))
		storySlug = Slug(storySlug)
	}

	audiobookExt := filepath.Ext(chapterAudioFiles[0])
	audiobookFile := filepath.Join(opts.OutputDir, storySlug+audiobookExt)

	if len(chapterAudioFiles) > 1 {
		// Get audio info from first chapter file for silence generation
		sampleRate, channels, err := GetAudioInfo(chapterAudioFiles[0])
		if err != nil {
			// Use defaults if probe fails
			sampleRate = 24000
			channels = 1
		}

		// Generate silence file for chapter gaps
		silenceFile := filepath.Join(opts.OutputDir, "silence"+audiobookExt)
		if err := GenerateSilence(silenceFile, opts.ChapterGapSec, sampleRate, channels); err != nil {
			return nil, fmt.Errorf("failed to generate silence: %w", err)
		}
		defer os.Remove(silenceFile)

		// Interleave chapter files with silence
		var concatList []string
		for i, f := range chapterAudioFiles {
			concatList = append(concatList, f)
			if i < len(chapterAudioFiles)-1 {
				concatList = append(concatList, silenceFile)
			}
		}

		fmt.Printf("Concatenating %d chapters -> %s\n", len(chapterAudioFiles), filepath.Base(audiobookFile))
		if err := ConcatAudioFiles(concatList, audiobookFile); err != nil {
			return nil, fmt.Errorf("failed to concatenate audiobook: %w", err)
		}
	} else {
		// Single chapter: just copy
		if err := ConcatAudioFiles(chapterAudioFiles, audiobookFile); err != nil {
			return nil, fmt.Errorf("failed to copy single chapter to audiobook: %w", err)
		}
	}

	result.AudiobookFile = audiobookFile

	// Step 5: Clean up chunks if not keeping them
	if !opts.KeepChunks {
		fmt.Println("Cleaning up chunks...")
		os.RemoveAll(chunksDir)
	}

	// Step 6: Write manifest
	manifestPath := filepath.Join(opts.OutputDir, "narrate.json")
	manifestData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal manifest: %w", err)
	}
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		return nil, fmt.Errorf("failed to write manifest: %w", err)
	}

	fmt.Printf("\nAudiobook: %s\n", audiobookFile)
	fmt.Printf("Manifest: %s\n", manifestPath)

	return result, nil
}
