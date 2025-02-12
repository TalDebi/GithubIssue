package common

import (
	"context"
	"encoding/base64"
	"fmt"

	"errors"
	"net/url"
	"strings"

	"github.com/google/go-github/v50/github"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	danaiov1alpha1 "github.com/TalDebi/GithubIssue/api/v1alpha1"
)

// FetchGithubIssue fetches the current GithubIssue details.
func FetchGithubIssue(ctx context.Context, c client.Reader, namespacedName types.NamespacedName) (*danaiov1alpha1.Issue, error) {
	githubIssue := &danaiov1alpha1.Issue{}
	if err := c.Get(ctx, namespacedName, githubIssue); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	return githubIssue, nil
}

// UpdateIssueStatus updates the status of the GitHub issue.
func UpdateIssueStatus(c client.Client, ctx context.Context, githubIssue *danaiov1alpha1.Issue, conditionType string, status metav1.ConditionStatus, reason, message string) error {

	condition := generateNewCondition(conditionType, status, reason, message)

	githubIssue.Status.Conditions = updateIssueConditions(githubIssue.Status.Conditions, condition)

	if err := c.Status().Update(ctx, githubIssue); err != nil {
		return fmt.Errorf("failed to update GithubIssue status: %w", err)
	}

	return nil
}

// generateNewCondition generates a new condition.
func generateNewCondition(conditionType string, status metav1.ConditionStatus, reason, message string) metav1.Condition {
	return metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

// updateIssueConditions adds a new condition or updates an existing one in the slice of conditions.
func updateIssueConditions(conditions []metav1.Condition, newCondition metav1.Condition) []metav1.Condition {
	for index := range conditions {
		if conditions[index].Type == newCondition.Type {
			conditions[index] = newCondition
			return conditions
		}
	}

	return append(conditions, newCondition)
}

// ParseRepoURL extracts owner and repo name from a GitHub URL.
func ParseRepoURL(repoURL string) (owner, repo string, err error) {
	parsedURL, err := url.Parse(repoURL)
	if err != nil {
		return "", "", errors.New("invalid URL format")
	}

	if parsedURL.Host != GithubUriHost {
		return "", "", errors.New("URL is not a valid GitHub repository")
	}

	parts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(parts) != 2 {
		return "", "", errors.New("invalid repo format")
	}

	return parts[0], parts[1], nil
}

// GetGitHubToken retrieves the GitHub token and decodes it if needed.
func GetGitHubToken() (string, error) {
	token := GithubRepoToken
	if token == "" {
		return "", errors.New("GITHUB_TOKEN environment variable not set")
	}

	if decoded, err := base64.StdEncoding.DecodeString(token); err == nil {
		return string(decoded), nil
	}

	return token, nil
}

// FindIssueByTitle searches by title the issue in the GitHub repo.
func FindIssueByTitle(issues []*github.Issue, title string) *github.Issue {
	for _, issue := range issues {
		if issue.GetTitle() == title {
			return issue
		}
	}
	return nil
}

// MapIssueStateToConditionStatus converts an issue state string to a Kubernetes ConditionStatus.
func MapIssueStateToConditionStatus(state string) metav1.ConditionStatus {
	switch state {
	case "open":
		return metav1.ConditionTrue
	case "closed":
		return metav1.ConditionFalse
	default:
		return metav1.ConditionUnknown
	}
}
