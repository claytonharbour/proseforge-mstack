package video

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewModelPoolDefaults(t *testing.T) {
	pool := NewModelPool(nil)
	if len(pool.models) != len(DefaultTTSModels) {
		t.Fatalf("expected %d models, got %d", len(DefaultTTSModels), len(pool.models))
	}
	for i, m := range DefaultTTSModels {
		if pool.models[i] != m {
			t.Errorf("model[%d]: expected %s, got %s", i, m, pool.models[i])
		}
	}
}

func TestNewModelPoolCustom(t *testing.T) {
	custom := []string{"model-a", "model-b", "model-c"}
	pool := NewModelPool(custom)
	if len(pool.models) != 3 {
		t.Fatalf("expected 3 models, got %d", len(pool.models))
	}
	for i, m := range custom {
		if pool.models[i] != m {
			t.Errorf("model[%d]: expected %s, got %s", i, m, pool.models[i])
		}
	}
}

func TestPickModelPriorityOrder(t *testing.T) {
	pool := NewModelPool([]string{"first", "second"})
	m, ok := pool.PickModel()
	if !ok || m != "first" {
		t.Errorf("expected (first, true), got (%s, %v)", m, ok)
	}
}

func TestPickModelSkipsCooledDown(t *testing.T) {
	pool := NewModelPool([]string{"first", "second"})
	// Put first on cooldown
	pool.availableAt["first"] = time.Now().Add(10 * time.Minute)

	m, ok := pool.PickModel()
	if !ok || m != "second" {
		t.Errorf("expected (second, true), got (%s, %v)", m, ok)
	}
}

func TestPickModelAllCooledDown(t *testing.T) {
	pool := NewModelPool([]string{"first", "second"})
	pool.availableAt["first"] = time.Now().Add(10 * time.Minute)
	pool.availableAt["second"] = time.Now().Add(10 * time.Minute)

	_, ok := pool.PickModel()
	if ok {
		t.Error("expected ok=false when all models on cooldown")
	}
}

func TestPickModelExpiredCooldown(t *testing.T) {
	pool := NewModelPool([]string{"first", "second"})
	// Set cooldown in the past — should be available
	pool.availableAt["first"] = time.Now().Add(-1 * time.Second)

	m, ok := pool.PickModel()
	if !ok || m != "first" {
		t.Errorf("expected (first, true), got (%s, %v)", m, ok)
	}
}

func TestMarkRateLimitedWithRetryAfter(t *testing.T) {
	pool := NewModelPool([]string{"model-a"})
	pool.MarkRateLimited("model-a", 30*time.Second)

	// Should now be on cooldown (at least 30s from now)
	avail := pool.availableAt["model-a"]
	minExpected := time.Now().Add(30 * time.Second)
	maxExpected := time.Now().Add(35 * time.Second) // 30s + up to 5s jitter
	if avail.Before(minExpected) || avail.After(maxExpected) {
		t.Errorf("cooldown %v not in expected range [%v, %v]", avail, minExpected, maxExpected)
	}
}

func TestMarkRateLimitedDefault(t *testing.T) {
	pool := NewModelPool([]string{"model-a"})
	pool.MarkRateLimited("model-a", 0)

	avail := pool.availableAt["model-a"]
	minExpected := time.Now().Add(60 * time.Second)
	maxExpected := time.Now().Add(65 * time.Second) // 60s + up to 5s jitter
	if avail.Before(minExpected) || avail.After(maxExpected) {
		t.Errorf("cooldown %v not in expected range [%v, %v]", avail, minExpected, maxExpected)
	}
}

func TestMarkAvailable(t *testing.T) {
	pool := NewModelPool([]string{"model-a"})
	pool.availableAt["model-a"] = time.Now().Add(10 * time.Minute)
	pool.MarkAvailable("model-a")

	if !pool.availableAt["model-a"].IsZero() {
		t.Error("expected availableAt to be zero after MarkAvailable")
	}

	m, ok := pool.PickModel()
	if !ok || m != "model-a" {
		t.Errorf("expected (model-a, true) after MarkAvailable, got (%s, %v)", m, ok)
	}
}

