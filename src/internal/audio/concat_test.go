package audio

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func ffmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

func ffprobeAvailable() bool {
	_, err := exec.LookPath("ffprobe")
	return err == nil
}

func getAudioDurationSec(path string) (float64, error) {
	cmd := exec.Command("ffprobe", "-v", "error",
		"-show_entries", "format=duration",
		"-of", "csv=p=0", path)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	return strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
}

func TestGenerateSilence(t *testing.T) {
	if !ffmpegAvailable() || !ffprobeAvailable() {
		t.Skip("ffmpeg/ffprobe not available")
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "silence.mp3")

	err := GenerateSilence(outPath, 2.0, 24000, 1)
	if err != nil {
		t.Fatalf("GenerateSilence failed: %v", err)
	}

	if _, err := os.Stat(outPath); os.IsNotExist(err) {
		t.Fatal("output file not created")
	}

	dur, err := getAudioDurationSec(outPath)
	if err != nil {
		t.Fatalf("failed to get duration: %v", err)
	}

	// Allow some tolerance
	if dur < 1.8 || dur > 2.5 {
		t.Errorf("duration = %.2f, want ~2.0", dur)
	}
}

func TestConcatAudioFiles(t *testing.T) {
	if !ffmpegAvailable() || !ffprobeAvailable() {
		t.Skip("ffmpeg/ffprobe not available")
	}

	dir := t.TempDir()

	// Generate two silence files to concatenate
	file1 := filepath.Join(dir, "part1.mp3")
	file2 := filepath.Join(dir, "part2.mp3")
	outPath := filepath.Join(dir, "combined.mp3")

	if err := GenerateSilence(file1, 1.0, 24000, 1); err != nil {
		t.Fatalf("GenerateSilence(1): %v", err)
	}
	if err := GenerateSilence(file2, 1.5, 24000, 1); err != nil {
		t.Fatalf("GenerateSilence(2): %v", err)
	}

	if err := ConcatAudioFiles([]string{file1, file2}, outPath); err != nil {
		t.Fatalf("ConcatAudioFiles: %v", err)
	}

	if _, err := os.Stat(outPath); os.IsNotExist(err) {
		t.Fatal("output file not created")
	}

	dur, err := getAudioDurationSec(outPath)
	if err != nil {
		t.Fatalf("failed to get duration: %v", err)
	}

	// Combined should be ~2.5 seconds
	if dur < 2.0 || dur > 3.5 {
		t.Errorf("combined duration = %.2f, want ~2.5", dur)
	}
}

func TestConcatAudioFiles_SingleFile(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("ffmpeg not available")
	}

	dir := t.TempDir()
	file1 := filepath.Join(dir, "only.mp3")
	outPath := filepath.Join(dir, "output.mp3")

	if err := GenerateSilence(file1, 1.0, 24000, 1); err != nil {
		t.Fatalf("GenerateSilence: %v", err)
	}

	if err := ConcatAudioFiles([]string{file1}, outPath); err != nil {
		t.Fatalf("ConcatAudioFiles: %v", err)
	}

	if _, err := os.Stat(outPath); os.IsNotExist(err) {
		t.Fatal("output file not created")
	}
}

func TestConcatAudioFiles_Empty(t *testing.T) {
	err := ConcatAudioFiles(nil, "/tmp/out.mp3")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestGetAudioInfo(t *testing.T) {
	if !ffmpegAvailable() || !ffprobeAvailable() {
		t.Skip("ffmpeg/ffprobe not available")
	}

	dir := t.TempDir()
	file := filepath.Join(dir, "test.mp3")

	if err := GenerateSilence(file, 0.5, 44100, 2); err != nil {
		t.Fatalf("GenerateSilence: %v", err)
	}

	sr, ch, err := GetAudioInfo(file)
	if err != nil {
		t.Fatalf("GetAudioInfo: %v", err)
	}

	if sr != 44100 {
		t.Errorf("sample rate = %d, want 44100", sr)
	}
	if ch != 2 {
		t.Errorf("channels = %d, want 2", ch)
	}
}
