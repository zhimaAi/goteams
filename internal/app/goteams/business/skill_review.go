package business

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"goteams/internal/app/goteams/define"
	"goteams/internal/app/goteams/validate"

	"github.com/zhimaAi/go_tools/logs"
)

var skillGitHubClient = &http.Client{
	Timeout: 3 * time.Second,
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

var skillGitHubOwner = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*$`)
var skillGitHubRepo = regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)

func skillGitHubRepository(address string) (string, error) {
	u, err := url.ParseRequestURI(address)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || !strings.EqualFold(u.Host, "github.com") || u.User != nil {
		return "", &validate.FieldError{Field: "source_url", Key: define.MsgSkillGitHubURLInvalid}
	}
	parts := strings.Split(strings.TrimPrefix(u.EscapedPath(), "/"), "/")
	if len(parts) < 2 {
		return "", &validate.FieldError{Field: "source_url", Key: define.MsgSkillGitHubURLInvalid}
	}
	owner, repo := parts[0], strings.TrimSuffix(parts[1], ".git")
	if !skillGitHubOwner.MatchString(owner) || !skillGitHubRepo.MatchString(repo) || repo == "." || repo == ".." {
		return "", &validate.FieldError{Field: "source_url", Key: define.MsgSkillGitHubURLInvalid}
	}
	return owner + "/" + repo, nil
}

func reviewSkill(source, sourceURL string) (string, error) {
	if source != define.SkillSourceGitHub {
		return define.SkillReviewPending, nil
	}
	repository, err := skillGitHubRepository(sourceURL)
	if err != nil {
		return "", err
	}
	stars, err := skillGitHubStars(repository)
	if err != nil {
		logs.Error("skills.github_review repository:%s, err:%s", repository, err.Error())
		return define.SkillReviewPending, nil
	}
	if stars > 100 {
		return define.SkillReviewApproved, nil
	}
	return define.SkillReviewPending, nil
}

func reviewSkillUpdate(current SkillModel, model UpdateSkillModel) (*string, error) {
	source, sourceURL := current.Source, current.SourceURL
	if model.Source != nil {
		source = *model.Source
	}
	if model.SourceURL != nil {
		sourceURL = *model.SourceURL
	}
	if source == current.Source && sourceURL == current.SourceURL {
		return nil, nil
	}
	if model.ReviewStatus != nil {
		return nil, &validate.FieldError{Field: "review_status", Key: define.MsgSkillReviewWithSourceChange}
	}
	status, err := reviewSkill(source, sourceURL)
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func skillGitHubStars(repository string) (int, error) {
	request, err := http.NewRequest(http.MethodGet, "https://api.github.com/repos/"+repository, nil)
	if err != nil {
		return 0, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "GoTeams")
	response, err := skillGitHubClient.Do(request)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("GitHub returned HTTP %d", response.StatusCode)
	}
	const maxBody = 1 << 20
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBody+1))
	if err != nil {
		return 0, err
	}
	if len(body) > maxBody {
		return 0, fmt.Errorf("GitHub response exceeds %d bytes", maxBody)
	}
	var result struct {
		Stars *int `json:"stargazers_count"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, err
	}
	if result.Stars == nil || *result.Stars < 0 {
		return 0, fmt.Errorf("GitHub response has invalid stargazers_count")
	}
	return *result.Stars, nil
}
