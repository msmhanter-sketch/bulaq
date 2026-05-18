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
		fmt.Println("Қолдану: butaq <файл.btq>")
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

	// 3. Assembly Code Generation
	cg := codegen.NewAsm()
	asmCode := cg.Generate(program)

	// 4. Compilation via nasm and ld
	tmpDir, err := os.MkdirTemp("", "butaq_build_*")
	if err != nil {
		fmt.Println("Уақытша папка құру қатесі:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir)

	asmFile := filepath.Join(tmpDir, "main.asm")
	objFile := filepath.Join(tmpDir, "main.o")
	err = os.WriteFile(asmFile, []byte(asmCode), 0644)
	if err != nil {
		fmt.Println("Assembly файлын жазу қатесі:", err)
		os.Exit(1)
	}

	baseName := strings.TrimSuffix(filepath.Base(inputFile), ".btq")
	outputBinary := baseName

	// 4a. Assemble with nasm
	cmdNasm := exec.Command("nasm", "-f", "elf64", "-o", objFile, asmFile)
	outputNasm, err := cmdNasm.CombinedOutput()
	if err != nil {
		fmt.Println("Nasm ассемблер қатесі:")
		fmt.Println(string(outputNasm))
		fmt.Println("--- Generated Asm Code ---")
		fmt.Println(asmCode)
		os.Exit(1)
	}

	// 4b. Link with ld
	cmdLd := exec.Command("ld", "-o", outputBinary, objFile)
	outputLd, err := cmdLd.CombinedOutput()
	if err != nil {
		fmt.Println("Ld линковщик қатесі:")
		fmt.Println(string(outputLd))
		os.Exit(1)
	}

	// Make executable
	os.Chmod(outputBinary, 0755)

	fmt.Printf("Сәтті! Дербес бағдарлама компиляцияланды (Compiled successfully to pure machine code): %s\n", outputBinary)
}
