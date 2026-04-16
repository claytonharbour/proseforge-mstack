package video

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// DefaultTTSModels lists AI Studio Gemini TTS models in priority order.
// Non-preview names are included as forward-looking fallbacks — they will
// 404 today but auto-activate if Google adds them.
var DefaultTTSModels = []string{
	"gemini-2.5-flash-tts",
	"gemini-2.5-flash-preview-tts",
	"gemini-2.5-pro-tts",
	"gemini-2.5-pro-preview-tts",
}

// DefaultCloudTTSModels lists Cloud TTS models (GA names only).
var DefaultCloudTTSModels = []string{
	"gemini-2.5-flash-tts",
	"gemini-2.5-pro-tts",
}

// DefaultVertexModels lists Vertex AI TTS models (GA names only).
var DefaultVertexModels = []string{
	"gemini-2.5-flash-tts",
	"gemini-2.5-pro-tts",
}

const ttsModelsCacheMaxAge = 24 * time.Hour

// QuotaType classifies a Gemini rate-limit response.
type QuotaType int

const (
	QuotaUnknown   QuotaType = iota
	QuotaPerMinute           // transient — wait retryDelay, retry same model
	QuotaPerDay              // exhausted — fail over immediately
)

// GeminiRetryInfo carries parsed retry/quota information from a Gemini 429 response body.
type GeminiRetryInfo struct {
	RetryDelay time.Duration
	Quota      QuotaType
}

// ModelPool tracks per-model cooldown state for rate-limit failover.
type ModelPool struct {
	mu          sync.Mutex
	models      []string
	availableAt map[string]time.Time
}

// NewModelPool creates a pool with the given models. If models is empty,
// DefaultTTSModels is used.
func NewModelPool(models []string) *ModelPool {
	if len(models) == 0 {
		models = DefaultTTSModels
	}
	avail := make(map[string]time.Time, len(models))
	for _, m := range models {
		avail[m] = time.Time{} // zero time = immediately available
	}
	return &ModelPool{
		models:      models,
		availableAt: avail,
	}
}

// PickModel returns the first model whose cooldown has expired, respecting
// priority order. Returns ("", false) when all models are on cooldown.
func (p *ModelPool) PickModel() (string, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	now := time.Now()
	for _, m := range p.models {
		if !p.availableAt[m].After(now) {
			return m, true
		}
	}
	return "", false
}

// EarliestAvailable returns the soonest time any model becomes available.
func (p *ModelPool) EarliestAvailable() time.Time {
	p.mu.Lock()
	defer p.mu.Unlock()

	var earliest time.Time
	for _, m := range p.models {
		t := p.availableAt[m]
		if earliest.IsZero() || t.Before(earliest) {
			earliest = t
		}
	}
	return earliest
}

// MarkRateLimited sets a cooldown for the given model. If cooldown > 0,
// that value is used; otherwise a default of 60s is applied. Jitter of up to
// 5s is added.
func (p *ModelPool) MarkRateLimited(model string, cooldown time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if cooldown <= 0 {
		cooldown = 60 * time.Second
	}
	cooldown = addJitter(cooldown, 5*time.Second)
	p.availableAt[model] = time.Now().Add(cooldown)
}

// MarkAvailable clears the cooldown for a model after a successful request.
func (p *ModelPool) MarkAvailable(model string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.availableAt[model] = time.Time{}
}

// parseRetryAfter extracts the integer-seconds value from a Retry-After
// header. Returns 0 if the header is absent or unparseable.
func parseRetryAfter(resp *http.Response) int {
	if resp == nil {
		return 0
	}
	val := resp.Header.Get("Retry-After")
	if val == "" {
		return 0
	}
	sec, err := strconv.Atoi(val)
	if err != nil || sec < 0 {
		return 0
	}
	return sec
}

// addJitter adds a random duration in [0, jitterMax) to base.
func addJitter(base, jitterMax time.Duration) time.Duration {
	if jitterMax <= 0 {
		return base
	}
	return base + time.Duration(rand.Int63n(int64(jitterMax)))
}

// geminiErrorResponse models the relevant parts of a Gemini API error body.
type geminiErrorResponse struct {
	Error struct {
		Details []json.RawMessage `json:"details"`
	} `json:"error"`
}

// geminiRetryDetail models a RetryInfo detail entry.
type geminiRetryDetail struct {
	Type       string `json:"@type"`
	RetryDelay string `json:"retryDelay"`
}

// geminiQuotaDetail models a QuotaFailure detail entry.
type geminiQuotaDetail struct {
	Type       string `json:"@type"`
	Violations []struct {
		Subject     string `json:"subject"`
		Description string `json:"description"`
		QuotaMetric string `json:"quotaMetric"`
		QuotaID     string `json:"quotaId"`
	} `json:"violations"`
}

