package vsdbg_test

import (
	"testing"

	"github.com/sclevine/spec"
	"github.com/sclevine/spec/report"
)

func TestUnitVSDBG(t *testing.T) {
	suite := spec.New("vsdbg", spec.Report(report.Terminal{}))
	suite("Detect", testDetect)
	suite("Build", testBuild)
	suite.Run(t)
}
