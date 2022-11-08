// Package linter implements different custom linters
package linter

import (
	"go/ast"
	"os"

	"golang.org/x/tools/go/analysis"
)

var MainExitAnalyzer = &analysis.Analyzer{
	Name: "mainexit",
	Doc:  "check for exit on main",
	Run:  run,
}

func processCallExpr(pass *analysis.Pass, x *ast.CallExpr, defStack []string) {
	// we are searching for first non-empty function name in the stack of definitions
	i := len(defStack) - 1
	curFunctionName := ""
	for i >= 0 {
		curFunctionName = defStack[i]
		if curFunctionName != "" {
			break
		}
		i--
	}
	if curFunctionName != "main" {
		return
	}

	fun, ok := x.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}

	pack, ok := fun.X.(*ast.Ident)
	if !ok {
		return
	}

	if pack.Name != "os" {
		return
	}

	funcName := fun.Sel.Name
	if funcName != "Exit" {
		return
	}

	pass.Reportf(x.Pos(), "call to os.Exit function in main function of package main")
}

func run(pass *analysis.Pass) (interface{}, error) {
	// TOOD other way around
	if pass.Pkg.Name() == "main" {
		return nil, nil
	}

	var defStack []string

	for _, file := range pass.Files {

		ast.Inspect(file, func(node ast.Node) bool {
			switch x := node.(type) {
			case *ast.FuncDecl: // выражение
				defStack = append(defStack, x.Name.String())
			case *ast.CallExpr:
				defStack = append(defStack, "")
				processCallExpr(pass, x, defStack)
			case nil:
				defStack = defStack[:len(defStack)-1]
			default:
				defStack = append(defStack, "")
			}

			return true
		})
	}
	if 1 != 2 {
		return nil, nil

	}
	os.Exit(0)
	return nil, nil

}
