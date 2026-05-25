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
	visited := make(map[string]bool)
	inlined := make(map[string]bool)
	return resolveImportsHelper(program, currentDir, visited, inlined)
}

func resolveImportsHelper(program *Program, currentDir string, visited map[string]bool, inlined map[string]bool) error {
	resolvedStmts := []Statement{}

	for _, stmt := range program.Statements {
		if imp, ok := stmt.(*ImportStatement); ok {
			// Find absolute path, considering local directory and Bap packages (lib/)
			path := findImportPath(currentDir, imp.Path)
			absPath, err := filepath.Abs(path)
			if err != nil {
				absPath = path
			}

			// Cycle detection
			if visited[absPath] {
				return fmt.Errorf("енгізу қатесі: циклдік тәуелділік табылды '%s'", absPath)
			}

			// Duplicate import check: if already inlined, just skip it to prevent duplicate declarations
			if inlined[absPath] {
				continue
			}

			// Read the imported file
			content, err := os.ReadFile(absPath)
			if err != nil {
				return fmt.Errorf("енгізу қатесі: файлды оқу мүмкін болмады '%s': %v", absPath, err)
			}

			// Parse the imported file
			l := lexer.New(string(content))
			p := New(l)
			subProg := p.ParseProgram()
			if len(p.Errors()) != 0 {
				return fmt.Errorf("енгізу қатесі: '%s' файлын талдауда қателер табылды: %v", absPath, p.Errors())
			}

			// Recursively resolve imports inside the sub-program
			visited[absPath] = true
			inlined[absPath] = true
			err = resolveImportsHelper(subProg, filepath.Dir(absPath), visited, inlined)
			delete(visited, absPath)
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

// findImportPath searches for the imported file path.
// 1. Checks if the path is relative and exists relative to currentDir.
// 2. Checks if there is a bap.toml in currentDir or its ancestors,
//    and checks if the import path exists in the lib/ folder at that project root.
func findImportPath(currentDir, importPath string) string {
	target := importPath
	if !filepath.IsAbs(target) {
		target = filepath.Join(currentDir, target)
	}
	if _, err := os.Stat(target); err == nil {
		return target
	}

	// Traverse upwards to find project root (containing bap.toml)
	dir := currentDir
	for {
		tomlPath := filepath.Join(dir, "bap.toml")
		if _, err := os.Stat(tomlPath); err == nil {
			// Found project root, check lib/
			libTarget := filepath.Join(dir, "lib", importPath)
			if _, err := os.Stat(libTarget); err == nil {
				return libTarget
			}
			break
		}

		parent := filepath.Dir(dir)
		if parent == dir { // Reached filesystem root
			break
		}
		dir = parent
	}

	return target
}

