package components

import (
	"bufio"
	"fmt"
	"net/http"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/paketo-buildpacks/libdependency/versionology"
)

type VsdbgRelease struct {
	SemVer         *semver.Version
	ReleaseVersion string `json:"version"`
	SplitVersion   []string
}

type Fetcher struct {
	scriptURL string
}

func (r VsdbgRelease) Version() *semver.Version {
	return r.SemVer
}

func NewFetcher() Fetcher {
	return Fetcher{
		scriptURL: "https://aka.ms/getvsdbgsh",
	}
}

func (f Fetcher) WithScriptURL(url string) Fetcher {
	f.scriptURL = url
	return f
}

func (f Fetcher) GetVersions() (versionology.VersionFetcherArray, error) {
	response, err := http.Get(f.scriptURL)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if !(response.StatusCode >= 200 && response.StatusCode < 300) {
		return nil, fmt.Errorf("received a non 200 status code from %s: status code %d received", f.scriptURL, response.StatusCode)
	}

	scanner := bufio.NewScanner(response.Body)

	var version string
	var inFunction, latest bool
	for scanner.Scan() {
		if inFunction && latest {
			version = strings.Split(strings.TrimSpace(scanner.Text()), "=")[1]
			break
		}

		if inFunction && strings.Contains(scanner.Text(), "latest)") {
			latest = true
			continue
		}

		if strings.Contains(scanner.Text(), "set_vsdbg_version()") {
			inFunction = true
			continue
		}
	}

	if !inFunction {
		return nil, fmt.Errorf("set_vsdbg_version() function not found")
	}

	if !latest {
		return nil, fmt.Errorf("latest version not found")
	}

	var release VsdbgRelease

	release.ReleaseVersion = version
	release.SplitVersion = strings.Split(version, ".")
	if len(release.SplitVersion) != 4 {
		return nil, fmt.Errorf("unexpect version: expected %q to be in the format of w.x.y.z", version)
	}

	release.SemVer, err = semver.NewVersion(fmt.Sprintf("%s+%s", strings.Join(release.SplitVersion[:3], "."), release.SplitVersion[3]))
	if err != nil {
		return nil, fmt.Errorf("%w: the following version string could not be parsed %q", err, release.ReleaseVersion)
	}

	return []versionology.VersionFetcher{release}, nil
}
