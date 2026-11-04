// Command audit extracts Go AST declarations, literal values and assertion calls
// for the semver original-test preservation audit.
package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
)

type fileAudit struct {
	Functions  []string `json:"functions"`
	Literals   []string `json:"literals"`
	Integers   []string `json:"integers"`
	Assertions int      `json:"assertions"`
}

func main() {
	result := map[string]fileAudit{}
	for _, path := range os.Args[1:] {
		if path == "--" {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			panic(err)
		}
		entry := fileAudit{}
		for _, declaration := range file.Decls {
			if function, ok := declaration.(*ast.FuncDecl); ok {
				entry.Functions = append(entry.Functions, function.Name.Name)
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.BasicLit:
				if node.Kind == token.INT {
					entry.Integers = append(entry.Integers, node.Value)
				}
				if node.Kind == token.STRING || node.Kind == token.CHAR {
					value, err := strconv.Unquote(node.Value)
					if err != nil {
						panic(err)
					}
					entry.Literals = append(entry.Literals, value)
				}
			case *ast.CallExpr:
				if identifier, ok := node.Fun.(*ast.Ident); ok && (identifier.Name == "assert_eq" || identifier.Name == "assert_ne" || identifier.Name == "assert_true") {
					entry.Assertions++
				}
			}
			return true
		})
		result[path] = entry
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		panic(err)
	}
}
