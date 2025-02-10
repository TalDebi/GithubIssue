package controller

import (
	"context"
	"encoding/base64"
	"errors"
	danaiov1alpha1 "github.com/TalDebi/GithubIssue.git/api/v1alpha1"
	"github.com/TalDebi/GithubIssue.git/internal/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"net/url"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
)

// fetchGithubIssue fetches the current GithubIssue details.
func fetchGithubIssue(ctx context.Context, c client.Reader, namespacedName types.NamespacedName) (*danaiov1alpha1.GithubIssue, error) {
	githubIssue := &danaiov1alpha1.GithubIssue{}
	if err := c.Get(ctx, namespacedName, githubIssue); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	return githubIssue, nil
}

// fetchGithubIssues retrieves GithubIssue resources in the given namespace.
func fetchGithubIssues(ctx context.Context, c client.Reader, namespaceName string) (*danaiov1alpha1.GithubIssueList, error) {
	logger := log.FromContext(ctx)

	existingGithubIssues := &danaiov1alpha1.GithubIssueList{}

	if err := c.List(ctx, existingGithubIssues, client.InNamespace(namespaceName)); err != nil {
		logger.Error(err, "Failed to list GithubIssues", "namespaceName", namespaceName)
		return nil, err
	}

	logger.Info("Successfully fetched GithubIssues", "namespaceName", namespaceName, "count", len(existingGithubIssues.Items))

	return existingGithubIssues, nil
}

// fetchNamespace fetches the current Namespace details.
func fetchNamespace(ctx context.Context, c client.Reader, namespacedName string) (*corev1.Namespace, error) {
	ns := &corev1.Namespace{}
	if err := c.Get(ctx, types.NamespacedName{Namespace: namespacedName, Name: namespacedName}, ns); err != nil {
		return nil, client.IgnoreNotFound(err)
	}
	return ns, nil
}

// listGithubIssuesInNamespace fetches all githubIssues in a namespace.
func listGithubIssuesInNamespace(ctx context.Context, c client.Reader, namespaceName string) (*danaiov1alpha1.GithubIssueList, error) {
	existingGithubIssues := &danaiov1alpha1.GithubIssueList{}
	if err := c.List(ctx, existingGithubIssues, client.InNamespace(namespaceName)); err != nil {
		return nil, client.IgnoreNotFound(err)
	}

	return existingGithubIssues, nil
}

// updateConditions updates the conditions.
func (r *GithubIssueReconciler) updateConditions(ctx context.Context, githubIssue *danaiov1alpha1.GithubIssue, conditionType string, status metav1.ConditionStatus, reason, message string) error {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}

	githubIssue.Status.Conditions = updateNewCondition(githubIssue.Status.Conditions, condition)

	if err := r.Status().Update(ctx, githubIssue); err != nil {
		r.Log.Error(err, "Failed to update GithubIssue status", "GithubIssue", githubIssue.Name)
		return err
	}

	return nil
}

// updateNewCondition appends a new condition or updates an existing one in the slice of conditions.
func updateNewCondition(conditions []metav1.Condition, newCondition metav1.Condition) []metav1.Condition {
	for index := range conditions {
		if conditions[index].Type == newCondition.Type {
			conditions[index] = newCondition
			return conditions
		}
	}

	return append(conditions, newCondition)
}

// parseRepoURL extracts owner and repo name from a GitHub URL.
func parseRepoURL(repoURL string) (owner, repo string, err error) {
	parsedURL, err := url.Parse(repoURL)
	if err != nil {
		return "", "", errors.New("invalid URL format")
	}

	if parsedURL.Host != common.GithubUriHost {
		return "", "", errors.New("URL is not a valid GitHub repository")
	}

	parts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(parts) != 2 {
		return "", "", errors.New("invalid repo format")
	}

	return parts[0], parts[1], nil
}

// getGitHubToken retrieves the GitHub token and decodes it if needed.
func getGitHubToken() (string, error) {
	token := common.GithubRepoToken
	if token == "" {
		return "", errors.New("GITHUB_TOKEN environment variable not set")
	}

	if decoded, err := base64.StdEncoding.DecodeString(token); err == nil {
		return string(decoded), nil
	}

	return token, nil
}
