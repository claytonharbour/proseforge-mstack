package validation

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const (
	// DefaultMaxSamples is the maximum number of frame samples to keep
	DefaultMaxSamples = 50
	// MetricsFileName is the name of the metrics file
	MetricsFileName = "frame-samples.json"
	// MetricsDir is the directory for mstack config files
	MetricsDir = ".mstack"
)

// FrameSamples tracks frame parameter usage for tuning defaults
type FrameSamples struct {
	Samples    []int `json:"samples"`
	MaxSamples int   `json:"max_samples"`
}

// GetMetricsPath returns the path to the metrics file
func GetMetricsPath() string {
	// Try to use project root first, fall back to home directory
	cwd, err := os.Getwd()
	if err != nil {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, MetricsDir, MetricsFileName)
	}
	return filepath.Join(cwd, MetricsDir, MetricsFileName)
}

// LoadFrameSamples loads existing frame samples from disk
func LoadFrameSamples() (*FrameSamples, error) {
	path := GetMetricsPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty samples with default max
			return &FrameSamples{
				Samples:    []int{},
				MaxSamples: DefaultMaxSamples,
			}, nil
		}
		return nil, err
	}

	var samples FrameSamples
	if err := json.Unmarshal(data, &samples); err != nil {
		return nil, err
	}

	// Ensure max_samples is set
	if samples.MaxSamples == 0 {
		samples.MaxSamples = DefaultMaxSamples
	}

	return &samples, nil
}

// SaveFrameSamples saves frame samples to disk
func SaveFrameSamples(samples *FrameSamples) error {
	path := GetMetricsPath()

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(samples, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, append(data, '\n'), 0644)
}

// RecordFrameSample adds a frame count sample with rolling window
func RecordFrameSample(frames int) error {
	samples, err := LoadFrameSamples()
	if err != nil {
		return err
	}

	// Append new sample
	samples.Samples = append(samples.Samples, frames)

	// Rolling window: drop oldest if over max
	if len(samples.Samples) > samples.MaxSamples {
		// FIFO: remove from front
		samples.Samples = samples.Samples[len(samples.Samples)-samples.MaxSamples:]
	}

	return SaveFrameSamples(samples)
}

// GetAverageFrames returns the average frame count from samples
func GetAverageFrames() (int, error) {
	samples, err := LoadFrameSamples()
	if err != nil {
		return 3, err // Default to 3 on error
	}

	if len(samples.Samples) == 0 {
		return 3, nil // Default to 3 if no samples
	}

	sum := 0
	for _, s := range samples.Samples {
		sum += s
	}

	return sum / len(samples.Samples), nil
}

// GetSampleCount returns the current number of samples
func GetSampleCount() (int, error) {
	samples, err := LoadFrameSamples()
	if err != nil {
		return 0, err
	}
	return len(samples.Samples), nil
}
