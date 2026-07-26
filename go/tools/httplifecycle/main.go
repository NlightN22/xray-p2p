package main

import (
	"github.com/timakin/bodyclose/passes/bodyclose"
	"golang.org/x/tools/go/analysis/multichecker"
)

func main() {
	multichecker.Main(Analyzer, bodyclose.Analyzer)
}
