package social

// TokenStatus represents the status of tokens for all platforms
type TokenStatus struct {
	X         *PlatformTokenStatus `json:"x,omitempty"`
	Facebook  *PlatformTokenStatus `json:"facebook,omitempty"`
	Instagram *PlatformTokenStatus `json:"instagram,omitempty"`
}

// PlatformTokenStatus represents token status for a single platform
type PlatformTokenStatus struct {
	Valid         bool   `json:"valid"`
	Handle        string `json:"handle,omitempty"`
	Page          string `json:"page,omitempty"`
	ExpiresInDays int    `json:"expires_in_days,omitempty"`
	Error         string `json:"error,omitempty"`
}

// RefreshResult represents the result of a token refresh operation
type RefreshResult struct {
	Success          bool   `json:"success"`
	OldExpiresInDays int    `json:"old_expires_in_days,omitempty"`
	NewExpiresInDays int    `json:"new_expires_in_days,omitempty"`
	Message          string `json:"message"`
	Error            string `json:"error,omitempty"`
}

// CampaignPostResult represents the result of posting a campaign item
type CampaignPostResult struct {
	Success  bool                      `json:"success"`
	PostedTo map[string]*PlatformPost  `json:"posted_to,omitempty"`
	Errors   map[string]string         `json:"errors,omitempty"`
	Status   string                    `json:"status"`
	DryRun   bool                      `json:"dry_run,omitempty"`
}

// PlatformPost represents a post to a single platform
type PlatformPost struct {
	ID  string `json:"id"`
	URL string `json:"url,omitempty"`
}

// PrelaunchResult represents the result of prelaunch checks
type PrelaunchResult struct {
	Passed   bool                    `json:"passed"`
	Checks   map[string]*CheckResult `json:"checks"`
	Warnings int                     `json:"warnings"`
	Errors   int                     `json:"errors"`
}

// CheckResult represents a single prelaunch check result
type CheckResult struct {
	Status  string `json:"status"` // "pass", "warn", "fail"
	Message string `json:"message,omitempty"`
}
