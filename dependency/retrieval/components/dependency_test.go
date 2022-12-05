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

	context("ConvertReleaseToDependeny", func() {
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

				case "/bad-archive":
					w.WriteHeader(http.StatusOK)
					_, err := w.Write([]byte("\x66\x4C\x61\x43\x00\x00\x00\x22"))
					Expect(err).NotTo(HaveOccurred())

				default:
					t.Fatalf("unknown path: %s", req.URL.Path)
				}
			}))

		})

		it("returns returns a cargo dependency generated from the given release", func() {
			dependency, err := components.ConvertReleaseToDependency(components.Release{
				SemVer:  semver.MustParse("17.4.11017-1"),
				Version: "17.4.11017.1",
				URL:     server.URL,
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(dependency).To(Equal(cargo.ConfigMetadataDependency{
				Checksum:       "sha256:5a95bcffa592dcc7689ef5b4d993da3ca805b3c58d1710da8effeedbda87d471",
				CPE:            "cpe:2.3:a:microsoft:vsdbg:17.4.11017.1:*:*:*:*:*:*:*",
				PURL:           fmt.Sprintf("pkg:generic/vsdbg@17.4.11017.1?checksum=5a95bcffa592dcc7689ef5b4d993da3ca805b3c58d1710da8effeedbda87d471&download_url=%s", server.URL),
				ID:             "vsdbg",
				Licenses:       []interface{}{"MIT", "MIT-0"},
				Name:           "Visual Studio Debugger",
				SHA256:         "",
				Source:         server.URL,
				SourceChecksum: "sha256:5a95bcffa592dcc7689ef5b4d993da3ca805b3c58d1710da8effeedbda87d471",
				SourceSHA256:   "",
				Stacks: []string{
					"io.buildpacks.stacks.bionic",
					"io.buildpacks.stacks.jammy",
				},
				StripComponents: 0,
				URI:             server.URL,
				Version:         "17.4.11017-1",
			}))
		})

		context("failure cases", func() {
			context("when the release get fails", func() {
				it("returns an error", func() {
					_, err := components.ConvertReleaseToDependency(components.Release{
						SemVer:  semver.MustParse("17.4.11017-1"),
						Version: "17.4.11017.1",
						URL:     "not a valid url",
					})
					Expect(err).To(MatchError(ContainSubstring("unsupported protocol scheme")))
				})
			})

			context("when the release get is a non 200", func() {
				it("returns an error", func() {
					_, err := components.ConvertReleaseToDependency(components.Release{
						SemVer:  semver.MustParse("17.4.11017-1"),
						Version: "17.4.11017.1",
						URL:     fmt.Sprintf("%s/non-200", server.URL),
					})
					Expect(err).To(MatchError(fmt.Sprintf("received a non 200 status code from %s: status code 418 received", fmt.Sprintf("%s/non-200", server.URL))))
				})
			})

			context("when the artifact is not a supported archive type", func() {
				it("returns an error", func() {
					_, err := components.ConvertReleaseToDependency(components.Release{
						SemVer:  semver.MustParse("17.4.11017-1"),
						Version: "17.4.11017.1",
						URL:     fmt.Sprintf("%s/bad-archive", server.URL),
					})
					Expect(err).To(MatchError(ContainSubstring("unsupported archive type")))
				})
			})
		})
	})
}
