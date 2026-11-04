package compiler

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lassejlv/rust-port-go/internal/syntax"
)

// CheckFile checks a binary crate, including declared external modules.
func CheckFile(sourcePath string) error {
	_, _, err := analyzeFile(sourcePath)
	return err
}

// EmitCFile emits C11 for a binary crate, loading its external modules.
func EmitCFile(sourcePath string) (string, error) {
	c, _, err := analyzeFile(sourcePath)
	if err != nil {
		return "", err
	}
	return emitChecked(c), nil
}

func analyzeFile(sourcePath string) (*checked, []string, error) {
	return analyzeCrate(&Crate{Root: sourcePath}, true)
}

type moduleLoader struct {
	active  []os.FileInfo
	sources []string
}

func loadCrate(sourcePath string) (*syntax.Program, []string, error) {
	absolute, err := filepath.Abs(sourcePath)
	if err != nil {
		return nil, nil, err
	}
	loader := &moduleLoader{}
	program, err := loader.file(absolute, filepath.Dir(absolute), 0)
	return program, loader.sources, err
}

func (l *moduleLoader) file(path, base string, depth int) (*syntax.Program, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("read module %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("module source %s must be a regular file", path)
	}
	for _, active := range l.active {
		if os.SameFile(info, active) {
			return nil, fmt.Errorf("module file cycle involving %s", path)
		}
	}
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read module %s: %w", path, err)
	}
	program, err := syntax.Parse(string(source))
	if err != nil {
		return nil, fmt.Errorf("%s:%w", path, err)
	}
	stampProgram(program, path)
	l.active = append(l.active, info)
	defer func() { l.active = l.active[:len(l.active)-1] }()
	l.sources = append(l.sources, path)
	if err = l.modules(program, base, depth); err != nil {
		return nil, err
	}
	return program, nil
}

func (l *moduleLoader) modules(program *syntax.Program, base string, depth int) error {
	if depth > 256 {
		return fmt.Errorf("module nesting exceeds the bootstrap limit of 256")
	}
	for _, module := range program.Modules {
		childBase := filepath.Join(base, module.Name)
		if module.External {
			candidates := []string{childBase + ".rs", filepath.Join(childBase, "mod.rs")}
			var selected string
			for _, candidate := range candidates {
				info, err := os.Stat(candidate)
				if os.IsNotExist(err) {
					continue
				}
				if err != nil {
					return fmt.Errorf("%s:%d:%d: inspect module %s: %w", module.Span.File, module.Span.Line, module.Span.Column, candidate, err)
				}
				if !info.Mode().IsRegular() {
					return fmt.Errorf("%s:%d:%d: module source %s must be a regular file", module.Span.File, module.Span.Line, module.Span.Column, candidate)
				}
				if selected != "" {
					return fmt.Errorf("%s:%d:%d: ambiguous module %q: both %s and %s exist", module.Span.File, module.Span.Line, module.Span.Column, module.Name, selected, candidate)
				}
				selected = candidate
			}
			if selected == "" {
				return fmt.Errorf("%s:%d:%d: missing module %q: expected %s or %s", module.Span.File, module.Span.Line, module.Span.Column, module.Name, candidates[0], candidates[1])
			}
			child, err := l.file(selected, childBase, depth+1)
			if err != nil {
				return err
			}
			module.Program = child
		} else {
			if err := l.modules(module.Program, childBase, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func stampProgram(program *syntax.Program, file string) {
	for _, structure := range program.Structs {
		structure.Span.File = file
		for i := range structure.Fields {
			structure.Fields[i].Span.File = file
		}
	}
	for _, function := range program.Functions {
		function.Span.File = file
		for i := range function.Params {
			function.Params[i].Span.File = file
		}
		stampBlock(function.Body, file)
	}
	for i := range program.Imports {
		program.Imports[i].Span.File = file
	}
	for _, module := range program.Modules {
		module.Span.File = file
		if module.Program != nil {
			stampProgram(module.Program, file)
		}
	}
}
func stampBlock(block *syntax.Block, file string) {
	if block == nil {
		return
	}
	block.Span.File = file
	for i := range block.Statements {
		statement := &block.Statements[i]
		statement.Span.File = file
		stampExpr(statement.Expr, file)
		stampExpr(statement.Target, file)
		stampBlock(statement.Body, file)
		stampBlock(statement.Else, file)
	}
	stampExpr(block.Tail, file)
}
func stampExpr(expression *syntax.Expr, file string) {
	if expression == nil {
		return
	}
	expression.Span.File = file
	for i := range expression.Fields {
		expression.Fields[i].Span.File = file
		stampExpr(expression.Fields[i].Value, file)
	}
	stampExpr(expression.Left, file)
	stampExpr(expression.Right, file)
	for _, argument := range expression.Args {
		stampExpr(argument, file)
	}
	stampBlock(expression.Body, file)
	stampBlock(expression.Else, file)
}

// protectSources also catches symlink and hard-link aliases to module sources.
func protectSources(outputPath string, sources []string) error {
	absolute, err := filepath.Abs(outputPath)
	if err != nil {
		return err
	}
	outputInfo, outputErr := os.Stat(absolute)
	if outputErr != nil && !os.IsNotExist(outputErr) {
		return outputErr
	}
	for _, source := range sources {
		sourceAbs, err := filepath.Abs(source)
		if err != nil {
			return err
		}
		if absolute == sourceAbs {
			return fmt.Errorf("output would overwrite source file %s", source)
		}
		if outputErr == nil {
			sourceInfo, err := os.Stat(sourceAbs)
			if err != nil {
				return err
			}
			if os.SameFile(sourceInfo, outputInfo) {
				return fmt.Errorf("output refers to source file %s", source)
			}
		}
	}
	return nil
}

// WriteC emits C atomically while protecting every source file in the crate.
func WriteC(sourcePath, outputPath string) error {
	outputAbs, err := filepath.Abs(outputPath)
	if err != nil {
		return err
	}
	outputPath = outputAbs
	c, sources, err := analyzeFile(sourcePath)
	if err != nil {
		return err
	}
	if err = protectSources(outputPath, sources); err != nil {
		return err
	}
	output, err := os.CreateTemp(filepath.Dir(outputPath), ".ferrugo-emit-*")
	if err != nil {
		return err
	}
	path := output.Name()
	defer os.Remove(path)
	if _, err = output.WriteString(emitChecked(c)); err != nil {
		output.Close()
		return err
	}
	if err = output.Chmod(0644); err != nil {
		output.Close()
		return err
	}
	if err = output.Close(); err != nil {
		return err
	}
	return os.Rename(path, outputPath)
}
