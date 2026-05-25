package main

import (
	"butaq/codegen"
	"butaq/interpreter"
	"butaq/lexer"
	"butaq/parser"
	"butaq/typechecker"
	"strings"
	"syscall/js"
)

func runButaqCode(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return map[string]interface{}{"error": "кіріс файлы бос"}
	}
	source := args[0].String()

	l := lexer.New(source)
	p := parser.New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		return map[string]interface{}{
			"error": "Синтаксистік қателер:\n" + strings.Join(p.Errors(), "\n"),
		}
	}

	tcEnv := typechecker.NewTypeEnv()
	tc := typechecker.New()
	tc.Check(prog, tcEnv)

	if len(tc.Errors) > 0 {
		return map[string]interface{}{
			"error": "Тип қателері:\n" + strings.Join(tc.ErrorStrings(), "\n"),
		}
	}

	ip := interpreter.New()
	out, err := ip.Run(prog)
	if err != nil {
		return map[string]interface{}{
			"error": "Орындалу қатесі: " + err.Error(),
		}
	}

	return map[string]interface{}{
		"output": out,
	}
}

func compileToNasm(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return map[string]interface{}{"error": "кіріс файлы бос"}
	}
	source := args[0].String()

	l := lexer.New(source)
	p := parser.New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		return map[string]interface{}{
			"error": "Синтаксистік қателер:\n" + strings.Join(p.Errors(), "\n"),
		}
	}

	tcEnv := typechecker.NewTypeEnv()
	tc := typechecker.New()
	tc.Check(prog, tcEnv)

	if len(tc.Errors) > 0 {
		return map[string]interface{}{
			"error": "Тип қателері:\n" + strings.Join(tc.ErrorStrings(), "\n"),
		}
	}

	cg := codegen.NewWithTC(tcEnv, tc, codegen.PlatformWindows)
	code := cg.Generate(prog)
	return map[string]interface{}{
		"code": code,
	}
}

func compileToLlvm(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return map[string]interface{}{"error": "кіріс файлы бос"}
	}
	source := args[0].String()

	l := lexer.New(source)
	p := parser.New(l)
	prog := p.ParseProgram()

	if len(p.Errors()) > 0 {
		return map[string]interface{}{
			"error": "Синтаксистік қателер:\n" + strings.Join(p.Errors(), "\n"),
		}
	}

	tcEnv := typechecker.NewTypeEnv()
	tc := typechecker.New()
	tc.Check(prog, tcEnv)

	if len(tc.Errors) > 0 {
		return map[string]interface{}{
			"error": "Тип қателері:\n" + strings.Join(tc.ErrorStrings(), "\n"),
		}
	}

	lg := codegen.NewLlvm(tcEnv, tc, codegen.PlatformWindows)
	code := lg.Generate(prog)
	return map[string]interface{}{
		"code": code,
	}
}

func main() {
	c := make(chan struct{}, 0)
	js.Global().Set("runButaqCode", js.FuncOf(runButaqCode))
	js.Global().Set("compileToNasm", js.FuncOf(compileToNasm))
	js.Global().Set("compileToLlvm", js.FuncOf(compileToLlvm))
	<-c
}
