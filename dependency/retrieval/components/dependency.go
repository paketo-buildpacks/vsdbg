package components

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/paketo-buildpacks/packit/v2/cargo"
)

func ConvertReleaseToDependency(release Release) (cargo.ConfigMetadataDependency, error) {
	response, err := http.Get(release.URL)
	if err != nil {
		return cargo.ConfigMetadataDependency{}, err
	}
	defer response.Body.Close()

	if !(response.StatusCode >= 200 && response.StatusCode < 300) {
		return cargo.ConfigMetadataDependency{}, fmt.Errorf("received a non 200 status code from %s: status code %d received", release.URL, response.StatusCode)
	}

	hasher := sha256.New()
	if _, err := io.Copy(hasher, response.Body); err != nil {
		return cargo.ConfigMetadataDependency{}, err
	}

	hash := fmt.Sprintf("sha256:%x", hasher.Sum(nil))

	purl := GeneratePURL("vsdbg", release.Version, strings.TrimPrefix(hash, "sha256:"), release.URL)

	return cargo.ConfigMetadataDependency{
		ID:      "vsdbg",
		Name:    "Visual Studio Debugger",
		Version: release.SemVer.String(),
		Stacks: []string{
			"*",
		},
		URI:            release.URL,
		Checksum:       hash,
		Source:         release.URL,
		SourceChecksum: hash,
		CPE:            fmt.Sprintf("cpe:2.3:a:microsoft:vsdbg:%s:*:*:*:*:*:*:*", release.Version),
		PURL:           purl,
		Licenses:       nil, // VSDBG does not use a standard open source license
	}, nil
}
