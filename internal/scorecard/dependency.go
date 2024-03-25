package scorecard

import (
	"fmt"
	"net/url"
	"strings"
	"sync"

	"golang.org/x/tools/go/vcs"
)

var cache sync.Map

type Dependency struct {
	// SHA1 commit hash expressed in hexadecimal format (optional)
	Commit string
	// VCS platform. eg. github.com
	Platform string
	// Name of the owner/organization of the repository
	Org string
	// Name of the repository
	Repo string
	// Root is the root package name
	Root string
}

func (d Dependency) String() string {
	return fmt.Sprintf("%s/%s/%s", d.Platform, d.Org, d.Repo)
}

func ParseDependencyForImportPath(importPath string) (Dependency, error) {
	type fn = func() (Dependency, error)

	getter, _ := cache.LoadOrStore(importPath, sync.OnceValues(func() (Dependency, error) {
		repoRoot, err := vcs.RepoRootForImportPath(importPath, false)
		if err != nil {
			return Dependency{}, err
		}

		repoRootURL, err := url.Parse(repoRoot.Repo)
		if err != nil {
			return Dependency{}, err
		}

		// Special case for gopkg.in, which obscures the source url in a way that
		// RepoRootForImportPath does not follow.
		if strings.Contains(repoRootURL.Host, "gopkg.in") {
			// Remove the version from the path
			path := strings.Trim(repoRootURL.Path, "/")
			path, _, _ = strings.Cut(path, ".")

			// If the path only contains one segment the real github path
			// is go-<name>/<name>
			if !strings.Contains(strings.Trim(repoRootURL.Path, "/"), "/") {
				path = "go-" + path + "/" + path
			}

			// Update the URL to github
			repoRootURL.Host = "github.com"
			repoRootURL.Path = path
		}

		org, repo, _ := strings.Cut(strings.Trim(repoRootURL.Path, "/"), "/")

		return Dependency{
			Platform: repoRootURL.Host,
			Org:      org,
			Repo:     repo,
			Root:     repoRoot.Root,
		}, nil
	}))

	return getter.(fn)()
}
