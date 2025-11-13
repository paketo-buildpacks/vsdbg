package main

import (
	"github.com/paketo-buildpacks/libdependency/retrieve"
	"github.com/paketo-buildpacks/vsdbg/dependency/retrieval/components"
)

func main() {
	fetcher := components.NewFetcher()
	generator := components.NewGenerator()
	retrieve.NewMetadataWithPlatforms("vsdbg", fetcher.GetVersions, generator.GenerateMetadata)
}
