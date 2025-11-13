package components

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/paketo-buildpacks/libdependency/retrieve"
	"github.com/paketo-buildpacks/libdependency/versionology"
	"github.com/paketo-buildpacks/packit/v2/cargo"
)

type Generator struct {
	UrlFormatter func(version string, os string, arch string) string
}

func NewGenerator() Generator {
	return Generator{
		UrlFormatter: func(version string, os string, arch string) string {
			return fmt.Sprintf("https://vsdebugger-cyg0dxb6czfafzaz.b01.azurefd.net/vsdbg-%s/vsdbg-%s-%s.tar.gz", version, os, arch)
		},
	}
}

func (g Generator) WithFakeUrl(url string) Generator {
	g.UrlFormatter = func(version string, os string, arch string) string {
		return url
	}
	return g
}

func (g Generator) GenerateMetadata(version versionology.VersionFetcher, platform retrieve.Platform) ([]versionology.Dependency, error) {
	vsdbgRelease := version.(VsdbgRelease)

	arch := platform.Arch
	if platform.Arch == "amd64" {
		arch = "x64"
	}

	url := g.UrlFormatter(strings.Join(vsdbgRelease.SplitVersion, "-"), platform.OS, arch)

	response, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if !(response.StatusCode >= 200 && response.StatusCode < 300) {
		return nil, fmt.Errorf("received a non 200 status code from %s: status code %d received", url, response.StatusCode)
	}

	hasher := sha256.New()
	if _, err := io.Copy(hasher, response.Body); err != nil {
		return nil, err
	}

	cpe := fmt.Sprintf("cpe:2.3:a:microsoft:vsdbg:%s:*:*:*:*:*:*:*", vsdbgRelease.ReleaseVersion)
	hash := fmt.Sprintf("%x", hasher.Sum(nil))
	purl := retrieve.GeneratePURL("vsdbg", vsdbgRelease.ReleaseVersion, hash, url)

	metadataDependency := cargo.ConfigMetadataDependency{
		ID:             "vsdbg",
		Name:           "Visual Studio Debugger",
		Version:        vsdbgRelease.SemVer.String(),
		Stacks:         []string{"*"},
		URI:            url,
		Checksum:       fmt.Sprintf("sha256:%s", hash),
		Source:         url,
		SourceChecksum: fmt.Sprintf("sha256:%s", hash),
		CPE:            cpe,
		PURL:           purl,
		Licenses:       nil, // VSDBG does not use a standard open source license
		OS:             platform.OS,
		Arch:           platform.Arch,
	}

	dependency, err := versionology.NewDependency(metadataDependency, "*")
	if err != nil {
		return nil, err
	}

	return []versionology.Dependency{dependency}, nil
}
