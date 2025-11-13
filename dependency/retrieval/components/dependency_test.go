package components_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/paketo-buildpacks/libdependency/retrieve"
	"github.com/paketo-buildpacks/libdependency/versionology"
	"github.com/paketo-buildpacks/packit/v2/cargo"
	"github.com/paketo-buildpacks/vsdbg/dependency/retrieval/components"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

const (
	lFile = `The MIT License (MIT)

Copyright (c) .NET Foundation and Contributors

All rights reserved.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
`
)

func testDependency(t *testing.T, context spec.G, it spec.S) {

	var (
		Expect = NewWithT(t).Expect
	)

	context("GenerateMetadata", func() {
		var (
			server *httptest.Server
		)

		it.Before(func() {
			buffer := bytes.NewBuffer(nil)
			gw := gzip.NewWriter(buffer)
			tw := tar.NewWriter(gw)

			licenseFile := "./LICENSE.txt"
			Expect(tw.WriteHeader(&tar.Header{Name: licenseFile, Mode: 0755, Size: int64(len(lFile))})).To(Succeed())
			_, err := tw.Write([]byte(lFile))
			Expect(err).NotTo(HaveOccurred())

			Expect(tw.Close()).To(Succeed())
			Expect(gw.Close()).To(Succeed())

			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method == http.MethodHead {
					http.Error(w, "NotFound", http.StatusNotFound)
					return
				}

				switch req.URL.Path {
				case "/":
					w.WriteHeader(http.StatusOK)
					_, err := w.Write(buffer.Bytes())
					Expect(err).NotTo(HaveOccurred())

				case "/non-200":
					w.WriteHeader(http.StatusTeapot)

				default:
					t.Fatalf("unknown path: %s", req.URL.Path)
				}
			}))
		})

		it("returns returns a cargo dependency generated from the given release", func() {
			generator := components.NewGenerator().WithFakeUrl(server.URL)
			dependencies, err := generator.GenerateMetadata(components.VsdbgRelease{
				SemVer:         semver.MustParse("17.4.11017-1"),
				ReleaseVersion: "17.4.11017.1",
				SplitVersion:   []string{"17", "4", "11017", "1"},
			}, retrieve.Platform{OS: "linux", Arch: "amd64"})
			Expect(err).NotTo(HaveOccurred())

			Expect(dependencies).To(HaveLen(1))
			dependency := dependencies[0]

			Expect(dependency).To(BeEquivalentTo(
				versionology.Dependency{
					ConfigMetadataDependency: cargo.ConfigMetadataDependency{
						Checksum:        "sha256:5a95bcffa592dcc7689ef5b4d993da3ca805b3c58d1710da8effeedbda87d471",
						CPE:             "cpe:2.3:a:microsoft:vsdbg:17.4.11017.1:*:*:*:*:*:*:*",
						PURL:            fmt.Sprintf("pkg:generic/vsdbg@17.4.11017.1?checksum=5a95bcffa592dcc7689ef5b4d993da3ca805b3c58d1710da8effeedbda87d471&download_url=%s", server.URL),
						ID:              "vsdbg",
						Licenses:        nil,
						Name:            "Visual Studio Debugger",
						SHA256:          "",
						Source:          server.URL,
						SourceChecksum:  "sha256:5a95bcffa592dcc7689ef5b4d993da3ca805b3c58d1710da8effeedbda87d471",
						SourceSHA256:    "",
						StripComponents: 0,
						URI:             server.URL,
						Version:         "17.4.11017-1",
						OS:              "linux",
						Arch:            "amd64",
						Stacks:          []string{"*"},
					},
					SemverVersion: semver.MustParse("17.4.11017-1"),
					Target:        "*",
				}))
		})

		context("failure cases", func() {
			context("when the release get fails", func() {
				it("returns an error", func() {
					generator := components.NewGenerator().WithFakeUrl("not a valid url")
					_, err := generator.GenerateMetadata(components.VsdbgRelease{
						SemVer:         semver.MustParse("17.4.11017-1"),
						ReleaseVersion: "17.4.11017.1",
						SplitVersion:   []string{"17", "4", "11017", "1"},
					}, retrieve.Platform{OS: "linux", Arch: "amd64"})
					Expect(err).To(MatchError(ContainSubstring("unsupported protocol scheme")))
				})
			})

			context("when the release get is a non 200", func() {
				it("returns an error", func() {
					generator := components.NewGenerator().WithFakeUrl(fmt.Sprintf("%s/non-200", server.URL))
					_, err := generator.GenerateMetadata(components.VsdbgRelease{
						SemVer:         semver.MustParse("17.4.11017-1"),
						ReleaseVersion: "17.4.11017.1",
						SplitVersion:   []string{"17", "4", "11017", "1"},
					}, retrieve.Platform{OS: "linux", Arch: "amd64"})
					Expect(err).To(MatchError(fmt.Sprintf("received a non 200 status code from %s: status code 418 received", fmt.Sprintf("%s/non-200", server.URL))))
				})
			})
		})
	})
}
