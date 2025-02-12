package common

import (
	"os"
	"time"
)

const FinalizerName = "githubissue.finalizers.dana.io/finalizer"
const ResyncPeriod = time.Minute
const GithubClientTimeout = 10 * time.Second
const GithubUriHost = "github.com"

var GithubRepoToken = os.Getenv("GITHUB_TOKEN")
