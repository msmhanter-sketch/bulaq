package main

import (
	"butaq/codegen"
	"butaq/lexer"
	"butaq/parser"
	"butaq/typechecker"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Қолдану: butaq <файл.bu>")
		os.Exit(1)
	}

	inputFile := os.Args[1]
	sourceCode, err := os.ReadFile(inputFile)
	if err != nil {
		fmt.Printf("Файлды оқу қатесі: %v\n", err)
		os.Exit(1)
	}

	// 1. Lexing & Parsing
	l := lexer.New(string(sourceCode))
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 0 {
		fmt.Println("Синтаксистік қателер:")
		for _, err := range p.Errors() {
			fmt.Printf("\t- %s\n", err)
		}
		os.Exit(1)
	}

	// 2. Static Type Checking (before execution)
	tcEnv := typechecker.NewTypeEnv()
	tc := typechecker.New()
	tc.Check(program, tcEnv)

	if len(tc.Errors) != 0 {
		fmt.Println("Статикалық тип қателері (Static Type Errors):")
		for _, err := range tc.Errors {
			fmt.Printf("\t- %s\n", err)
		}
		os.Exit(1)
	}

	// 3. C++ Code Generation
	cg := codegen.New(tcEnv)
	cppCode := cg.Generate(program)

	// 4. Compilation via g++
	tmpDir, err := os.MkdirTemp("", "butaq_build_*")
	if err != nil {
		fmt.Println("Уақытша папка құру қатесі:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	cppFile := filepath.Join(tmpDir, "main.cpp")
	err = os.WriteFile(cppFile, []byte(cppCode), 0644)
	if err != nil {
		fmt.Println("C++ файлын жазу қатесі:", err)
		os.Exit(1)
	}

	baseName := strings.TrimSuffix(filepath.Base(inputFile), ".bu")
	outputBinary := baseName

	cmd := exec.Command("g++", "-O3", "-std=c++17", "-o", outputBinary, cppFile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("C++ компиляция қатесі:")
		fmt.Println(string(output))
		// Print generated code for debug
		fmt.Println("--- Generated C++ Code ---")
		fmt.Println(cppCode)
		os.Exit(1)
	}

	// Make executable
	os.Chmod(outputBinary, 0755)

	fmt.Printf("Сәтті! Дербес бағдарлама компиляцияланды (Compiled successfully to standalone native binary): %s\n", outputBinary)
}
