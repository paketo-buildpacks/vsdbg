package components_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/paketo-buildpacks/vsdbg/dependency/retrieval/components"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testReleases(t *testing.T, context spec.G, it spec.S) {

	var (
		Expect = NewWithT(t).Expect
	)

	context("Fetcher", func() {
		var (
			fetcher components.Fetcher

			server *httptest.Server
		)

		it.Before(func() {
			server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if req.Method == http.MethodHead {
					http.Error(w, "NotFound", http.StatusNotFound)
					return
				}

				switch req.URL.Path {
				case "/":
					w.WriteHeader(http.StatusOK)
					contents, err := os.ReadFile(filepath.Join("testdata", "GetVsDbg.sh"))
					Expect(err).NotTo(HaveOccurred())

					_, err = w.Write(contents)
					Expect(err).NotTo(HaveOccurred())

				case "/non-200":
					w.WriteHeader(http.StatusTeapot)

				case "/no-function":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `{
    version_string="$(echo "$1" | awk '{print tolower($0)}')"
    case "$version_string" in
        latest)
            __VsDbgVersion=17.4.11017.1
					`)

				case "/no-latest-version":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `set_vsdbg_version()
{
    version_string="$(echo "$1" | awk '{print tolower($0)}')"
    case "$version_string" in
        oldest)
            __VsDbgVersion=17.4.11017.1
					`)

				case "/wrong-version-format":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `set_vsdbg_version()
{
    version_string="$(echo "$1" | awk '{print tolower($0)}')"
    case "$version_string" in
        latest)
            __VsDbgVersion=wrong format
					`)

				case "/no-version-parse":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintln(w, `set_vsdbg_version()
{
    version_string="$(echo "$1" | awk '{print tolower($0)}')"
    case "$version_string" in
        latest)
            __VsDbgVersion=not.valid.semver.version
					`)

				default:
					t.Fatalf("unknown path: %s", req.URL.Path)
				}
			}))

			fetcher = components.NewFetcher().WithScriptURL(server.URL)
		})

		it("fetches a list of relevant releases", func() {
			releases, err := fetcher.Get()
			Expect(err).NotTo(HaveOccurred())

			Expect(releases).To(Equal([]components.Release{
				{
					SemVer:  semver.MustParse("17.4.11017+1"),
					Version: "17.4.11017.1",
					URL:     "https://vsdebugger-cyg0dxb6czfafzaz.b01.azurefd.net/vsdbg-17-4-11017-1/vsdbg-linux-x64.tar.gz",
				},
			}))
		})

		context("failure cases", func() {
			context("when the script get fails", func() {
				it.Before(func() {
					fetcher = fetcher.WithScriptURL("not a valid URL")
				})

				it("returns an error", func() {
					_, err := fetcher.Get()
					Expect(err).To(MatchError(ContainSubstring("unsupported protocol scheme")))
				})
			})

			context("when the script get returns non 200 code", func() {
				it.Before(func() {
					fetcher = fetcher.WithScriptURL(fmt.Sprintf("%s/non-200", server.URL))
				})

				it("returns an error", func() {
					_, err := fetcher.Get()
					Expect(err).To(MatchError(fmt.Sprintf("received a non 200 status code from %s: status code 418 received", fmt.Sprintf("%s/non-200", server.URL))))
				})
			})

			context("when there is no set_vsdbg_version function", func() {
				it.Before(func() {
					fetcher = fetcher.WithScriptURL(fmt.Sprintf("%s/no-function", server.URL))
				})

				it("returns an error", func() {
					_, err := fetcher.Get()
					Expect(err).To(MatchError(`set_vsdbg_version() function not found`))
				})
			})

			context("when there is no latest version", func() {
				it.Before(func() {
					fetcher = fetcher.WithScriptURL(fmt.Sprintf("%s/no-latest-version", server.URL))
				})

				it("returns an error", func() {
					_, err := fetcher.Get()
					Expect(err).To(MatchError(`latest version not found`))
				})
			})

			context("when the version is not w.x.y.z format", func() {
				it.Before(func() {
					fetcher = fetcher.WithScriptURL(fmt.Sprintf("%s/wrong-version-format", server.URL))
				})

				it("returns an error", func() {
					_, err := fetcher.Get()
					Expect(err).To(MatchError(`unexpect version: expected "wrong format" to be in the format of w.x.y.z`))
				})
			})

			context("when the version cannot be parsed", func() {
				it.Before(func() {
					fetcher = fetcher.WithScriptURL(fmt.Sprintf("%s/no-version-parse", server.URL))
				})

				it("returns an error", func() {
					_, err := fetcher.Get()
					Expect(err).To(MatchError(ContainSubstring("Invalid Semantic Version")))
				})
			})
		})
	})
}
