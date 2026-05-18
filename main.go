package main

import (
	"butaq/lexer"
	"butaq/parser"
	"butaq/typechecker"
	"butaq/vm"
	"fmt"
	"os"
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
	tc := typechecker.New()
	tc.Check(program, typechecker.NewTypeEnv()) // uses root TypeEnv

	if len(tc.Errors) != 0 {
		fmt.Println("Статикалық тип қателері (Static Type Errors):")
		for _, err := range tc.Errors {
			fmt.Printf("\t- %s\n", err)
		}
		os.Exit(1)
	}

	// 3. VM Execution
	v := vm.New()
	env := vm.NewEnvironment()
	v.Eval(program, env)
}
