package main

import (
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/buildssa"
	"golang.org/x/tools/go/ssa"
)

type ownershipAnalysis struct {
	pass       *analysis.Pass
	files      []*fileAllowances
	resultMemo map[functionResult]bool
	visiting   map[functionResult]bool
	reported   map[token.Pos]bool
	inspected  map[*ssa.Function]bool
}

type functionResult struct {
	function *ssa.Function
	index    int
}

func inspectFactoryOwnership(pass *analysis.Pass, files []*fileAllowances) {
	ssaResult := pass.ResultOf[buildssa.Analyzer].(*buildssa.SSA)
	analysis := ownershipAnalysis{
		pass:       pass,
		files:      files,
		resultMemo: make(map[functionResult]bool),
		visiting:   make(map[functionResult]bool),
		reported:   make(map[token.Pos]bool),
		inspected:  make(map[*ssa.Function]bool),
	}
	for _, function := range ssaResult.SrcFuncs {
		analysis.inspectFunctionTree(function)
	}
	for _, member := range ssaResult.Pkg.Members {
		if function, ok := member.(*ssa.Function); ok {
			analysis.inspectFunctionTree(function)
		}
	}
}

func (a *ownershipAnalysis) inspectFunctionTree(function *ssa.Function) {
	if function == nil || a.inspected[function] {
		return
	}
	a.inspected[function] = true
	if function.Signature != nil && function.Signature.Results().Len() > 0 &&
		!strings.HasSuffix(a.pass.Fset.Position(function.Pos()).Filename, "_test.go") {
		a.inspectReturns(function)
	}
	for _, nested := range function.AnonFuncs {
		a.inspectFunctionTree(nested)
	}
}

func (a *ownershipAnalysis) inspectReturns(function *ssa.Function) {
	for _, block := range function.Blocks {
		for _, instruction := range block.Instrs {
			returned, ok := instruction.(*ssa.Return)
			if !ok || len(returned.Results) != function.Signature.Results().Len() {
				continue
			}
			for index, value := range returned.Results {
				if isOwnedHTTPClient(function.Signature.Results().At(index).Type()) {
					continue
				}
				for _, origin := range a.creationOrigins(value, make(map[ssa.Value]bool)) {
					if origin == token.NoPos || a.reported[origin] ||
						consumeAllowance(a.pass.Fset, a.files, origin, "owned-client-result") {
						continue
					}
					a.reported[origin] = true
					a.pass.Reportf(origin, "return an owned HTTP client from this factory; returning it as a non-owned result loses shutdown responsibility")
				}
			}
		}
	}
}

func (a *ownershipAnalysis) creationOrigins(value ssa.Value, seen map[ssa.Value]bool) []token.Pos {
	if value == nil || seen[value] {
		return nil
	}
	seen[value] = true
	switch current := value.(type) {
	case *ssa.Call:
		if a.callResultCreatesOwned(&current.Call, 0) {
			return []token.Pos{current.Pos()}
		}
	case *ssa.Extract:
		call, ok := current.Tuple.(*ssa.Call)
		if ok && a.callResultCreatesOwned(&call.Call, current.Index) {
			return []token.Pos{call.Pos()}
		}
	case *ssa.ChangeInterface:
		return a.creationOrigins(current.X, seen)
	case *ssa.Convert:
		return a.creationOrigins(current.X, seen)
	case *ssa.MakeInterface:
		return a.creationOrigins(current.X, seen)
	case *ssa.Phi:
		var origins []token.Pos
		for _, edge := range current.Edges {
			origins = append(origins, a.creationOrigins(edge, seen)...)
		}
		return uniquePositions(origins)
	}
	return nil
}

func (a *ownershipAnalysis) callResultCreatesOwned(call *ssa.CallCommon, index int) bool {
	resultType := callResultType(call.Signature(), index)
	if !isOwnedHTTPClient(resultType) {
		return false
	}
	callee := call.StaticCallee()
	if callee == nil {
		return false
	}
	if callee.Pkg != nil && callee.Pkg.Pkg.Path() == infrastructurePkg {
		return true
	}
	return a.functionResultCreatesOwned(callee, index)
}

func (a *ownershipAnalysis) functionResultCreatesOwned(function *ssa.Function, index int) bool {
	key := functionResult{function: function, index: index}
	if result, ok := a.resultMemo[key]; ok {
		return result
	}
	if a.visiting[key] || function.Signature == nil || index >= function.Signature.Results().Len() {
		return false
	}
	a.visiting[key] = true
	defer delete(a.visiting, key)
	for _, block := range function.Blocks {
		for _, instruction := range block.Instrs {
			returned, ok := instruction.(*ssa.Return)
			if !ok || index >= len(returned.Results) {
				continue
			}
			if len(a.creationOrigins(returned.Results[index], make(map[ssa.Value]bool))) > 0 {
				a.resultMemo[key] = true
				return true
			}
		}
	}
	a.resultMemo[key] = false
	return false
}

func callResultType(signature *types.Signature, index int) types.Type {
	if signature != nil && index >= 0 && index < signature.Results().Len() {
		return signature.Results().At(index).Type()
	}
	return nil
}

func consumeAllowance(files *token.FileSet, candidates []*fileAllowances, node token.Pos, rule string) bool {
	for _, candidate := range candidates {
		if candidate.consume(files, node, rule) {
			return true
		}
	}
	return false
}

func uniquePositions(values []token.Pos) []token.Pos {
	seen := make(map[token.Pos]bool, len(values))
	result := make([]token.Pos, 0, len(values))
	for _, value := range values {
		if value != token.NoPos && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func isOwnedHTTPClient(value types.Type) bool {
	if value == nil {
		return false
	}
	methods := types.NewMethodSet(value)
	hasDo := false
	hasShutdown := false
	for index := 0; index < methods.Len(); index++ {
		method := methods.At(index).Obj()
		signature, ok := method.Type().(*types.Signature)
		if !ok {
			continue
		}
		switch method.Name() {
		case "Do":
			hasDo = isHTTPDoSignature(signature)
		case "Shutdown":
			hasShutdown = isShutdownSignature(signature)
		}
	}
	return hasDo && hasShutdown
}

func isHTTPDoSignature(signature *types.Signature) bool {
	return signature.Params().Len() == 1 &&
		signature.Results().Len() == 2 &&
		isHTTPPointer(signature.Params().At(0).Type(), "Request") &&
		isHTTPPointer(signature.Results().At(0).Type(), "Response")
}

func isShutdownSignature(signature *types.Signature) bool {
	if signature.Params().Len() != 1 || signature.Results().Len() != 1 {
		return false
	}
	contextType := signature.Params().At(0).Type()
	named, ok := types.Unalias(contextType).(*types.Named)
	if !ok || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != "context" || named.Obj().Name() != "Context" {
		return false
	}
	errorType := types.Universe.Lookup("error").Type()
	return types.Identical(signature.Results().At(0).Type(), errorType)
}
