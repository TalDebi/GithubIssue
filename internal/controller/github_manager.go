package controller

import (
	"context"
	"fmt"

	danaiov1alpha1 "github.com/TalDebi/GithubIssue/api/v1alpha1"
	"github.com/TalDebi/GithubIssue/internal/common"
	"github.com/google/go-github/v50/github"
)

// applyIssue checks if an issue exists, creates a new issue or updates the existing one.
func (r *IssueReconciler) applyIssue(ctx context.Context, repoOwner, repoName string, githubIssue *danaiov1alpha1.Issue) (*github.Issue, error) {
	existingIssue, err := r.fetchExistingIssue(ctx, repoOwner, repoName, githubIssue.Spec.Title)
	if err != nil {
		return nil, err
	}

	if existingIssue == nil {
		return r.createIssue(ctx, repoOwner, repoName, githubIssue)
	}

	if existingIssue.GetBody() != githubIssue.Spec.Description {
		return r.updateIssue(ctx, repoOwner, repoName, existingIssue, githubIssue)
	}

	return existingIssue, nil
}

// fetchExistingIssue finds issue by title from the GitHub repo.
func (r *IssueReconciler) fetchExistingIssue(ctx context.Context, repoOwner, repoName, title string) (*github.Issue, error) {
	ctxWithTimeout, cancel := context.WithTimeout(ctx, common.GithubClientTimeout)
	defer cancel()

	issues, _, err := r.GitHubClient.Issues.ListByRepo(ctxWithTimeout, repoOwner, repoName, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch issues: %w", err)
	}

	return common.FindIssueByTitle(issues, title), nil
}

// createIssue creates a new GitHub issue based on the GitHubIssue details.
func (r *IssueReconciler) createIssue(ctx context.Context, repoOwner, repoName string, githubIssue *danaiov1alpha1.Issue) (*github.Issue, error) {
	newIssue := &github.IssueRequest{
		Title: github.String(githubIssue.Spec.Title),
		Body:  github.String(githubIssue.Spec.Description),
	}
	createdIssue, _, err := r.GitHubClient.Issues.Create(ctx, repoOwner, repoName, newIssue)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub issue: %w", err)
	}
	return createdIssue, nil
}

// updateIssue updates the body of the existing GitHub issue.
func (r *IssueReconciler) updateIssue(ctx context.Context, repoOwner, repoName string, existingIssue *github.Issue, githubIssue *danaiov1alpha1.Issue) (*github.Issue, error) {
	update := &github.IssueRequest{
		Body: github.String(githubIssue.Spec.Description),
	}
	_, _, err := r.GitHubClient.Issues.Edit(ctx, repoOwner, repoName, existingIssue.GetNumber(), update)
	if err != nil {
		return nil, fmt.Errorf("failed to update GitHub issue: %w", err)
	}
	return existingIssue, nil
}

// closeIssue closes the GitHub issue.
func (r *IssueReconciler) closeIssue(ctx context.Context, issue *danaiov1alpha1.Issue) error {
	repoOwner, repoName, err := common.ParseRepoURL(issue.Spec.Repo)
	if err != nil {
		return fmt.Errorf("invalid repository URL: %w", err)
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, common.GithubClientTimeout)
	defer cancel()

	issues, _, err := r.GitHubClient.Issues.ListByRepo(ctxWithTimeout, repoOwner, repoName, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch issues: %w", err)
	}

	for _, i := range issues {
		if i.GetTitle() == issue.Spec.Title {
			update := &github.IssueRequest{State: github.String("closed")}
			_, _, err := r.GitHubClient.Issues.Edit(ctxWithTimeout, repoOwner, repoName, i.GetNumber(), update)
			if err != nil {
				return fmt.Errorf("failed to close GitHub issue: %w", err)
			}
		}
	}

	return nil
}
