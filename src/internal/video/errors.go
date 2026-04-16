package video

import "fmt"

// TTSRateLimitError indicates TTS API rate limiting or transient server error (HTTP 429/500/503).
type TTSRateLimitError struct {
	StatusCode       int
	Model            string
	ModelsTriedCount int
}

func (e *TTSRateLimitError) Error() string {
	return fmt.Sprintf("TTS rate limited (HTTP %d) — exhausted all %d model(s), last tried: %s", e.StatusCode, e.ModelsTriedCount, e.Model)
}

// TTSAuthError indicates missing or invalid TTS credentials.
type TTSAuthError struct {
	Detail string
}

func (e *TTSAuthError) Error() string {
	return fmt.Sprintf("TTS auth failed: %s", e.Detail)
}

// FFmpegError wraps an ffmpeg execution failure.
type FFmpegError struct {
	Cause error
}

func (e *FFmpegError) Error() string {
	return fmt.Sprintf("ffmpeg failed: %v", e.Cause)
}

func (e *FFmpegError) Unwrap() error {
	return e.Cause
}
