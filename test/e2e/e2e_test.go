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

package e2e

import (
	"context"
	"fmt"
	"time"

	"os/exec"

	danaiov1alpha1 "github.com/TalDebi/GithubIssue/api/v1alpha1"
	"github.com/TalDebi/GithubIssue/internal/controller"
	"github.com/TalDebi/GithubIssue/test/utils"
	"github.com/google/go-github/v50/github"
	"github.com/migueleliasweb/go-github-mock/src/mock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// namespace where the project is deployed in
const namespace = "githubissue-system"

var _ = Describe("Manager", Ordered, func() {
	var controllerPodName string

	// Before running the tests, set up the environment by creating the namespace,
	// installing CRDs, and deploying the controller.
	BeforeAll(func() {
		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("installing CRDs")
		cmd = exec.Command("make", "install")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

		By("deploying the controller-manager")
		cmd = exec.Command("make", "deploy", fmt.Sprintf("IMG=%s", projectImage))
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")
	})

	// After each test, check for failures and collect logs, events,
	// and pod descriptions for debugging.
	AfterEach(func() {
		specReport := CurrentSpecReport()
		if specReport.Failed() {
			By("Fetching controller manager pod logs")
			cmd := exec.Command("kubectl", "logs", controllerPodName, "-n", namespace)
			controllerLogs, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Controller logs:\n %s", controllerLogs)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Controller logs: %s", err)
			}

			By("Fetching Kubernetes events")
			cmd = exec.Command("kubectl", "get", "events", "-n", namespace, "--sort-by=.lastTimestamp")
			eventsOutput, err := utils.Run(cmd)
			if err == nil {
				_, _ = fmt.Fprintf(GinkgoWriter, "Kubernetes events:\n%s", eventsOutput)
			} else {
				_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get Kubernetes events: %s", err)
			}

			By("Fetching controller manager pod description")
			cmd = exec.Command("kubectl", "describe", "pod", controllerPodName, "-n", namespace)
			podDescription, err := utils.Run(cmd)
			if err == nil {
				fmt.Println("Pod description:\n", podDescription)
			} else {
				fmt.Println("Failed to describe controller pod")
			}
		}
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

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
		It("should fail to create an issue on github if repo is not valid", func() {
			By("Creating the GithubIssue")
			invalidGithubIssue := &danaiov1alpha1.Issue{
				ObjectMeta: metav1.ObjectMeta{
					Name:      typeNamespacedName.Name,
					Namespace: typeNamespacedName.Namespace,
				},
				Spec: danaiov1alpha1.IssueSpec{
					Repo:        invalidRepo,
					Title:       testTitle,
					Description: testDescription,
				},
			}

			Expect(k8sClient.Create(ctx, invalidGithubIssue)).ToNot(Succeed())
		})
		It("should fail to update an issue on github if repo is not valid", func() {
			By("Creating the GithubIssue")
			existingGithubIssue := &danaiov1alpha1.Issue{
				ObjectMeta: metav1.ObjectMeta{
					Name:      typeNamespacedName.Name,
					Namespace: typeNamespacedName.Namespace,
				},
				Spec: danaiov1alpha1.IssueSpec{
					Repo:        testRepository,
					Title:       testTitle,
					Description: testDescription,
				},
			}
			Expect(k8sClient.Create(ctx, existingGithubIssue)).To(Succeed())

			By("Reconciling the created resource")
			controllerReconciler := &controller.IssueReconciler{
				Client:       k8sClient,
				Scheme:       k8sClient.Scheme(),
				GitHubClient: githubClient,
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Updating the GithubIssue")
			existingGithubIssue.Spec.Repo = invalidRepo
			Expect(k8sClient.Update(ctx, existingGithubIssue)).ToNot(Succeed())
		})
		It("should succeed to create an issue on github if repo is valid", func() {
			By("Creating the GithubIssue")
			existingGithubIssue := &danaiov1alpha1.Issue{
				ObjectMeta: metav1.ObjectMeta{
					Name:      typeNamespacedName.Name,
					Namespace: typeNamespacedName.Namespace,
				},
				Spec: danaiov1alpha1.IssueSpec{
					Repo:        testRepository,
					Title:       testTitle,
					Description: testDescription,
				},
			}
			Expect(k8sClient.Create(ctx, existingGithubIssue)).To(Succeed())

			By("Reconciling the created resource")
			controllerReconciler := &controller.IssueReconciler{
				Client:       k8sClient,
				Scheme:       k8sClient.Scheme(),
				GitHubClient: githubClient,
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
		It("should close issue on github when GithubIssue is deleted", func() {
			By("Creating the GithubIssue")
			existingGithubIssue := &danaiov1alpha1.Issue{
				ObjectMeta: metav1.ObjectMeta{
					Name:      typeNamespacedName.Name,
					Namespace: typeNamespacedName.Namespace,
				},
				Spec: danaiov1alpha1.IssueSpec{
					Repo:        testRepository,
					Title:       testTitle,
					Description: testDescription,
				},
			}
			Expect(k8sClient.Create(ctx, existingGithubIssue)).To(Succeed())

			By("Reconciling the created resource")
			controllerReconciler := &controller.IssueReconciler{
				Client:       k8sClient,
				Scheme:       k8sClient.Scheme(),
				GitHubClient: githubClient,
			}
			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("Deleting the GithubIssue resource")
			Expect(k8sClient.Delete(ctx, existingGithubIssue)).To(Succeed())

			By("Reconciling after deletion")
			_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
