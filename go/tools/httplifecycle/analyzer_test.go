package main

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	testdata := analysistest.TestData()
	const fixtures = "github.com/NlightN22/xray-p2p/go/tools/httplifecyclefixtures/"
	analysistest.Run(t, testdata, Analyzer, fixtures+"bad", fixtures+"dot", fixtures+"good")
}
