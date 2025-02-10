/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"github.com/google/go-github/v50/github"
	"golang.org/x/oauth2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"

	"github.com/TalDebi/GithubIssue.git/internal/common"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	danaiov1alpha1 "github.com/TalDebi/GithubIssue.git/api/v1alpha1"
)

// GithubIssueReconciler reconciles a GithubIssue object
type GithubIssueReconciler struct {
	Scheme *runtime.Scheme
	client.Client
	Log          logr.Logger
	GitHubClient *github.Client
}

// +kubebuilder:rbac:groups=githubissue.dana.io,resources=githubissues,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=githubissue.dana.io,resources=githubissues/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=githubissue.dana.io,resources=githubissues/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=list;watch;get;update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *GithubIssueReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := log.FromContext(ctx)

	logger.Info("Starting reconciliation for GithubIssue", "Namespace", req.Namespace, "Name", req.Name)

	githubIssue, err := fetchGithubIssue(ctx, r.Client, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, err
	}
	if githubIssue == nil {
		return ctrl.Result{}, nil
	}

	logger.Info("Fetched GithubIssue", "GithubIssue", githubIssue)

	ns, err := fetchNamespace(ctx, r.Client, req.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	if ns == nil {
		return ctrl.Result{}, nil
	}

	logger.Info("Fetched Namespace", "GithubIssue", ns)

	if err := r.updateConditions(ctx, githubIssue, "LabelsApplied", metav1.ConditionTrue, "Success", "GithubIssue have been successfully updated"); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update conditions: %w", err)
	}

	repoOwner, repoName, err := parseRepoURL(githubIssue.Spec.Repo)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("invalid repository URL: %w", err)
	}

	logger.Info("repo details", "repoOwner", repoOwner, "repoName", repoName)

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	issues, _, err := r.GitHubClient.Issues.ListByRepo(ctxWithTimeout, repoOwner, repoName, nil)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to fetch issues: %w", err)
	}

	var existingIssue *github.Issue
	for _, i := range issues {
		if i.GetTitle() == githubIssue.Spec.Title {
			existingIssue = i
			break
		}
	}

	logger.Info("1", "GithubIssue", existingIssue)

	//if existingIssue == nil {
	//	// Create new issue
	//	newIssue := &github.IssueRequest{
	//		Title: github.String(githubIssue.Spec.Title),
	//		Body:  github.String(githubIssue.Spec.Description),
	//	}
	//	createdIssue, _, err := r.GitHubClient.Issues.Create(ctxWithTimeout, repoOwner, repoName, newIssue)
	//	if err != nil {
	//		return ctrl.Result{}, fmt.Errorf("failed to create GitHub issue: %w", err)
	//	}
	//	existingIssue = createdIssue
	//} else if existingIssue.GetBody() != githubIssue.Spec.Description {
	//	// Update issue description
	//	update := &github.IssueRequest{
	//		Body: github.String(githubIssue.Spec.Description),
	//	}
	//	_, _, err := r.GitHubClient.Issues.Edit(ctxWithTimeout, repoOwner, repoName, existingIssue.GetNumber(), update)
	//	if err != nil {
	//		return ctrl.Result{}, fmt.Errorf("failed to update GitHub issue: %w", err)
	//	}
	//}
	//
	//// Update CRD status
	//githubIssue.Status.Conditions = []metav1.Condition{{
	//	Type:   "Open",
	//	Status: metav1.ConditionStatus(existingIssue.GetState()),
	//}}
	//if err := r.Status().Update(ctx, githubIssue); err != nil {
	//	return ctrl.Result{}, err
	//}
	//
	return ctrl.Result{RequeueAfter: common.ResyncPeriod}, nil
}

// Finalizer: Handle deletion by closing the GitHub issue
func (r *GithubIssueReconciler) finalizeIssue(ctx context.Context, issue *danaiov1alpha1.GithubIssue) error {
	repoOwner, repoName, err := parseRepoURL(issue.Spec.Repo)
	if err != nil {
		return fmt.Errorf("invalid repository URL: %w", err)
	}

	ctxWithTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
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

// SetupWithManager sets up the controller with the Manager.
func (r *GithubIssueReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ctx := context.Background()

	token, err := getGitHubToken()
	if err != nil {
		return err
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	r.GitHubClient = github.NewClient(tc)

	return ctrl.NewControllerManagedBy(mgr).
		For(&danaiov1alpha1.GithubIssue{}).
		Named("githubissue").
		Complete(r)
}