// parseGeminiRetryInfo extracts retry delay and quota type from a Gemini 429
// response body. Gemini puts this info in error.details[] rather than HTTP
// headers.
func parseGeminiRetryInfo(body []byte) GeminiRetryInfo {
	var info GeminiRetryInfo

	var errResp geminiErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		return info
	}

	for _, raw := range errResp.Error.Details {
		// Try RetryInfo
		var retry geminiRetryDetail
		if json.Unmarshal(raw, &retry) == nil && strings.HasSuffix(retry.Type, "RetryInfo") {
			if d := parseDurationString(retry.RetryDelay); d > 0 {
				info.RetryDelay = d
			}
		}

		// Try QuotaFailure
		var quota geminiQuotaDetail
		if json.Unmarshal(raw, &quota) == nil && strings.HasSuffix(quota.Type, "QuotaFailure") {
			for _, v := range quota.Violations {
				combined := strings.ToLower(v.Subject + " " + v.Description + " " + v.QuotaMetric + " " + v.QuotaID)
				if strings.Contains(combined, "perday") || strings.Contains(combined, "per_day") || strings.Contains(combined, "daily") {
					info.Quota = QuotaPerDay
					break
				}
				if strings.Contains(combined, "perminute") || strings.Contains(combined, "per_minute") {
					info.Quota = QuotaPerMinute
				}
			}
		}
	}

	// Heuristic: if we got a retryDelay but no explicit quota type, infer from delay length
	if info.RetryDelay > 0 && info.Quota == QuotaUnknown {
		if info.RetryDelay < 5*time.Minute {
			info.Quota = QuotaPerMinute
		}
	}

	return info
}

// parseDurationString parses a Go-style duration string (e.g. "34s",
// "15.002899939s"). Returns 0 on error or negative values.
func parseDurationString(s string) time.Duration {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	d, err := time.ParseDuration(s)
	if err != nil || d < 0 {
		return 0
	}
	return d
}

// otherModels returns DefaultTTSModels minus the excluded model.
func otherModels(exclude string) []string {
	var out []string
	for _, m := range DefaultTTSModels {
		if m != exclude {
			out = append(out, m)
		}
	}
	return out
}

// ttsModelsCache is the JSON structure written to ~/.mstack/tts_models.json.
type ttsModelsCache struct {
	Models     []string  `json:"models"`
	Discovered time.Time `json:"discovered"`
}

// ttsModelsCachePath returns the path to the TTS models cache file.
func ttsModelsCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".mstack", "tts_models.json")
}

// geminiModelListResponse models the Gemini /models endpoint response.
type geminiModelListResponse struct {
	Models []geminiModelInfo `json:"models"`
}

type geminiModelInfo struct {
	Name                       string   `json:"name"`
	SupportedGenerationMethods []string `json:"supportedGenerationMethods"`
}

// modelDiscoveryHTTPClient is overridable for testing.
var modelDiscoveryHTTPClient *http.Client

func getDiscoveryHTTPClient() *http.Client {
	if modelDiscoveryHTTPClient != nil {
		return modelDiscoveryHTTPClient
	}
	return &http.Client{Timeout: 15 * time.Second}
}

// modelDiscoveryBaseURL is overridable for testing.
var modelDiscoveryBaseURL = "https://generativelanguage.googleapis.com"

