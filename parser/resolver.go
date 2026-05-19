package parser

import (
	"butaq/lexer"
	"fmt"
	"os"
	"path/filepath"
)

// ResolveImports recursively resolves all ImportStatements inside the program.
// It loads, lexes, and parses the imported files, inserting their statements
// inline in place of the ImportStatement.
func ResolveImports(program *Program, currentDir string) error {
	resolvedStmts := []Statement{}

	for _, stmt := range program.Statements {
		if imp, ok := stmt.(*ImportStatement); ok {
			// Find absolute path
			path := imp.Path
			if !filepath.IsAbs(path) {
				path = filepath.Join(currentDir, path)
			}

			// Read the imported file
			content, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("енгізу қатесі: файлды оқу мүмкін болмады '%s': %v", path, err)
			}

			// Parse the imported file
			l := lexer.New(string(content))
			p := New(l)
			subProg := p.ParseProgram()
			if len(p.Errors()) != 0 {
				return fmt.Errorf("енгізу қатесі: '%s' файлын талдауда қателер табылды: %v", path, p.Errors())
			}

			// Recursively resolve imports inside the sub-program
			err = ResolveImports(subProg, filepath.Dir(path))
			if err != nil {
				return err
			}

			// Append all statements from the imported file
			resolvedStmts = append(resolvedStmts, subProg.Statements...)
		} else {
			resolvedStmts = append(resolvedStmts, stmt)
		}
	}

	program.Statements = resolvedStmts
	return nil
}
