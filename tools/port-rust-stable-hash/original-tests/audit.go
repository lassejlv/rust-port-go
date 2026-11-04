// SPDX-License-Identifier: MIT OR Apache-2.0
// Extract declarations, signed literal values, assertions and source vectors.
package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"math/big"
	"os"
	"strconv"
)

type stats struct {
	Strings    []string `json:"strings"`
	Integers   []string `json:"integers"`
	Assertions []string `json:"assertions"`
	Calls      []string `json:"calls"`
}
type function struct {
	Name string `json:"name"`
	Line int    `json:"line"`
	stats
}
type fileStats struct {
	Functions []function `json:"functions"`
	Vector    [][]string `json:"vector,omitempty"`
	All       stats      `json:"all"`
}

func number(text string) string {
	integer, ok := new(big.Int).SetString(text, 0)
	if !ok {
		panic("invalid Go literal " + text)
	}
	return integer.String()
}
func inspect(root ast.Node) stats {
	var out stats
	ast.Inspect(root, func(node ast.Node) bool {
		switch node := node.(type) {
		case *ast.UnaryExpr:
			if node.Op == token.SUB {
				if literal, ok := node.X.(*ast.BasicLit); ok && literal.Kind == token.INT {
					out.Integers = append(out.Integers, "-"+number(literal.Value))
					return false
				}
			}
		case *ast.BasicLit:
			if node.Kind == token.INT {
				out.Integers = append(out.Integers, number(node.Value))
			}
			if node.Kind == token.STRING || node.Kind == token.CHAR {
				value, err := strconv.Unquote(node.Value)
				if err != nil {
					panic(err)
				}
				out.Strings = append(out.Strings, value)
			}
		case *ast.CallExpr:
			if identifier, ok := node.Fun.(*ast.Ident); ok {
				out.Calls = append(out.Calls, identifier.Name)
				switch identifier.Name {
				case "assert_eq":
					out.Assertions = append(out.Assertions, "assert_eq")
				case "assert_ne":
					out.Assertions = append(out.Assertions, "assert_ne")
				case "assert_true":
					out.Assertions = append(out.Assertions, "assert")
				}
			} else if selector, ok := node.Fun.(*ast.SelectorExpr); ok {
				out.Calls = append(out.Calls, selector.Sel.Name)
			}
		}
		return true
	})
	return out
}
func main() {
	out := map[string]fileStats{}
	for _, path := range os.Args[1:] {
		if path == "--" {
			continue
		}
		fileset := token.NewFileSet()
		file, err := parser.ParseFile(fileset, path, nil, 0)
		if err != nil {
			panic(err)
		}
		entry := fileStats{All: inspect(file)}
		for _, declaration := range file.Decls {
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				entry.Functions = append(entry.Functions, function{Name: declaration.Name.Name, Line: fileset.Position(declaration.Pos()).Line, stats: inspect(declaration.Body)})
			case *ast.GenDecl:
				for _, spec := range declaration.Specs {
					if value, ok := spec.(*ast.ValueSpec); ok && len(value.Names) == 1 && value.Names[0].Name == "TEST_VECTOR" {
						rows := value.Values[0].(*ast.CompositeLit)
						for _, row := range rows.Elts {
							var values []string
							for _, value := range row.(*ast.CompositeLit).Elts {
								values = append(values, number(value.(*ast.BasicLit).Value))
							}
							entry.Vector = append(entry.Vector, values)
						}
					}
				}
			}
		}
		out[path] = entry
	}
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		panic(err)
	}
}
