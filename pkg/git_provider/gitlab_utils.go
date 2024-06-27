package git_provider

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"

	"github.com/quickube/piper/pkg/conf"
	"github.com/quickube/piper/pkg/utils"
	"github.com/xanzy/go-gitlab"
	"golang.org/x/net/context"
)

func ValidateGitlabPermissions(ctx context.Context, client *gitlab.Client, cfg *conf.GlobalConfig) error {

	repoAdminScopes := []string{"api"}
	repoGranularScopes := []string{"write_repository", "read_api"}

	scopes, err := GetGitlabScopes(ctx, client)

	if err != nil {
		return fmt.Errorf("failed to get scopes: %v", err)
	}
	if len(scopes) == 0 {
		return fmt.Errorf("permissions error: no scopes found for the gitlab client")
	}

	if utils.ListContains(repoAdminScopes, scopes) {
		return nil
	}
	if utils.ListContains(repoGranularScopes, scopes) {
		return nil
	}

	return fmt.Errorf("permissions error: %v is not a valid scope for the project level permissions", scopes)
}

func GetGitlabScopes(ctx context.Context, client *gitlab.Client) ([]string, error) {
   
    user, resp,err := client.Users.CurrentUser()
	fmt.Println(user.ID)
    if err != nil{
		return nil, err
    }
    if resp.StatusCode == 400 {
		return nil, err
    }
    a := gitlab.ListPersonalAccessTokensOptions{
        UserID: &user.ID,
	}
    accessTokens, resp,err := client.PersonalAccessTokens.ListPersonalAccessTokens(&a)
    fmt.Println(accessTokens)
    if err != nil{
		return nil, err
    }
    if resp.StatusCode == 400 {
		return nil, err
    }

	scopes := accessTokens[0].Scopes
	fmt.Println("Gitlab Token Scopes are:", scopes)

	return scopes, nil
}

func IsGroupWebhookEnabled(c *GitlabClientImpl) (*gitlab.GroupHook, bool) {
	emptyHook := gitlab.GroupHook{}
	hooks, resp, err := c.client.Groups.ListGroupHooks(c.cfg.GitProviderConfig.OrgName, nil)
	if err != nil {
		return &emptyHook, false
	}
	if resp.StatusCode != 200 {
		return &emptyHook, false
	}
	if len(hooks) == 0 {
		return &emptyHook, false
	}
	for _, hook := range hooks {
		if hook.AlertStatus == "triggered" && hook.URL == c.cfg.GitProviderConfig.WebhookURL {
			return hook, true
		}
	}
	return &emptyHook, false
}

func isProjectWebhookEnabled(c *GitlabClientImpl, project string) (*gitlab.ProjectHook, bool) {
	emptyHook := gitlab.ProjectHook{}
	hooks, resp, err := c.client.Projects.ListProjectHooks(project, nil)
	if err != nil {
		return &emptyHook, false
	}
	if resp.StatusCode != 200 {
		return &emptyHook, false
	}
	if len(hooks) == 0 {
		return &emptyHook, false
	}

	for _, hook := range hooks {
		if hook.URL == c.cfg.GitProviderConfig.WebhookURL {
			return hook, true
		}
	}

	return &emptyHook, false
}

func extractLabelsId(labels []*gitlab.EventLabel) []string {
	var returnLabelsList []string
	for _, label := range labels {
		returnLabelsList = append(returnLabelsList, fmt.Sprint(label.ID))
	}
	return returnLabelsList
}

func validatePayload(r *http.Request, secret []byte) ([]byte, error){
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading request body: %v", err)
	}

	// Get GitLab signature from headers
	gitlabSignature := r.Header.Get("X-Gitlab-Token")
	if gitlabSignature == "" {
		return nil, fmt.Errorf("no GitLab signature found in headers")
	}

	h := hmac.New(sha256.New, secret)
	_, err = h.Write(payload)
	if err != nil {
		return nil, fmt.Errorf("error computing HMAC: %v", err)
	}
	expectedMAC := hex.EncodeToString(h.Sum(nil))

	isEquall := hmac.Equal([]byte(gitlabSignature), []byte(expectedMAC))
	if !isEquall {
		return nil, fmt.Errorf("secret not correct")
	}
	return payload, nil
}


