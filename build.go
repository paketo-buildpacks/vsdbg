package vsdbg

import (
	"os"
	"path/filepath"
	"time"

	"github.com/paketo-buildpacks/packit/v2"
	"github.com/paketo-buildpacks/packit/v2/chronos"
	"github.com/paketo-buildpacks/packit/v2/draft"
	"github.com/paketo-buildpacks/packit/v2/postal"
	"github.com/paketo-buildpacks/packit/v2/sbom"
	"github.com/paketo-buildpacks/packit/v2/scribe"
)

//go:generate faux --interface SBOMGenerator --output fakes/sbom_generator.go
//go:generate faux --interface DependencyManager --output fakes/dependency_manager.go

type SBOMGenerator interface {
	Generate(dir string) (sbom.SBOM, error)
}

// DependencyManager defines the interface for picking the best matching
// dependency and installing it.
type DependencyManager interface {
	Resolve(path, id, version, stack string) (postal.Dependency, error)
	Deliver(dependency postal.Dependency, cnbPath, destinationPath, platformPath string) error
}

func Build(
	dependencyManager DependencyManager,
	sbomGenerator SBOMGenerator,
	logger scribe.Emitter,
	clock chronos.Clock,
) packit.BuildFunc {
	return func(context packit.BuildContext) (packit.BuildResult, error) {
		logger.Title("%s %s", context.BuildpackInfo.Name, context.BuildpackInfo.Version)

		planner := draft.NewPlanner()

		logger.Process("Resolving Visual Studio Debugger version")
		entry, sortedEntries := planner.Resolve(PlanDependencyVSDBG, context.Plan.Entries, nil)
		logger.Candidates(sortedEntries)

		version, _ := entry.Metadata["version"].(string)
		dependency, err := dependencyManager.Resolve(filepath.Join(context.CNBPath, "buildpack.toml"), entry.Name, version, context.Stack)
		if err != nil {
			return packit.BuildResult{}, err
		}

		logger.SelectedDependency(entry, dependency, clock.Now())

		launch, build := planner.MergeLayerTypes(PlanDependencyVSDBG, context.Plan.Entries)

		layer, err := context.Layers.Get(PlanDependencyVSDBG)
		if err != nil {
			return packit.BuildResult{}, err
		}

		cachedSHA, ok := layer.Metadata[DependencySHAKey].(string)
		if ok && cachedSHA == dependency.SHA256 {
			logger.Process("Reusing cached layer %s", layer.Path)
			layer.Launch, layer.Build, layer.Cache = launch, build, build

			return packit.BuildResult{
				Layers: []packit.Layer{layer},
			}, nil
		}

		layer, err = layer.Reset()
		if err != nil {
			return packit.BuildResult{}, err
		}

		layer.Launch, layer.Build, layer.Cache = launch, build, build

		logger.Process("Executing build process")
		logger.Subprocess("Installing Visual Studio Debugger %s", dependency.Version)

		duration, err := clock.Measure(func() error {
			return dependencyManager.Deliver(dependency, context.CNBPath, layer.Path, context.Platform.Path)
		})
		if err != nil {
			return packit.BuildResult{}, err
		}

		logger.Action("Completed in %s", duration.Round(time.Millisecond))
		logger.Break()

		vsdbgBinPath := filepath.Join(layer.Path, "vsdbg")
		info, err := os.Stat(vsdbgBinPath)
		if err != nil {
			return packit.BuildResult{}, err
		}

		err = os.Chmod(vsdbgBinPath, info.Mode()|0110)
		if err != nil {
			// not tested
			return packit.BuildResult{}, err
		}

		logger.GeneratingSBOM(layer.Path)
		var sbomContent sbom.SBOM
		duration, err = clock.Measure(func() error {
			sbomContent, err = sbomGenerator.Generate(layer.Path)
			return err
		})
		if err != nil {
			return packit.BuildResult{}, err
		}

		logger.Action("Completed in %s", duration.Round(time.Millisecond))
		logger.Break()

		logger.FormattingSBOM(context.BuildpackInfo.SBOMFormats...)
		layer.SBOM, err = sbomContent.InFormats(context.BuildpackInfo.SBOMFormats...)
		if err != nil {
			return packit.BuildResult{}, err
		}

		layer.SharedEnv.Append("PATH", layer.Path, ":")
		logger.EnvironmentVariables(layer)

		layer.Metadata = map[string]interface{}{
			DependencySHAKey: dependency.SHA256,
		}

		return packit.BuildResult{
			Layers: []packit.Layer{layer},
		}, nil
	}
}
