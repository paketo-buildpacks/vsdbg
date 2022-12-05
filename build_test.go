package vsdbg_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/chronos"
	"github.com/paketo-buildpacks/packit/v2/postal"
	"github.com/paketo-buildpacks/packit/v2/sbom"
	"github.com/paketo-buildpacks/packit/v2/scribe"
	vsdbg "github.com/paketo-buildpacks/vsdbg"
	"github.com/paketo-buildpacks/vsdbg/fakes"
	"github.com/sclevine/spec"

	. "github.com/onsi/gomega"
)

func testBuild(t *testing.T, context spec.G, it spec.S) {

	var (
		Expect = NewWithT(t).Expect

		layersDir  string
		cnbDir     string
		workingDir string

		sbomGenerator     *fakes.SBOMGenerator
		dependencyManager *fakes.DependencyManager
		logEmitter        scribe.Emitter

		buffer *bytes.Buffer

		build        packit.BuildFunc
		buildContext packit.BuildContext
	)

	it.Before(func() {
		var err error
		layersDir, err = os.MkdirTemp("", "layers")
		Expect(err).NotTo(HaveOccurred())

		cnbDir, err = os.MkdirTemp("", "cnb")
		Expect(err).NotTo(HaveOccurred())

		workingDir, err = os.MkdirTemp("", "working-dir")
		Expect(err).NotTo(HaveOccurred())

		dependency := postal.Dependency{
			ID:       "vsdbg",
			Name:     "vsdbg-dependency-name",
			Checksum: "sha256:vsdbg-dependency-sha",
			Stacks:   []string{"some-stack"},
			URI:      "vsdbg-dependency-uri",
			Version:  "vsdbg-dependency-version",
		}

		dependencyManager = &fakes.DependencyManager{}
		dependencyManager.ResolveCall.Returns.Dependency = dependency

		dependencyManager.DeliverCall.Stub = func(dependency postal.Dependency, cnbDir, targetLayerPath, platformPath string) error {
			err = os.MkdirAll(filepath.Join(layersDir, "vsdbg"), os.ModePerm)
			if err != nil {
				return fmt.Errorf("issue with stub call: %s", err)
			}

			vsdbgBinPath := filepath.Join(layersDir, "vsdbg", "vsdbg")
			err = os.WriteFile(vsdbgBinPath, []byte{}, 0600)
			if err != nil {
				return fmt.Errorf("issue with stub call: %s", err)
			}

			// vsdbg file has -rw-r--r-- permissions when extracted
			err = os.Chmod(vsdbgBinPath, 0644)
			if err != nil {
				return fmt.Errorf("issue with stub call: %s", err)
			}

			return nil
		}

		sbomGenerator = &fakes.SBOMGenerator{}
		sbomGenerator.GenerateCall.Returns.SBOM = sbom.SBOM{}

		buffer = bytes.NewBuffer(nil)
		logEmitter = scribe.NewEmitter(buffer)

		build = vsdbg.Build(
			dependencyManager,
			sbomGenerator,
			logEmitter,
			chronos.DefaultClock,
		)

		buildContext = packit.BuildContext{
			BuildpackInfo: packit.BuildpackInfo{
				Name:        "Some Buildpack",
				Version:     "some-version",
				SBOMFormats: []string{sbom.CycloneDXFormat, sbom.SPDXFormat},
			},
			WorkingDir: workingDir,
			CNBPath:    cnbDir,
			Plan: packit.BuildpackPlan{
				Entries: []packit.BuildpackPlanEntry{
					{Name: "vsdbg"},
				},
			},
			Platform: packit.Platform{Path: "platform"},
			Layers:   packit.Layers{Path: layersDir},
			Stack:    "some-stack",
		}
	})

	it.After(func() {
		Expect(os.RemoveAll(layersDir)).To(Succeed())
		Expect(os.RemoveAll(cnbDir)).To(Succeed())
		Expect(os.RemoveAll(workingDir)).To(Succeed())
	})

	it("returns a result that installs vsdbg", func() {
		result, err := build(buildContext)
		Expect(err).NotTo(HaveOccurred())

		Expect(result.Layers).To(HaveLen(1))
		layer := result.Layers[0]

		Expect(layer.Name).To(Equal("vsdbg"))

		Expect(layer.Path).To(Equal(filepath.Join(layersDir, "vsdbg")))

		Expect(layer.SharedEnv).To(HaveLen(2))
		Expect(layer.SharedEnv["PATH.delim"]).To(Equal(":"))
		Expect(layer.SharedEnv["PATH.append"]).To(Equal(filepath.Join(layersDir, "vsdbg")))

		Expect(layer.BuildEnv).To(BeEmpty())
		Expect(layer.LaunchEnv).To(BeEmpty())
		Expect(layer.ProcessLaunchEnv).To(BeEmpty())

		Expect(layer.Build).To(BeFalse())
		Expect(layer.Launch).To(BeFalse())
		Expect(layer.Cache).To(BeFalse())

		Expect(layer.Metadata).To(HaveLen(1))
		Expect(layer.Metadata["dependency-checksum"]).To(Equal("sha256:vsdbg-dependency-sha"))

		Expect(layer.SBOM.Formats()).To(Equal([]packit.SBOMFormat{
			{
				Extension: sbom.Format(sbom.CycloneDXFormat).Extension(),
				Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.CycloneDXFormat),
			},
			{
				Extension: sbom.Format(sbom.SPDXFormat).Extension(),
				Content:   sbom.NewFormattedReader(sbom.SBOM{}, sbom.SPDXFormat),
			},
		}))

		Expect(dependencyManager.ResolveCall.Receives.Path).To(Equal(filepath.Join(cnbDir, "buildpack.toml")))
		Expect(dependencyManager.ResolveCall.Receives.Id).To(Equal("vsdbg"))
		Expect(dependencyManager.ResolveCall.Receives.Version).To(Equal(""))
		Expect(dependencyManager.ResolveCall.Receives.Stack).To(Equal("some-stack"))

		Expect(dependencyManager.DeliverCall.Receives.Dependency).To(Equal(postal.Dependency{
			ID:       "vsdbg",
			Name:     "vsdbg-dependency-name",
			Checksum: "sha256:vsdbg-dependency-sha",
			Stacks:   []string{"some-stack"},
			URI:      "vsdbg-dependency-uri",
			Version:  "vsdbg-dependency-version",
		}))

		Expect(dependencyManager.DeliverCall.Receives.CnbPath).To(Equal(cnbDir))
		Expect(dependencyManager.DeliverCall.Receives.DestinationPath).To(Equal(filepath.Join(layersDir, "vsdbg")))
		Expect(dependencyManager.DeliverCall.Receives.PlatformPath).To(Equal("platform"))

		Expect(sbomGenerator.GenerateCall.Receives.Dir).To(Equal(filepath.Join(layersDir, "vsdbg")))

		Expect(buffer.String()).To(ContainSubstring("Some Buildpack some-version"))
		Expect(buffer.String()).To(ContainSubstring("Executing build process"))
		Expect(buffer.String()).To(ContainSubstring("Installing Visual Studio Debugger"))

		// test that the existing file permissions for vsdbg binary are preserved
		// with the addition of owner and group-execute permissions
		info, err := os.Stat(filepath.Join(layersDir, "vsdbg", "vsdbg"))
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Mode().String()).To(Equal("-rwxr-xr--"))
	})

	context("when build plan entries require vsdbg at build/launch", func() {
		it.Before(func() {
			buildContext.Plan.Entries[0].Metadata = make(map[string]interface{})
			buildContext.Plan.Entries[0].Metadata["build"] = true
			buildContext.Plan.Entries[0].Metadata["launch"] = true
		})

		it("makes the layer available at the right times", func() {
			result, err := build(buildContext)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Layers).To(HaveLen(1))
			layer := result.Layers[0]

			Expect(layer.Name).To(Equal("vsdbg"))
			Expect(layer.Path).To(Equal(filepath.Join(layersDir, "vsdbg")))
			Expect(layer.Metadata).To(Equal(map[string]interface{}{
				"dependency-checksum": "sha256:vsdbg-dependency-sha",
			}))

			Expect(layer.Build).To(BeTrue())
			Expect(layer.Launch).To(BeTrue())
			Expect(layer.Cache).To(BeTrue())
		})
	})

	context("when rebuilding a layer", func() {
		it.Before(func() {
			err := os.WriteFile(filepath.Join(layersDir, fmt.Sprintf("%s.toml", vsdbg.PlanDependencyVSDBG)), []byte(`[metadata]
dependency-checksum = "sha256:vsdbg-dependency-sha"
			`), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			buildContext.Plan.Entries[0].Metadata = make(map[string]interface{})
			buildContext.Plan.Entries[0].Metadata["build"] = true
			buildContext.Plan.Entries[0].Metadata["launch"] = false
		})

		it("skips the build process if the cached dependency sha matches the selected dependency sha", func() {
			result, err := build(buildContext)
			Expect(err).NotTo(HaveOccurred())

			Expect(result.Layers).To(HaveLen(1))
			layer := result.Layers[0]

			Expect(layer.Name).To(Equal("vsdbg"))

			Expect(layer.Build).To(BeTrue())
			Expect(layer.Launch).To(BeFalse())
			Expect(layer.Cache).To(BeTrue())

			Expect(buffer.String()).ToNot(ContainSubstring("Executing build process"))
			Expect(buffer.String()).To(ContainSubstring("Reusing cached layer"))

			Expect(dependencyManager.DeliverCall.CallCount).To(Equal(0))
		})
	})

	context("failure cases", func() {
		context("when dependency resolution fails", func() {
			it.Before(func() {
				dependencyManager.ResolveCall.Returns.Error = errors.New("failed to resolve dependency")
			})
			it("returns an error", func() {
				_, err := build(buildContext)

				Expect(err).To(MatchError(ContainSubstring("failed to resolve dependency")))
			})
		})

		context("when vsdbg layer cannot be fetched", func() {
			it.Before(func() {
				Expect(os.Chmod(layersDir, 0000)).To(Succeed())
			})

			it.After(func() {
				Expect(os.Chmod(layersDir, os.ModePerm)).To(Succeed())
			})

			it("returns an error", func() {
				_, err := build(buildContext)

				Expect(err).To(MatchError(ContainSubstring("permission denied")))
			})
		})

		context("when vsdbg layer cannot be reset", func() {
			it.Before(func() {
				Expect(os.MkdirAll(filepath.Join(layersDir, vsdbg.PlanDependencyVSDBG), os.ModePerm))
				Expect(os.Chmod(layersDir, 0500)).To(Succeed())
			})

			it.After(func() {
				Expect(os.Chmod(layersDir, os.ModePerm)).To(Succeed())
			})

			it("returns an error", func() {
				_, err := build(buildContext)

				Expect(err).To(MatchError(ContainSubstring("permission denied")))
			})
		})

		context("when dependency cannot be installed", func() {
			it.Before(func() {
				dependencyManager.DeliverCall.Stub = func(dependency postal.Dependency, cnbDir, targetLayerPath, platformPath string) error {
					return errors.New("failed to install dependency")
				}
			})
			it("returns an error", func() {
				_, err := build(buildContext)

				Expect(err).To(MatchError(ContainSubstring("failed to install dependency")))
			})
		})

		context("when dependency cannot be installed", func() {
			it.Before(func() {
				dependencyManager.DeliverCall.Stub = nil
			})
			it("returns an error", func() {
				_, err := build(buildContext)

				Expect(err).To(MatchError(ContainSubstring("no such file or directory")))
			})
		})

		context("when generating the SBOM returns an error", func() {
			it.Before(func() {
				buildContext.BuildpackInfo.SBOMFormats = []string{"random-format"}
			})

			it("returns an error", func() {
				_, err := build(buildContext)
				Expect(err).To(MatchError(`unsupported SBOM format: 'random-format'`))
			})
		})

		context("when formatting the sbom returns an error", func() {
			it.Before(func() {
				sbomGenerator.GenerateCall.Returns.Error = errors.New("failed to generate sbom")
			})

			it("returns an error", func() {
				_, err := build(buildContext)
				Expect(err).To(MatchError(ContainSubstring("failed to generate sbom")))
			})
		})

	})
}
