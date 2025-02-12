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

	danaiov1alpha1 "github.com/TalDebi/GithubIssue/api/v1alpha1"
	"github.com/TalDebi/GithubIssue/internal/common"
	"github.com/go-logr/logr"
	"github.com/google/go-github/v50/github"
	"golang.org/x/oauth2"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// IssueReconciler reconciles a Issue object
type IssueReconciler struct {
	Scheme *runtime.Scheme
	client.Client
	Log          logr.Logger
	GitHubClient *github.Client
}

// +kubebuilder:rbac:groups=github.dana.io,resources=issues,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=github.dana.io,resources=issues/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=github.dana.io,resources=issues/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=list;watch;get;update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *IssueReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	logger := log.FromContext(ctx)

	logger.Info("Starting reconciliation for GithubIssue", "Namespace", req.Namespace, "Name", req.Name)

	githubIssue, err := common.FetchGithubIssue(ctx, r.Client, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, err
	}
	if githubIssue == nil {
		return ctrl.Result{}, nil
	}

	logger.Info("Fetched GithubIssue", "GithubIssue", githubIssue)

	repoOwner, repoName, err := common.ParseRepoURL(githubIssue.Spec.Repo)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("invalid repository URL: %w", err)
	}

	logger.Info("repo details", "repoOwner", repoOwner, "repoName", repoName)

	if githubIssue.ObjectMeta.DeletionTimestamp.IsZero() {
		if !controllerutil.ContainsFinalizer(githubIssue, common.FinalizerName) {
			controllerutil.AddFinalizer(githubIssue, common.FinalizerName)
			if err := r.Update(ctx, githubIssue); err != nil {
				return ctrl.Result{}, err
			}
		}
	} else {
		if controllerutil.ContainsFinalizer(githubIssue, common.FinalizerName) {
			if err := r.handleDelete(ctx, githubIssue); err != nil {
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	logger.Info("Applying Github issue")

	appliedIssue, err := r.applyIssue(ctx, repoOwner, repoName, githubIssue)
	if err != nil {
		return ctrl.Result{}, err
	}

	appliedIssueState := appliedIssue.GetState()
	if err := common.UpdateIssueStatus(
		r.Client,
		ctx,
		githubIssue,
		appliedIssueState,
		common.MapIssueStateToConditionStatus(appliedIssueState),
		"StatusUpdated",
		"Github issue status has been updated",
	); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("Github issue has been applied")

	return ctrl.Result{RequeueAfter: common.ResyncPeriod}, nil
}

// handleDelete handles deletion by closing the GitHub issue
func (r *IssueReconciler) handleDelete(ctx context.Context, githubIssue *danaiov1alpha1.Issue) error {
	logger := log.FromContext(ctx)

	if err := r.closeIssue(ctx, githubIssue); err != nil {
		return fmt.Errorf("failed to close issue: %w", err)
	}

	if err := r.removeFinalizer(ctx, githubIssue); err != nil {
		return fmt.Errorf("failed to remove finalizer: %w", err)
	}

	logger.Info("Deletion handled successfully", "GitHubIssue", githubIssue)

	return nil
}

// removeFinalizer removes the finalizer from the GitHub issue during the deletion process.
func (r *IssueReconciler) removeFinalizer(
	ctx context.Context, githubIssue *danaiov1alpha1.Issue) error {
	logger := log.FromContext(ctx)

	controllerutil.RemoveFinalizer(githubIssue, common.FinalizerName)

	if err := r.Update(ctx, githubIssue); err != nil {
		return fmt.Errorf("failed to update NamespaceLabel to remove finalizer: %w", err)
	}

	logger.Info("Finalizer removed successfully", "GitHubIssue", githubIssue)

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IssueReconciler) SetupWithManager(mgr ctrl.Manager) error {
	ctx := context.Background()

	token, err := common.GetGitHubToken()
	if err != nil {
		return err
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	r.GitHubClient = github.NewClient(tc)

	return ctrl.NewControllerManagedBy(mgr).
		For(&danaiov1alpha1.Issue{}).
		Named("issue").
		Complete(r)
}
