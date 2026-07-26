package main

import (
	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

func inspectFactoryOwnership(pass *analysis.Pass, file *ast.File, allowed *fileAllowances) {
	reported := make(map[token.Pos]bool)
	ast.Inspect(file, func(node ast.Node) bool {
		switch function := node.(type) {
		case *ast.FuncDecl:
			inspectFunctionReturns(pass, function.Body, signatureOf(pass, function.Name), allowed, reported)
		case *ast.FuncLit:
			inspectFunctionReturns(pass, function.Body, signatureOf(pass, function.Type), allowed, reported)
		}
		return true
	})
}

func inspectFunctionReturns(
	pass *analysis.Pass,
	body *ast.BlockStmt,
	signature *types.Signature,
	allowed *fileAllowances,
	reported map[token.Pos]bool,
) {
	if body == nil || signature == nil || signature.Results().Len() == 0 {
		return
	}
	origins := make(map[*types.Var]token.Pos)
	ast.Inspect(body, func(node ast.Node) bool {
		switch current := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.AssignStmt:
			recordAssignmentOrigins(pass, current.Lhs, current.Rhs, origins)
		case *ast.DeclStmt:
			recordDeclarationOrigins(pass, current, origins)
		case *ast.ReturnStmt:
			reportOwnershipErasure(pass, current, signature, origins, allowed, reported)
		}
		return true
	})
}

func recordAssignmentOrigins(pass *analysis.Pass, lhs, rhs []ast.Expr, origins map[*types.Var]token.Pos) {
	if len(lhs) != len(rhs) {
		return
	}
	for index, expression := range rhs {
		origin := ownedClientCreation(pass, expression)
		identifier, ok := lhs[index].(*ast.Ident)
		if !ok {
			continue
		}
		if variable, ok := pass.TypesInfo.ObjectOf(identifier).(*types.Var); ok {
			if origin == token.NoPos {
				delete(origins, variable)
			} else {
				origins[variable] = origin
			}
		}
	}
}

func recordDeclarationOrigins(pass *analysis.Pass, declaration *ast.DeclStmt, origins map[*types.Var]token.Pos) {
	general, ok := declaration.Decl.(*ast.GenDecl)
	if !ok {
		return
	}
	for _, spec := range general.Specs {
		value, ok := spec.(*ast.ValueSpec)
		if !ok || len(value.Names) != len(value.Values) {
			continue
		}
		for index, expression := range value.Values {
			origin := ownedClientCreation(pass, expression)
			if origin == token.NoPos {
				continue
			}
			if variable, ok := pass.TypesInfo.ObjectOf(value.Names[index]).(*types.Var); ok {
				origins[variable] = origin
			}
		}
	}
}

func reportOwnershipErasure(
	pass *analysis.Pass,
	statement *ast.ReturnStmt,
	signature *types.Signature,
	origins map[*types.Var]token.Pos,
	allowed *fileAllowances,
	reported map[token.Pos]bool,
) {
	if len(statement.Results) == 0 {
		for index := 0; index < signature.Results().Len(); index++ {
			result := signature.Results().At(index)
			reportOwnershipOrigin(pass, result.Type(), origins[result], allowed, reported)
		}
		return
	}
	if len(statement.Results) != signature.Results().Len() {
		return
	}
	for index, expression := range statement.Results {
		origin := ownedClientCreation(pass, expression)
		if identifier, ok := expression.(*ast.Ident); ok {
			if variable, ok := pass.TypesInfo.ObjectOf(identifier).(*types.Var); ok && origins[variable] != token.NoPos {
				origin = origins[variable]
			}
		}
		reportOwnershipOrigin(pass, signature.Results().At(index).Type(), origin, allowed, reported)
	}
}

func reportOwnershipOrigin(
	pass *analysis.Pass,
	resultType types.Type,
	origin token.Pos,
	allowed *fileAllowances,
	reported map[token.Pos]bool,
) {
	if isOwnedHTTPClient(resultType) || origin == token.NoPos || reported[origin] ||
		allowed.consume(pass.Fset, origin, "owned-client-result") {
		return
	}
	reported[origin] = true
	pass.Reportf(origin, "return an owned HTTP client from this factory; returning it as a non-owned result loses shutdown responsibility")
}

func ownedClientCreation(pass *analysis.Pass, expression ast.Expr) token.Pos {
	call, ok := expression.(*ast.CallExpr)
	if ok && isOwnedHTTPClient(pass.TypesInfo.TypeOf(call)) {
		return call.Pos()
	}
	return token.NoPos
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

func signatureOf(pass *analysis.Pass, expression ast.Expr) *types.Signature {
	signature, _ := pass.TypesInfo.TypeOf(expression).(*types.Signature)
	return signature
}