func TestEarliestAvailable(t *testing.T) {
	pool := NewModelPool([]string{"first", "second"})
	now := time.Now()
	t1 := now.Add(5 * time.Minute)
	t2 := now.Add(10 * time.Minute)
	pool.availableAt["first"] = t2
	pool.availableAt["second"] = t1

	earliest := pool.EarliestAvailable()
	if !earliest.Equal(t1) {
		t.Errorf("expected earliest=%v, got %v", t1, earliest)
	}
}

func TestEarliestAvailableAllZero(t *testing.T) {
	pool := NewModelPool([]string{"first", "second"})
	earliest := pool.EarliestAvailable()
	if !earliest.IsZero() {
		t.Errorf("expected zero time when all available, got %v", earliest)
	}
}

func TestParseRetryAfterPresent(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Retry-After", "42")
	if got := parseRetryAfter(resp); got != 42 {
		t.Errorf("expected 42, got %d", got)
	}
}

func TestParseRetryAfterMissing(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	if got := parseRetryAfter(resp); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestParseRetryAfterInvalid(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Retry-After", "not-a-number")
	if got := parseRetryAfter(resp); got != 0 {
		t.Errorf("expected 0, got %d", got)
	}
}

func TestParseRetryAfterNegative(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Retry-After", "-5")
	if got := parseRetryAfter(resp); got != 0 {
		t.Errorf("expected 0 for negative, got %d", got)
	}
}

func TestParseRetryAfterNilResponse(t *testing.T) {
	if got := parseRetryAfter(nil); got != 0 {
		t.Errorf("expected 0 for nil response, got %d", got)
	}
}

func TestAddJitter(t *testing.T) {
	base := 10 * time.Second
	jitterMax := 5 * time.Second
	for i := 0; i < 100; i++ {
		result := addJitter(base, jitterMax)
		if result < base || result >= base+jitterMax {
			t.Errorf("addJitter(%v, %v) = %v, out of range [%v, %v)", base, jitterMax, result, base, base+jitterMax)
		}
	}
}

func TestAddJitterZeroMax(t *testing.T) {
	base := 10 * time.Second
	result := addJitter(base, 0)
	if result != base {
		t.Errorf("expected %v with zero jitterMax, got %v", base, result)
	}
}

func TestAddJitterNegativeMax(t *testing.T) {
	base := 10 * time.Second
	result := addJitter(base, -1*time.Second)
	if result != base {
		t.Errorf("expected %v with negative jitterMax, got %v", base, result)
	}
}

func TestOtherModels(t *testing.T) {
	others := otherModels("gemini-2.5-flash-preview-tts")
	if len(others) != len(DefaultTTSModels)-1 {
		t.Fatalf("expected %d other models, got %d", len(DefaultTTSModels)-1, len(others))
	}
	for _, o := range others {
		if o == "gemini-2.5-flash-preview-tts" {
			t.Error("should not contain excluded model")
		}
	}
}

func TestOtherModelsUnknown(t *testing.T) {
	others := otherModels("unknown-model")
	if len(others) != len(DefaultTTSModels) {
		t.Errorf("expected %d models when excluding unknown, got %d", len(DefaultTTSModels), len(others))
	}
}

func TestOtherModelsExcludeAll(t *testing.T) {
	// Excluding one that exists should return the rest
	for _, exclude := range DefaultTTSModels {
		others := otherModels(exclude)
		for _, o := range others {
			if o == exclude {
				t.Errorf("otherModels(%s) should not contain %s", exclude, o)
			}
		}
	}
}

func TestParseDurationString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
	}{
		{"simple seconds", "34s", 34 * time.Second},
		{"fractional seconds", "15.002899939s", 15002899939 * time.Nanosecond},
		{"minutes", "2m", 2 * time.Minute},
		{"mixed", "1m30s", 90 * time.Second},
		{"empty", "", 0},
		{"whitespace only", "  ", 0},
		{"negative", "-5s", 0},
		{"garbage", "not-a-duration", 0},
		{"whitespace padded", " 34s ", 34 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDurationString(tt.input)
			if got != tt.expected {
				t.Errorf("parseDurationString(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseGeminiRetryInfo(t *testing.T) {
	tests := []struct {
		name          string
		body          string
		wantDelay     time.Duration
		wantQuotaType QuotaType
	}{
		{
			name: "standard per-minute with retry delay",
			body: `{
				"error": {
					"details": [
						{"@type": "type.googleapis.com/google.rpc.RetryInfo", "retryDelay": "34s"},
						{"@type": "type.googleapis.com/google.rpc.QuotaFailure", "violations": [{"subject": "PerMinute", "description": "rate limit"}]}
					]
				}
			}`,
			wantDelay:     34 * time.Second,
			wantQuotaType: QuotaPerMinute,
		},
		{
			name: "per-day quota exhausted",
			body: `{
				"error": {
					"details": [
						{"@type": "type.googleapis.com/google.rpc.RetryInfo", "retryDelay": "1s"},
						{"@type": "type.googleapis.com/google.rpc.QuotaFailure", "violations": [{"subject": "PerDay", "description": "daily limit"}]}
					]
				}
			}`,
			wantDelay:     1 * time.Second,
			wantQuotaType: QuotaPerDay,
		},
		{
			name: "per-day with daily keyword in description",
			body: `{
				"error": {
					"details": [
						{"@type": "type.googleapis.com/google.rpc.RetryInfo", "retryDelay": "2s"},
						{"@type": "type.googleapis.com/google.rpc.QuotaFailure", "violations": [{"subject": "quota", "description": "daily quota exceeded"}]}
					]
				}
			}`,
			wantDelay:     2 * time.Second,
			wantQuotaType: QuotaPerDay,
		},
		{
			name: "fractional delay",
			body: `{
				"error": {
					"details": [
						{"@type": "type.googleapis.com/google.rpc.RetryInfo", "retryDelay": "15.002899939s"}
					]
				}
			}`,
			wantDelay:     15002899939 * time.Nanosecond,
			wantQuotaType: QuotaPerMinute, // heuristic: short delay = per-minute
		},
		{
			name:          "empty body",
			body:          "",
			wantDelay:     0,
			wantQuotaType: QuotaUnknown,
		},
		{
			name:          "non-JSON body",
			body:          "this is not json",
			wantDelay:     0,
			wantQuotaType: QuotaUnknown,
		},
		{
			name: "missing details array",
			body: `{
				"error": {"message": "something went wrong"}
			}`,
			wantDelay:     0,
			wantQuotaType: QuotaUnknown,
		},
		{
			name: "RetryInfo only, no QuotaFailure (short delay)",
			body: `{
				"error": {
					"details": [
						{"@type": "type.googleapis.com/google.rpc.RetryInfo", "retryDelay": "10s"}
					]
				}
			}`,
			wantDelay:     10 * time.Second,
			wantQuotaType: QuotaPerMinute, // heuristic: <5min = per-minute
		},
		{
			name: "RetryInfo only with long delay",
			body: `{
				"error": {
					"details": [
						{"@type": "type.googleapis.com/google.rpc.RetryInfo", "retryDelay": "10m0s"}
					]
				}
			}`,
			wantDelay:     10 * time.Minute,
			wantQuotaType: QuotaUnknown, // >=5min, can't infer
		},
		{
			name: "real Gemini quotaMetric/quotaId format",
			body: `{
				"error": {
					"code": 429,
					"message": "You exceeded your current quota",
					"status": "RESOURCE_EXHAUSTED",
					"details": [
						{"@type": "type.googleapis.com/google.rpc.Help", "links": [{"description": "Learn more", "url": "https://ai.google.dev/gemini-api/docs/rate-limits"}]},
						{"@type": "type.googleapis.com/google.rpc.QuotaFailure", "violations": [{"quotaMetric": "generativelanguage.googleapis.com/generate_requests_per_model_per_day", "quotaId": "GenerateRequestsPerDayPerProjectPerModel"}]}
					]
				}
			}`,
			wantDelay:     0,
			wantQuotaType: QuotaPerDay,
		},
		{
			name: "real Gemini per-minute quotaMetric format",
			body: `{
				"error": {
					"code": 429,
					"status": "RESOURCE_EXHAUSTED",
					"details": [
						{"@type": "type.googleapis.com/google.rpc.RetryInfo", "retryDelay": "34s"},
						{"@type": "type.googleapis.com/google.rpc.QuotaFailure", "violations": [{"quotaMetric": "generativelanguage.googleapis.com/generate_requests_per_minute_per_project_per_model", "quotaId": "GenerateRequestsPerMinutePerProjectPerModel"}]}
					]
				}
			}`,
			wantDelay:     34 * time.Second,
			wantQuotaType: QuotaPerMinute,
		},
		{
			name: "per-day takes precedence over per-minute",
			body: `{
				"error": {
					"details": [
						{"@type": "type.googleapis.com/google.rpc.RetryInfo", "retryDelay": "5s"},
						{"@type": "type.googleapis.com/google.rpc.QuotaFailure", "violations": [
							{"subject": "PerMinute", "description": "rate limit"},
							{"subject": "PerDay", "description": "daily limit"}
						]}
					]
				}
			}`,
			wantDelay:     5 * time.Second,
			wantQuotaType: QuotaPerDay,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := parseGeminiRetryInfo([]byte(tt.body))
			if info.RetryDelay != tt.wantDelay {
				t.Errorf("RetryDelay = %v, want %v", info.RetryDelay, tt.wantDelay)
			}
			if info.Quota != tt.wantQuotaType {
				t.Errorf("Quota = %v, want %v", info.Quota, tt.wantQuotaType)
			}
		})
	}
}

// --- Discovery tests ---

func TestDiscoverTTSModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models" {
			http.NotFound(w, r)
			return
		}
		resp := geminiModelListResponse{
			Models: []geminiModelInfo{
				{Name: "models/gemini-2.5-pro-preview-tts", SupportedGenerationMethods: []string{"generateContent"}},
				{Name: "models/gemini-2.5-flash-preview-tts", SupportedGenerationMethods: []string{"generateContent"}},
				{Name: "models/gemini-2.0-flash", SupportedGenerationMethods: []string{"generateContent"}},
				{Name: "models/text-embedding-004", SupportedGenerationMethods: []string{"embedContent"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	old := modelDiscoveryBaseURL
	modelDiscoveryBaseURL = srv.URL
	defer func() { modelDiscoveryBaseURL = old }()

	models, err := DiscoverTTSModels("test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 TTS models, got %d: %v", len(models), models)
	}
	// flash should come before pro (priority sort)
	if models[0] != "gemini-2.5-flash-preview-tts" {
		t.Errorf("expected flash first, got %s", models[0])
	}
	if models[1] != "gemini-2.5-pro-preview-tts" {
		t.Errorf("expected pro second, got %s", models[1])
	}
}

func TestDiscoverTTSModelsNoTTSModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := geminiModelListResponse{
			Models: []geminiModelInfo{
				{Name: "models/gemini-2.0-flash", SupportedGenerationMethods: []string{"generateContent"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	old := modelDiscoveryBaseURL
	modelDiscoveryBaseURL = srv.URL
	defer func() { modelDiscoveryBaseURL = old }()

	_, err := DiscoverTTSModels("test-key")
	if err == nil {
		t.Fatal("expected error when no TTS models found")
	}
}

func TestDiscoverTTSModelsAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "internal error")
	}))
	defer srv.Close()

	old := modelDiscoveryBaseURL
	modelDiscoveryBaseURL = srv.URL
	defer func() { modelDiscoveryBaseURL = old }()

	_, err := DiscoverTTSModels("test-key")
	if err == nil {
		t.Fatal("expected error on API failure")
	}
}

func TestDiscoverTTSModelsSortOrder(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := geminiModelListResponse{
			Models: []geminiModelInfo{
				{Name: "models/gemini-2.5-pro-preview-tts", SupportedGenerationMethods: []string{"generateContent"}},
				{Name: "models/gemini-2.5-flash-tts", SupportedGenerationMethods: []string{"generateContent"}},
				{Name: "models/gemini-2.5-pro-tts", SupportedGenerationMethods: []string{"generateContent"}},
				{Name: "models/gemini-2.5-flash-preview-tts", SupportedGenerationMethods: []string{"generateContent"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	old := modelDiscoveryBaseURL
	modelDiscoveryBaseURL = srv.URL
	defer func() { modelDiscoveryBaseURL = old }()

	models, err := DiscoverTTSModels("test-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Expected order: flash (non-preview), flash-preview, pro (non-preview), pro-preview
	expected := []string{
		"gemini-2.5-flash-tts",
		"gemini-2.5-flash-preview-tts",
		"gemini-2.5-pro-tts",
		"gemini-2.5-pro-preview-tts",
	}
	if len(models) != len(expected) {
		t.Fatalf("expected %d models, got %d: %v", len(expected), len(models), models)
	}
	for i, m := range expected {
		if models[i] != m {
			t.Errorf("models[%d] = %s, want %s", i, models[i], m)
		}
	}
}

// --- Priority sort tests ---

func TestTTSModelPriority(t *testing.T) {
	tests := []struct {
		model    string
		expected int
	}{
		{"gemini-2.5-flash-tts", 0},
		{"gemini-2.5-flash-preview-tts", 1},
		{"gemini-2.5-pro-tts", 10},
		{"gemini-2.5-pro-preview-tts", 11},
	}
	for _, tt := range tests {
		got := ttsModelPriority(tt.model)
		if got != tt.expected {
			t.Errorf("ttsModelPriority(%q) = %d, want %d", tt.model, got, tt.expected)
		}
	}
}

// --- Cache tests ---

func TestResolveTTSModelsFreshCache(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "tts_models.json")

	// Write a fresh cache
	cache := ttsModelsCache{
		Models:     []string{"cached-model-a", "cached-model-b"},
		Discovered: time.Now(),
	}
	data, _ := json.Marshal(cache)
	os.WriteFile(cachePath, data, 0644)

	// Override ttsModelsCachePath to use temp dir
	origCachePath := ttsModelsCachePath
	_ = origCachePath // ttsModelsCachePath is a func, we can't override it directly

	// Test readTTSModelsCache directly
	cached, err := readTTSModelsCache(cachePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cached.Models) != 2 {
		t.Fatalf("expected 2 cached models, got %d", len(cached.Models))
	}
	if cached.Models[0] != "cached-model-a" {
		t.Errorf("expected cached-model-a, got %s", cached.Models[0])
	}
}

func TestWriteAndReadCache(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "subdir", "tts_models.json")

	models := []string{"model-x", "model-y"}
	if err := writeTTSModelsCache(cachePath, models); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	cached, err := readTTSModelsCache(cachePath)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if len(cached.Models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(cached.Models))
	}
	if cached.Models[0] != "model-x" || cached.Models[1] != "model-y" {
		t.Errorf("unexpected models: %v", cached.Models)
	}
	if time.Since(cached.Discovered) > time.Minute {
		t.Error("discovered timestamp too old")
	}
}

func TestReadCacheMissingFile(t *testing.T) {
	_, err := readTTSModelsCache("/nonexistent/path/tts_models.json")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestReadCacheCorruptJSON(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "tts_models.json")
	os.WriteFile(cachePath, []byte("not json"), 0644)

	_, err := readTTSModelsCache(cachePath)
	if err == nil {
		t.Error("expected error for corrupt JSON")
	}
}

// --- CheckModelQuota tests ---

func TestCheckModelQuotaSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Respond with minimal success
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"candidates":[{"content":{"parts":[{"inlineData":{"data":"AAAA"}}]}}]}`)
	}))
	defer srv.Close()

	old := modelDiscoveryBaseURL
	modelDiscoveryBaseURL = srv.URL
	defer func() { modelDiscoveryBaseURL = old }()

	ok, err := CheckModelQuota("test-key", "test-model", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Error("expected quota available")
	}
}

func TestCheckModelQuotaZeroLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		fmt.Fprint(w, `{
			"error": {
				"code": 429,
				"message": "Resource exhausted",
				"details": [
					{"@type": "type.googleapis.com/google.rpc.QuotaFailure", "violations": [{"quotaMetric": "per_day"}]},
					{"limit": 0}
				]
			}
		}`)
	}))
	defer srv.Close()

	old := modelDiscoveryBaseURL
	modelDiscoveryBaseURL = srv.URL
	defer func() { modelDiscoveryBaseURL = old }()

	ok, err := CheckModelQuota("test-key", "test-model", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected no quota for zero limit")
	}
}

func TestCheckModelQuotaTransientRateLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(429)
		fmt.Fprint(w, `{
			"error": {
				"details": [
					{"@type": "type.googleapis.com/google.rpc.RetryInfo", "retryDelay": "34s"},
					{"@type": "type.googleapis.com/google.rpc.QuotaFailure", "violations": [{"subject": "PerMinute"}]}
				]
			}
		}`)
	}))
	defer srv.Close()

	old := modelDiscoveryBaseURL
	modelDiscoveryBaseURL = srv.URL
	defer func() { modelDiscoveryBaseURL = old }()

	ok, err := CheckModelQuota("test-key", "test-model", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Error("expected no quota for transient rate limit")
	}
}

func TestCheckModelQuotaServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, "internal error")
	}))
	defer srv.Close()

	old := modelDiscoveryBaseURL
	modelDiscoveryBaseURL = srv.URL
	defer func() { modelDiscoveryBaseURL = old }()

	ok, err := CheckModelQuota("test-key", "test-model", false)
	if err == nil {
		t.Fatal("expected error on server error")
	}
	if ok {
		t.Error("expected false on server error")
	}
}

// --- hasZeroLimit tests ---

func TestHasZeroLimit(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "limit: 0 at top level",
			body: `{"limit": 0}`,
			want: true,
		},
		{
			name: "limit: 0 nested in error",
			body: `{"error": {"details": [{"limit": 0}]}}`,
			want: true,
		},
		{
			name: "limit: 100",
			body: `{"limit": 100}`,
			want: false,
		},
		{
			name: "no limit field",
			body: `{"error": {"code": 429}}`,
			want: false,
		},
		{
			name: "invalid JSON",
			body: `not json`,
			want: false,
		},
		{
			name: "empty body",
			body: `{}`,
			want: false,
		},
		{
			name: "real Gemini zero-quota response",
			body: `{
				"error": {
					"code": 429,
					"message": "You exceeded your current quota",
					"status": "RESOURCE_EXHAUSTED",
					"details": [
						{"@type": "type.googleapis.com/google.rpc.QuotaFailure", "violations": [{"quotaMetric": "per_day", "quotaId": "PerDay", "limit": 0}]}
					]
				}
			}`,
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasZeroLimit([]byte(tt.body))
			if got != tt.want {
				t.Errorf("hasZeroLimit() = %v, want %v", got, tt.want)
			}
		})
	}
}
