package main

import (
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
)

const (
	allowPrefix       = "nethttp-lifecycle:allow"
	infrastructurePkg = "github.com/NlightN22/xray-p2p/go/internal/nethttp"
)

var (
	allowPattern = regexp.MustCompile(`^nethttp-lifecycle:allow (http-client-constructor|http-transport-constructor|http-server-constructor|inline-http-do) owner=\S+ lifetime=\S+ reason=\S(?:.*\S)?$`)
	validRules   = map[string]bool{
		"http-client-constructor":    true,
		"http-transport-constructor": true,
		"http-server-constructor":    true,
		"inline-http-do":             true,
	}
)

var Analyzer = &analysis.Analyzer{
	Name: "httplifecycle",
	Doc:  "enforces explicit ownership of HTTP clients, transports, and servers",
	Run:  run,
}

type allowance struct {
	rule string
	end  token.Pos
	used bool
}

type fileAllowances struct {
	file       *ast.File
	allowances []*allowance
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		if strings.HasSuffix(pass.Fset.Position(file.Pos()).Filename, "_test.go") {
			continue
		}
		allowed := parseAllowances(pass, file)
		if pass.Pkg.Path() != infrastructurePkg {
			inspectFile(pass, file, allowed)
		}
		reportUnusedAllowances(pass, allowed)
	}
	return nil, nil
}

func parseAllowances(pass *analysis.Pass, file *ast.File) *fileAllowances {
	result := &fileAllowances{file: file}
	for _, group := range file.Comments {
		for _, comment := range group.List {
			text := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(comment.Text, "//"), "/*"), "*/"))
			if !strings.HasPrefix(text, allowPrefix) {
				continue
			}
			if !allowPattern.MatchString(text) {
				pass.Reportf(comment.Pos(), "invalid HTTP lifecycle allowance; use %q", allowPrefix+" <rule> owner=<owner> lifetime=<lifetime> reason=<reason>")
				continue
			}
			fields := strings.Fields(text)
			if len(fields) < 2 || !validRules[fields[1]] {
				pass.Reportf(comment.Pos(), "unknown HTTP lifecycle allowance rule")
				continue
			}
			result.allowances = append(result.allowances, &allowance{rule: fields[1], end: comment.End()})
		}
	}
	return result
}

func inspectFile(pass *analysis.Pass, file *ast.File, allowed *fileAllowances) {
	ast.Inspect(file, func(node ast.Node) bool {
		switch current := node.(type) {
		case *ast.CompositeLit:
			rule, noun := constructionRule(pass.TypesInfo.TypeOf(current))
			if rule != "" && !allowed.consume(pass.Fset, current.Pos(), rule) {
				pass.Reportf(current.Pos(), "construct %s through go/internal/nethttp so its owner and shutdown are explicit", noun)
			}
		case *ast.CallExpr:
			if rule, noun := newConstructionRule(pass, current); rule != "" {
				if !allowed.consume(pass.Fset, current.Pos(), rule) {
					pass.Reportf(current.Pos(), "construct %s through go/internal/nethttp so its owner and shutdown are explicit", noun)
				}
			} else if isInlineDo(pass, current) && !allowed.consume(pass.Fset, current.Pos(), "inline-http-do") {
				pass.Reportf(current.Pos(), "store and own the HTTP client before calling Do; do not use factory(...).Do(...)")
			}
		}
		return true
	})
}

func constructionRule(value types.Type) (string, string) {
	named, ok := types.Unalias(value).(*types.Named)
	if !ok || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != "net/http" {
		return "", ""
	}
	switch named.Obj().Name() {
	case "Client":
		return "http-client-constructor", "http.Client"
	case "Transport":
		return "http-transport-constructor", "http.Transport"
	case "Server":
		return "http-server-constructor", "http.Server"
	default:
		return "", ""
	}
}

func newConstructionRule(pass *analysis.Pass, call *ast.CallExpr) (string, string) {
	identifier, ok := call.Fun.(*ast.Ident)
	if !ok || identifier.Name != "new" || len(call.Args) != 1 {
		return "", ""
	}
	builtin, ok := pass.TypesInfo.Uses[identifier].(*types.Builtin)
	if !ok || builtin.Name() != "new" {
		return "", ""
	}
	return constructionRule(pass.TypesInfo.TypeOf(call.Args[0]))
}

func isInlineDo(pass *analysis.Pass, call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Do" {
		return false
	}
	if _, ok := selector.X.(*ast.CallExpr); !ok {
		return false
	}
	selection := pass.TypesInfo.Selections[selector]
	if selection == nil {
		return false
	}
	signature, ok := selection.Obj().Type().(*types.Signature)
	if !ok || signature.Params().Len() != 1 || signature.Results().Len() != 2 {
		return false
	}
	request := signature.Params().At(0).Type()
	response := signature.Results().At(0).Type()
	return isHTTPPointer(request, "Request") && isHTTPPointer(response, "Response")
}

func isHTTPPointer(value types.Type, name string) bool {
	pointer, ok := types.Unalias(value).(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := types.Unalias(pointer.Elem()).(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "net/http" && named.Obj().Name() == name
}

func (f *fileAllowances) consume(files *token.FileSet, node token.Pos, rule string) bool {
	nodePosition := files.Position(node)
	for _, candidate := range f.allowances {
		if candidate.used || candidate.rule != rule {
			continue
		}
		commentPosition := files.Position(candidate.end)
		if commentPosition.Filename == nodePosition.Filename &&
			(commentPosition.Line == nodePosition.Line || commentPosition.Line+1 == nodePosition.Line) {
			candidate.used = true
			return true
		}
	}
	return false
}

func reportUnusedAllowances(pass *analysis.Pass, allowed *fileAllowances) {
	for _, candidate := range allowed.allowances {
		if !candidate.used {
			pass.Reportf(candidate.end, "HTTP lifecycle allowance does not match a forbidden construction")
		}
	}
}