// DiscoverTTSModels queries the Gemini API for available TTS models.
// Returns model IDs sorted by priority (flash before pro, non-preview before preview).
func DiscoverTTSModels(apiKey string) ([]string, error) {
	url := fmt.Sprintf("%s/v1beta/models?key=%s", modelDiscoveryBaseURL, apiKey)
	client := getDiscoveryHTTPClient()

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("model discovery request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read model list response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("model discovery returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var listResp geminiModelListResponse
	if err := json.Unmarshal(body, &listResp); err != nil {
		return nil, fmt.Errorf("failed to parse model list: %w", err)
	}

	var ttsModels []string
	for _, m := range listResp.Models {
		// Model name comes as "models/gemini-2.5-flash-preview-tts", strip prefix
		id := m.Name
		if strings.HasPrefix(id, "models/") {
			id = strings.TrimPrefix(id, "models/")
		}

		// Filter: must contain "tts" and support generateContent
		if !strings.Contains(strings.ToLower(id), "tts") {
			continue
		}
		supportsGenerate := false
		for _, method := range m.SupportedGenerationMethods {
			if method == "generateContent" {
				supportsGenerate = true
				break
			}
		}
		if !supportsGenerate {
			continue
		}
		ttsModels = append(ttsModels, id)
	}

	if len(ttsModels) == 0 {
		return nil, fmt.Errorf("no TTS models found in API response")
	}

	// Sort: flash before pro, non-preview before preview
	sort.SliceStable(ttsModels, func(i, j int) bool {
		return ttsModelPriority(ttsModels[i]) < ttsModelPriority(ttsModels[j])
	})

	return ttsModels, nil
}

// ttsModelPriority returns a sort key: lower = higher priority.
// flash < pro, non-preview < preview.
func ttsModelPriority(model string) int {
	m := strings.ToLower(model)
	score := 0
	if strings.Contains(m, "pro") {
		score += 10
	}
	if strings.Contains(m, "preview") {
		score += 1
	}
	return score
}

// ResolveTTSModels returns the list of TTS models to use, checking the
// file cache first, then falling back to API discovery, then to DefaultTTSModels.
func ResolveTTSModels(apiKey string) []string {
	cachePath := ttsModelsCachePath()

	// Try reading cache
	if cachePath != "" {
		if cached, err := readTTSModelsCache(cachePath); err == nil {
			if time.Since(cached.Discovered) < ttsModelsCacheMaxAge && len(cached.Models) > 0 {
				return cached.Models
			}
		}
	}

	// Cache miss or stale — discover from API
	models, err := DiscoverTTSModels(apiKey)
	if err != nil {
		// Discovery failed — try stale cache
		if cachePath != "" {
			if cached, cacheErr := readTTSModelsCache(cachePath); cacheErr == nil && len(cached.Models) > 0 {
				return cached.Models
			}
		}
		return DefaultTTSModels
	}

	// Write cache
	if cachePath != "" {
		_ = writeTTSModelsCache(cachePath, models)
	}

	return models
}

// InvalidateTTSModelsCache removes the cache file so the next call to
// ResolveTTSModels re-discovers models from the API.
func InvalidateTTSModelsCache() {
	if p := ttsModelsCachePath(); p != "" {
		os.Remove(p)
	}
}

func readTTSModelsCache(path string) (*ttsModelsCache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cache ttsModelsCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	return &cache, nil
}

func writeTTSModelsCache(path string, models []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	cache := ttsModelsCache{
		Models:     models,
		Discovered: time.Now(),
	}
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// QuotaZero is a special QuotaType indicating a model reports limit: 0.
const QuotaZero QuotaType = 3

// CheckModelQuota makes a minimal generateContent request to test whether a
// model has any usable quota. Returns true if the model responds successfully.
// When verbose is true, the raw HTTP response body is logged to stderr.
func CheckModelQuota(apiKey, model string, verbose bool) (bool, error) {
	url := fmt.Sprintf("%s/v1beta/models/%s:generateContent?key=%s", modelDiscoveryBaseURL, model, apiKey)

	reqBody := GeminiTTSRequest{
		Contents: []GeminiContent{
			{Parts: []GeminiPart{{Text: "test"}}},
		},
		GenerationConfig: GeminiConfig{
			ResponseModalities: []string{"AUDIO"},
			SpeechConfig: GeminiSpeechConfig{
				VoiceConfig: GeminiVoiceConfig{
					PrebuiltVoiceConfig: GeminiPrebuiltVoiceConfig{
						VoiceName: "Kore",
					},
				},
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return false, fmt.Errorf("failed to marshal quota check request: %w", err)
	}

	client := getDiscoveryHTTPClient()
	req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return false, fmt.Errorf("failed to create quota check request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("quota check request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read quota check response: %w", err)
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "[verbose] Quota check %s: HTTP %d\n[verbose] %s\n", model, resp.StatusCode, string(body))
	}

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}

	if resp.StatusCode == 429 {
		// Surface any help URLs from the response
		if urls := extractHelpURLs(body); len(urls) > 0 {
			for _, u := range urls {
				fmt.Fprintf(os.Stderr, "    Quota info: %s\n", u)
			}
		}

		// Check for zero-limit pattern: limit: 0 means model has no quota at all
		if hasZeroLimit(body) {
			return false, nil
		}
		// Transient rate limit (has quota, just exhausted temporarily)
		info := parseGeminiRetryInfo(body)
		if info.Quota == QuotaPerDay || info.Quota == QuotaPerMinute {
			return false, nil
		}
		return false, nil
	}

	return false, fmt.Errorf("quota check returned HTTP %d: %s", resp.StatusCode, string(body))
}

// geminiHelpDetail models a Help detail entry with links.
type geminiHelpDetail struct {
	Type  string `json:"@type"`
	Links []struct {
		Description string `json:"description"`
		URL         string `json:"url"`
	} `json:"links"`
}

// extractHelpURLs pulls any URLs from Help detail entries in a Gemini error response.
func extractHelpURLs(body []byte) []string {
	var errResp geminiErrorResponse
	if err := json.Unmarshal(body, &errResp); err != nil {
		return nil
	}
	var urls []string
	for _, raw := range errResp.Error.Details {
		var help geminiHelpDetail
		if json.Unmarshal(raw, &help) == nil && strings.HasSuffix(help.Type, "Help") {
			for _, link := range help.Links {
				if link.URL != "" {
					urls = append(urls, link.URL)
				}
			}
		}
	}
	return urls
}

// hasZeroLimit checks if a 429 response body contains a "limit": 0 field,
// indicating the model has no allocated quota at all.
func hasZeroLimit(body []byte) bool {
	// Look for "limit": 0 or "limit":0 pattern
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return false
	}
	return searchForZeroLimit(raw)
}

func searchForZeroLimit(v interface{}) bool {
	switch val := v.(type) {
	case map[string]interface{}:
		for k, child := range val {
			if strings.EqualFold(k, "limit") {
				if num, ok := child.(float64); ok && num == 0 {
					return true
				}
			}
			if searchForZeroLimit(child) {
				return true
			}
		}
	case []interface{}:
		for _, child := range val {
			if searchForZeroLimit(child) {
				return true
			}
		}
	}
	return false
}
