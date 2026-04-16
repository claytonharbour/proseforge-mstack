package social

// Service provides social media management operations
type Service interface {
	// X/Twitter operations
	ShowXProfile(project string) (map[string]interface{}, error)
	UpdateXProfile(project string, settings map[string]string) error
	PostToX(project string, text string) (string, error) // Returns tweet ID

	// Facebook operations
	ShowFacebookPage(project string) (map[string]interface{}, error)
	UpdateFacebookPage(project string, updates map[string]string) error
	PostToFacebook(project string, message string) (string, error) // Returns post ID

	// Instagram operations
	ShowInstagramProfile(project string) (map[string]interface{}, error)
	UpdateInstagramProfile(project string, updates map[string]string) error
	PostToInstagram(project string, caption string, imageURL string) (string, error) // Returns media ID

	// Campaign operations
	LoadCampaign(project string, campaign string) (map[string]interface{}, error)
	SaveCampaign(project string, campaign string, data map[string]interface{}) error
	ListPosts(project string, campaign string) error
	PostCampaignItem(project string, campaign string, postID string) error

	// Secrets operations
	SyncSecrets(project string, action string) error // action: "push", "pull", "list", "diff"
	CheckCredentials(project string) (bool, []string) // Returns (ok, issues)

	// Token and prelaunch operations
	CheckTokens(project string, platforms []string) (*TokenStatus, error)
	RefreshFacebookToken(project string) (*RefreshResult, error)
	RunPrelaunchCheck(project, campaign string) (*PrelaunchResult, error)
	PostCampaignItemWithResult(project, campaign, postID string, dryRun bool) (*CampaignPostResult, error)
}

type service struct{}

// NewService creates a new SocialService
func NewService() Service {
	return &service{}
}

func (s *service) ShowXProfile(project string) (map[string]interface{}, error) {
	return showXProfile(project)
}

func (s *service) UpdateXProfile(project string, settings map[string]string) error {
	return updateXProfile(project, settings)
}

func (s *service) PostToX(project string, text string) (string, error) {
	return postToX(project, text)
}

func (s *service) ShowFacebookPage(project string) (map[string]interface{}, error) {
	return showFacebookPage(project)
}

func (s *service) UpdateFacebookPage(project string, updates map[string]string) error {
	return updateFacebookPage(project, updates)
}

func (s *service) PostToFacebook(project string, message string) (string, error) {
	return postToFacebook(project, message)
}

func (s *service) ShowInstagramProfile(project string) (map[string]interface{}, error) {
	return showInstagramProfile(project)
}

func (s *service) UpdateInstagramProfile(project string, updates map[string]string) error {
	return updateInstagramProfile(project, updates)
}

func (s *service) PostToInstagram(project string, caption string, imageURL string) (string, error) {
	return postToInstagram(project, caption, imageURL)
}

func (s *service) LoadCampaign(project string, campaign string) (map[string]interface{}, error) {
	return loadCampaign(project, campaign)
}

func (s *service) SaveCampaign(project string, campaign string, data map[string]interface{}) error {
	return saveCampaign(project, campaign, data)
}

func (s *service) ListPosts(project string, campaign string) error {
	return listPosts(project, campaign)
}

func (s *service) PostCampaignItem(project string, campaign string, postID string) error {
	return postCampaignItem(project, campaign, postID)
}

func (s *service) SyncSecrets(project string, action string) error {
	return syncSecrets(project, action)
}

func (s *service) CheckCredentials(project string) (bool, []string) {
	return checkCredentials(project)
}

func (s *service) CheckTokens(project string, platforms []string) (*TokenStatus, error) {
	return checkTokens(project, platforms)
}

func (s *service) RefreshFacebookToken(project string) (*RefreshResult, error) {
	return refreshFacebookToken(project)
}

func (s *service) RunPrelaunchCheck(project, campaign string) (*PrelaunchResult, error) {
	return runPrelaunchCheck(project, campaign)
}

func (s *service) PostCampaignItemWithResult(project, campaign, postID string, dryRun bool) (*CampaignPostResult, error) {
	return postCampaignItemWithResult(project, campaign, postID, dryRun)
}
