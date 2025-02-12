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
	"time"

	"github.com/google/go-github/v50/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	danaiov1alpha1 "github.com/TalDebi/GithubIssue/api/v1alpha1"
)

var _ = Describe("GithubIssue Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			testTitle       = "test-title"
			testDescription = "test-description"
			testRepository  = "https://github.com/TalDebi/GithubIssue"
			invalidRepo     = "/invalid/repo"
		)

		ctx := context.Background()
		var (
			typeNamespacedName types.NamespacedName
			githubClient       *github.Client
		)

		BeforeEach(func() {
			mockedHTTPClient := mock.NewMockedHTTPClient(
				mock.WithRequestMatch(
					mock.GetReposIssuesByOwnerByRepo,
					[]*github.Issue{
						{
							Title: github.String(testTitle),
							Body:  github.String(testDescription),
							State: github.String("open"),
						},
					},
					[]*github.Issue{
						{
							Title: github.String(testTitle),
							Body:  github.String(testDescription),
							State: github.String("close"),
						},
					},
				),
				mock.WithRequestMatch(
					mock.PostReposIssuesByOwnerByRepo,
					&github.Issue{
						Title: github.String(testTitle),
						Body:  github.String(testDescription),
						State: github.String("open"),
					},
				),
				mock.WithRequestMatch(
					mock.PatchReposIssuesByOwnerByRepoByIssueNumber,
					&github.Issue{
						Title: github.String(testTitle),
						Body:  github.String(testDescription),
						State: github.String("open"),
					},
					nil,
				),
			)

			githubClient = github.NewClient(mockedHTTPClient)

			resourceName := fmt.Sprintf("test-resource-%d", time.Now().UnixNano())
			testNamespace := fmt.Sprintf("test-ns-%d", time.Now().UnixNano())
			typeNamespacedName = types.NamespacedName{
				Name:      resourceName,
				Namespace: testNamespace,
			}
			By("creating the test namespace")
			ns := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: typeNamespacedName.Namespace,
				},
			}
			Expect(k8sClient.Create(ctx, ns)).To(Succeed())
		})
		AfterEach(func() {
			By("Cleanup the specific resource instance GithubIssue")
			resource := &danaiov1alpha1.Issue{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)

			if err == nil || !errors.IsNotFound(err) {
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}

			By("cleaning up the test namespace")
			ns := &corev1.Namespace{}
			nsErr := k8sClient.Get(ctx, types.NamespacedName{Name: typeNamespacedName.Namespace}, ns)

			if nsErr == nil || !errors.IsNotFound(nsErr) {
				Expect(k8sClient.Delete(ctx, ns)).To(Succeed())
			}
		})

		Context("Create Tests", func() {
			createTests := []struct {
				name            string
				spec            danaiov1alpha1.IssueSpec
				expectToSucceed bool
			}{
				{
					name:            "should fail to create an issue on github if repo is not valid",
					spec:            danaiov1alpha1.IssueSpec{Repo: invalidRepo, Title: testTitle, Description: testDescription},
					expectToSucceed: false},
				{
					name:            "should succeed to create an issue on github if repo is valid",
					spec:            danaiov1alpha1.IssueSpec{Repo: testRepository, Title: testTitle, Description: testDescription},
					expectToSucceed: true},
			}

			for _, tc := range createTests {
				t := tc
				It(t.name, func() {
					By(fmt.Sprintf("Creating the GithubIssue for test: %s", t.name))
					existingGithubIssue := &danaiov1alpha1.Issue{
						ObjectMeta: metav1.ObjectMeta{
							Name:      typeNamespacedName.Name,
							Namespace: typeNamespacedName.Namespace,
						},
						Spec: t.spec,
					}

					err := k8sClient.Create(ctx, existingGithubIssue)
					if !t.expectToSucceed {
						Expect(err).ToNot(Succeed())
					} else {
						Expect(err).To(Succeed())
					}

					By("Reconciling the created resource")
					controllerReconciler := &IssueReconciler{
						Client:       k8sClient,
						Scheme:       k8sClient.Scheme(),
						GitHubClient: githubClient,
					}
					_, reconcileErr := controllerReconciler.Reconcile(ctx, reconcile.Request{
						NamespacedName: typeNamespacedName,
					})
					Expect(reconcileErr).NotTo(HaveOccurred())
				})
			}
		})

		Context("Update Tests", func() {
			updateTests := []struct {
				name            string
				initialSpec     danaiov1alpha1.IssueSpec
				updatedSpec     danaiov1alpha1.IssueSpec
				expectToSucceed bool
			}{
				{
					name:            "should fail to update an issue on github if repo is not valid",
					initialSpec:     danaiov1alpha1.IssueSpec{Repo: testRepository, Title: testTitle, Description: testDescription},
					updatedSpec:     danaiov1alpha1.IssueSpec{Repo: invalidRepo, Title: testTitle, Description: testDescription},
					expectToSucceed: false},
			}

			for _, tc := range updateTests {
				t := tc
				It(t.name, func() {
					By(fmt.Sprintf("Creating the GithubIssue for test: %s", t.name))
					existingGithubIssue := &danaiov1alpha1.Issue{
						ObjectMeta: metav1.ObjectMeta{
							Name:      typeNamespacedName.Name,
							Namespace: typeNamespacedName.Namespace,
						},
						Spec: t.initialSpec,
					}
					Expect(k8sClient.Create(ctx, existingGithubIssue)).To(Succeed())

					By("Reconciling the created resource")
					controllerReconciler := &IssueReconciler{
						Client:       k8sClient,
						Scheme:       k8sClient.Scheme(),
						GitHubClient: githubClient,
					}
					_, reconcileErr := controllerReconciler.Reconcile(ctx, reconcile.Request{
						NamespacedName: typeNamespacedName,
					})
					Expect(reconcileErr).NotTo(HaveOccurred())

					By("Updating the GithubIssue")
					existingGithubIssue.Spec = t.updatedSpec
					err := k8sClient.Update(ctx, existingGithubIssue)
					if !t.expectToSucceed {
						Expect(err).ToNot(Succeed())
					} else {
						Expect(err).To(Succeed())
					}
				})
			}
		})

		Context("Delete Tests", func() {
			deleteTests := []struct {
				name            string
				spec            danaiov1alpha1.IssueSpec
				expectToSucceed bool
			}{
				{
					name:            "should close issue on github when GithubIssue is deleted",
					spec:            danaiov1alpha1.IssueSpec{Repo: testRepository, Title: testTitle, Description: testDescription},
					expectToSucceed: true},
			}

			for _, tc := range deleteTests {
				t := tc
				It(t.name, func() {
					By(fmt.Sprintf("Creating the GithubIssue for test: %s", t.name))
					existingGithubIssue := &danaiov1alpha1.Issue{
						ObjectMeta: metav1.ObjectMeta{
							Name:      typeNamespacedName.Name,
							Namespace: typeNamespacedName.Namespace,
						},
						Spec: t.spec,
					}
					Expect(k8sClient.Create(ctx, existingGithubIssue)).To(Succeed())

					By("Reconciling the created resource")
					controllerReconciler := &IssueReconciler{
						Client:       k8sClient,
						Scheme:       k8sClient.Scheme(),
						GitHubClient: githubClient,
					}
					_, reconcileErr := controllerReconciler.Reconcile(ctx, reconcile.Request{
						NamespacedName: typeNamespacedName,
					})
					Expect(reconcileErr).NotTo(HaveOccurred())

					By("Deleting the GithubIssue resource")
					Expect(k8sClient.Delete(ctx, existingGithubIssue)).To(Succeed())

					By("Reconciling after deletion")
					_, reconcileErr = controllerReconciler.Reconcile(ctx, reconcile.Request{
						NamespacedName: typeNamespacedName,
					})
					if !t.expectToSucceed {
						Expect(reconcileErr).ToNot(Succeed())
					} else {
						Expect(reconcileErr).To(Succeed())
					}
				})
			}
		})
	})
})
