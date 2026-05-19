package codegen

import (
	"butaq/parser"
	"butaq/typechecker"
	"fmt"
	"math"
	"strings"
)

// ---------------------------------------------------------------------------
// NASM x86-64 Code Generator for Butaq
//
// ABI: System V AMD64 (Linux/macOS) and Microsoft x64 (Windows).
// We generate platform-aware code based on a build tag.
// For simplicity, we generate Linux NASM by default (works with WSL/NASM on Linux).
// ---------------------------------------------------------------------------

type Platform int

const (
	PlatformLinux   Platform = iota
	PlatformWindows Platform = iota
)

type CppGenerator struct { // kept name for backward compat with main.go
	env      *typechecker.TypeEnv
	tc       *typechecker.TypeChecker
	platform Platform

	// Assembly sections
	dataSec strings.Builder // .data section
	bssSec  strings.Builder // .bss section
	textSec strings.Builder // .text section
	funcSec strings.Builder // function bodies

	// Counters for unique label generation
	labelCount int
	strCount   int
	floatCount int

	// Variable stack frame offsets: varName -> rbp offset (negative)
	varOffsets    map[string]int
	currentOffset int // grows downward (e.g. -8, -16, ...)

	// Variable type tracking: true = string/ptr, false = float/number
	varIsString map[string]bool

	// Function context
	currentFunc  string
	funcParams   map[string][]string       // funcName -> param names
	funcParamOff map[string]map[string]int // funcName -> paramName -> rbp offset

	// Known functions from typechecker
	funcs map[string]*typechecker.FuncSig

	// Known structs from typechecker
	structs map[string]*parser.StructStatement

	// Loop label stacks for break and continue
	loopStartLabels []string
	loopEndLabels   []string
}

func New(env *typechecker.TypeEnv) *CppGenerator {
	return NewWithTC(env, nil, PlatformLinux)
}

func NewWithTC(env *typechecker.TypeEnv, tc *typechecker.TypeChecker, platform Platform) *CppGenerator {
	cg := &CppGenerator{
		env:          env,
		tc:           tc,
		platform:     platform,
		varOffsets:   make(map[string]int),
		varIsString:  make(map[string]bool),
		funcParams:   make(map[string][]string),
		funcParamOff: make(map[string]map[string]int),
	}
	if tc != nil {
		cg.funcs = tc.GetFuncs()
		cg.structs = tc.GetStructs()
	} else {
		cg.funcs = make(map[string]*typechecker.FuncSig)
		cg.structs = make(map[string]*parser.StructStatement)
	}
	return cg
}

// ---------------------------------------------------------------------------
// Generate — top-level entry point
// ---------------------------------------------------------------------------

func (cg *CppGenerator) Generate(program *parser.Program) string {
	// Setup sections
	cg.dataSec.WriteString("section .data\n")
	cg.bssSec.WriteString("section .bss\n")
	cg.textSec.WriteString("section .text\n")

	if cg.platform == PlatformLinux {
		cg.textSec.WriteString("    global _start\n\n")
	} else {
		cg.textSec.WriteString("    global main\n")
		cg.textSec.WriteString("    extern printf\n")
		cg.textSec.WriteString("    extern strlen\n")
		cg.textSec.WriteString("    extern strcmp\n")
		cg.textSec.WriteString("    extern strcat\n")
		cg.textSec.WriteString("    extern strcpy\n")
		cg.textSec.WriteString("    extern malloc\n")
		cg.textSec.WriteString("    extern free\n")
		cg.textSec.WriteString("    extern sprintf\n")
		cg.textSec.WriteString("    extern fopen\n")
		cg.textSec.WriteString("    extern fclose\n")
		cg.textSec.WriteString("    extern fread\n")
		cg.textSec.WriteString("    extern fwrite\n")
		cg.textSec.WriteString("    extern fseek\n")
		cg.textSec.WriteString("    extern ftell\n")
		cg.textSec.WriteString("    extern fflush\n")
		cg.textSec.WriteString("    extern scanf\n")
		cg.textSec.WriteString("    extern rand\n")
		cg.textSec.WriteString("    extern SetConsoleOutputCP\n")
		cg.textSec.WriteString("    extern ExitProcess\n\n")
	}

	// Built-in string constants
	cg.dataSec.WriteString("    fmt_float db \"%g\", 10, 0\n")
	cg.dataSec.WriteString("    fmt_str   db \"%s\", 10, 0\n")
	cg.dataSec.WriteString("    fmt_int   db \"%lld\", 10, 0\n")
	cg.dataSec.WriteString("    fmt_numstr db \"%g\", 0\n")
	cg.dataSec.WriteString("    fmt_input  db \"%[^\\n]\", 0\n") // кіру: read line from stdin
	cg.dataSec.WriteString("    file_r    db \"r\", 0\n")
	cg.dataSec.WriteString("    file_w    db \"w\", 0\n")
	cg.dataSec.WriteString("    newline   db 10, 0\n")
	cg.dataSec.WriteString("    err_null  db \"Қате: бос сілтемеге (null) жүгіну!\", 0\n")
	cg.dataSec.WriteString("    err_div0  db \"Қате: нөлге бөлуге болмайды!\", 0\n")
	cg.dataSec.WriteString("    err_bounds db \"Қате: жиын/тізім шекарасынан шығу!\", 0\n")
	// Static 64KB scratch buffer for string concat / file read
	cg.bssSec.WriteString("    scratch_buf resb 65536\n")
	cg.bssSec.WriteString("    input_buf   resb 4096\n") // кіру: stdin line buffer
	cg.bssSec.WriteString("    numstr_buf  resb 64\n")
	cg.bssSec.WriteString("    char_buf    resb 4\n") // 1-char string for символ

	// Collect all function definitions first (forward declarations)
	var mainStmts []parser.Statement
	for _, stmt := range program.Statements {
		if fs, ok := stmt.(*parser.FunctionStatement); ok {
			cg.genFunctionDef(fs)
		} else {
			mainStmts = append(mainStmts, stmt)
		}
	}

	// Generate _start / main
	cg.varOffsets = make(map[string]int)
	cg.currentOffset = 0

	// First pass: calculate stack frame size for main
	stackSize := cg.calcStackSize(mainStmts)
	stackSize = alignTo16(stackSize)

	var entryLabel string
	if cg.platform == PlatformLinux {
		entryLabel = "_start"
	} else {
		entryLabel = "main"
	}

	cg.textSec.WriteString(entryLabel + ":\n")
	cg.textSec.WriteString("    push rbp\n")
	cg.textSec.WriteString("    mov rbp, rsp\n")
	if stackSize > 0 {
		cg.textSec.WriteString(fmt.Sprintf("    sub rsp, %d\n", stackSize))
	}

	for _, stmt := range mainStmts {
		cg.genStatement(stmt, &cg.textSec)
	}

	// Exit
	if cg.platform == PlatformLinux {
		cg.textSec.WriteString("    ; --- exit ---\n")
		cg.textSec.WriteString("    mov rax, 60\n")  // syscall: exit
		cg.textSec.WriteString("    xor rdi, rdi\n") // exit code 0
		cg.textSec.WriteString("    syscall\n")
	} else {
		cg.textSec.WriteString("    ; --- exit ---\n")
		cg.textSec.WriteString("    xor eax, eax\n")
		if stackSize > 0 {
			cg.textSec.WriteString(fmt.Sprintf("    add rsp, %d\n", stackSize))
		}
		cg.textSec.WriteString("    pop rbp\n")
		cg.textSec.WriteString("    ret\n")
	}

	// Assemble output: external print helper (Linux only for now)
	var out strings.Builder
	out.WriteString("; Butaq compiled output — NASM x86-64\n")
	out.WriteString("; Generated automatically. Do not edit.\n\n")
	out.WriteString("    default rel\n\n")

	if cg.platform == PlatformLinux {
		out.WriteString("    extern printf\n")
		out.WriteString("    extern fflush\n")
		out.WriteString("    extern stdout\n")
		out.WriteString("    extern strlen\n")
		out.WriteString("    extern strcmp\n")
		out.WriteString("    extern strcat\n")
		out.WriteString("    extern strcpy\n")
		out.WriteString("    extern malloc\n")
		out.WriteString("    extern free\n")
		out.WriteString("    extern sprintf\n")
		out.WriteString("    extern fopen\n")
		out.WriteString("    extern fclose\n")
		out.WriteString("    extern fread\n")
		out.WriteString("    extern fwrite\n")
		out.WriteString("    extern fseek\n")
		out.WriteString("    extern ftell\n")
		out.WriteString("    extern rand\n")
		out.WriteString("    extern scanf\n\n")
	}

	// Generate runtime error helpers
	cg.funcSec.WriteString("\n; --- Runtime error handlers ---\n")

	// 1. Null pointer error
	cg.funcSec.WriteString("_runtime_null_pointer_error:\n")
	if cg.platform == PlatformLinux {
		cg.funcSec.WriteString("    lea rsi, [err_null]\n")
		cg.funcSec.WriteString("    lea rdi, [fmt_str]\n")
		cg.funcSec.WriteString("    xor rax, rax\n")
		cg.funcSec.WriteString("    call printf\n")
		cg.funcSec.WriteString("    mov rax, 60\n")
		cg.funcSec.WriteString("    mov rdi, 1\n")
		cg.funcSec.WriteString("    syscall\n")
	} else {
		cg.funcSec.WriteString("    lea rdx, [err_null]\n")
		cg.funcSec.WriteString("    lea rcx, [fmt_str]\n")
		cg.funcSec.WriteString("    sub rsp, 32\n")
		cg.funcSec.WriteString("    call printf\n")
		cg.funcSec.WriteString("    add rsp, 32\n")
		cg.funcSec.WriteString("    mov rcx, 1\n")
		cg.funcSec.WriteString("    call ExitProcess\n")
	}

	// 2. Division by zero error
	cg.funcSec.WriteString("_runtime_divide_by_zero_error:\n")
	if cg.platform == PlatformLinux {
		cg.funcSec.WriteString("    lea rsi, [err_div0]\n")
		cg.funcSec.WriteString("    lea rdi, [fmt_str]\n")
		cg.funcSec.WriteString("    xor rax, rax\n")
		cg.funcSec.WriteString("    call printf\n")
		cg.funcSec.WriteString("    mov rax, 60\n")
		cg.funcSec.WriteString("    mov rdi, 1\n")
		cg.funcSec.WriteString("    syscall\n")
	} else {
		cg.funcSec.WriteString("    lea rdx, [err_div0]\n")
		cg.funcSec.WriteString("    lea rcx, [fmt_str]\n")
		cg.funcSec.WriteString("    sub rsp, 32\n")
		cg.funcSec.WriteString("    call printf\n")
		cg.funcSec.WriteString("    add rsp, 32\n")
		cg.funcSec.WriteString("    mov rcx, 1\n")
		cg.funcSec.WriteString("    call ExitProcess\n")
	}

	// 3. Array bounds error
	cg.funcSec.WriteString("_runtime_array_bounds_error:\n")
	if cg.platform == PlatformLinux {
		cg.funcSec.WriteString("    lea rsi, [err_bounds]\n")
		cg.funcSec.WriteString("    lea rdi, [fmt_str]\n")
		cg.funcSec.WriteString("    xor rax, rax\n")
		cg.funcSec.WriteString("    call printf\n")
		cg.funcSec.WriteString("    mov rax, 60\n")
		cg.funcSec.WriteString("    mov rdi, 1\n")
		cg.funcSec.WriteString("    syscall\n")
	} else {
		cg.funcSec.WriteString("    lea rdx, [err_bounds]\n")
		cg.funcSec.WriteString("    lea rcx, [fmt_str]\n")
		cg.funcSec.WriteString("    sub rsp, 32\n")
		cg.funcSec.WriteString("    call printf\n")
		cg.funcSec.WriteString("    add rsp, 32\n")
		cg.funcSec.WriteString("    mov rcx, 1\n")
		cg.funcSec.WriteString("    call ExitProcess\n")
	}

	out.WriteString(cg.dataSec.String())
	out.WriteString(cg.bssSec.String())
	out.WriteString("\n")
	out.WriteString(cg.textSec.String())
	out.WriteString(cg.funcSec.String())

	return out.String()
}

// ---------------------------------------------------------------------------
// Stack frame helpers
// ---------------------------------------------------------------------------

func (cg *CppGenerator) calcStackSize(stmts []parser.Statement) int {
	count := cg.countVars(stmts)
	return count * 8 // each var gets 8 bytes (double/pointer)
}

func (cg *CppGenerator) countVars(stmts []parser.Statement) int {
	seen := make(map[string]bool)
	cg.collectUniqueVarNames(stmts, seen)
	return len(seen)
}

func (cg *CppGenerator) collectUniqueVarNames(stmts []parser.Statement, seen map[string]bool) {
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *parser.VarAssignStatement:
			seen[s.Name.Value] = true
		case *parser.IfStatement:
			if s.Consequence != nil {
				cg.collectUniqueVarNames(s.Consequence.Statements, seen)
			}
			if s.Alternative != nil {
				cg.collectUniqueVarNames(s.Alternative.Statements, seen)
			}
		case *parser.WhileStatement:
			if s.Body != nil {
				cg.collectUniqueVarNames(s.Body.Statements, seen)
			}
		}
	}
}

func alignTo16(n int) int {
	if n == 0 {
		return 0
	}
	return ((n + 15) / 16) * 16
}

func (cg *CppGenerator) allocVar(name string) int {
	if off, ok := cg.varOffsets[name]; ok {
		return off
	}
	cg.currentOffset -= 8
	cg.varOffsets[name] = cg.currentOffset
	return cg.currentOffset
}

func (cg *CppGenerator) getVarOffset(name string) (int, bool) {
	off, ok := cg.varOffsets[name]
	return off, ok
}

// ---------------------------------------------------------------------------
// Label generation
// ---------------------------------------------------------------------------

func (cg *CppGenerator) newLabel(prefix string) string {
	cg.labelCount++
	return fmt.Sprintf(".%s_%d", prefix, cg.labelCount)
}

func (cg *CppGenerator) newStr(value string) string {
	cg.strCount++
	label := fmt.Sprintf("str_%d", cg.strCount)
	// Escape the string for NASM
	escaped := nasmEscapeString(value)
	cg.dataSec.WriteString(fmt.Sprintf("    %s db %s, 0\n", label, escaped))
	return label
}

func (cg *CppGenerator) newFloat(value float64) string {
	cg.floatCount++
	label := fmt.Sprintf("flt_%d", cg.floatCount)
	bits := math.Float64bits(value)
	cg.dataSec.WriteString(fmt.Sprintf("    %s dq 0x%016X  ; %g\n", label, bits, value))
	return label
}

// ---------------------------------------------------------------------------
// Statement code generation
// ---------------------------------------------------------------------------

func (cg *CppGenerator) genStatement(node parser.Statement, sec *strings.Builder) {
	switch n := node.(type) {

	case *parser.VarAssignStatement:
		cg.genVarAssign(n, sec)

	case *parser.PrintStatement:
		cg.genPrint(n, sec)

	case *parser.IfStatement:
		cg.genIf(n, sec)

	case *parser.WhileStatement:
		cg.genWhile(n, sec)

	case *parser.BreakStatement:
		if len(cg.loopEndLabels) > 0 {
			target := cg.loopEndLabels[len(cg.loopEndLabels)-1]
			sec.WriteString(fmt.Sprintf("    jmp %s\n", target))
		} else {
			sec.WriteString("    ; ERROR: үзу сыртта қолданылды\n")
		}

	case *parser.ContinueStatement:
		if len(cg.loopStartLabels) > 0 {
			target := cg.loopStartLabels[len(cg.loopStartLabels)-1]
			sec.WriteString(fmt.Sprintf("    jmp %s\n", target))
		} else {
			sec.WriteString("    ; ERROR: жалғастыру сыртта қолданылды\n")
		}

	case *parser.ReturnStatement:
		cg.genExpression(n.Value, sec)
		if cg.isStringExpr(n.Value) {
			sec.WriteString("    movq xmm0, rax\n") // pass string pointer via xmm0
		}
		sec.WriteString("    ; қайтару\n")
		sec.WriteString("    mov rsp, rbp\n")
		sec.WriteString("    pop rbp\n")
		sec.WriteString("    ret\n")

	case *parser.IndexAssignStatement:
		// arr idx val тізім_қой
		sec.WriteString(fmt.Sprintf("    ; %s[…] тізім_қой\n", n.Array.Value))
		// Load array pointer into r12
		off, ok := cg.getVarOffset(n.Array.Value)
		if !ok {
			sec.WriteString(fmt.Sprintf("    ; ERROR: undeclared array '%s'\n", n.Array.Value))
			return
		}
		sec.WriteString(fmt.Sprintf("    mov r12, [rbp%+d]\n", off)) // r12 = array ptr
		sec.WriteString("    cmp r12, 0\n")
		sec.WriteString("    je _runtime_null_pointer_error\n")
		// Evaluate index into r13 (int)
		cg.genExpression(n.Index, sec)
		sec.WriteString("    cvttsd2si r13, xmm0\n") // r13 = int index
		// Check bounds
		sec.WriteString("    cmp r13, 0\n")
		sec.WriteString("    jl _runtime_array_bounds_error\n")
		sec.WriteString("    cmp r13, [r12]\n")
		sec.WriteString("    jge _runtime_array_bounds_error\n")

		sec.WriteString("    imul r13, 8\n")         // byte offset
		sec.WriteString("    add r13, 8\n")          // skip 8-byte length prefix
		// Evaluate value
		sec.WriteString("    push r12\n")
		sec.WriteString("    push r13\n")
		cg.genExpression(n.Value, sec)
		sec.WriteString("    pop r13\n")
		sec.WriteString("    pop r12\n")
		// Store at array[index] based on type
		valType := cg.tc.Check(n.Value, cg.env)
		if valType == typechecker.STRING_TYPE || valType == typechecker.ARRAY_TYPE || strings.HasPrefix(string(valType), "ҚҰРЫЛЫМ_") {
			sec.WriteString("    mov [r12 + r13], rax\n")
		} else {
			sec.WriteString("    movsd [r12 + r13], xmm0\n")
		}

	case *parser.FreeStatement:
		// val бос  — free(ptr)
		sec.WriteString("    ; бос\n")
		cg.genExpression(n.Value, sec) // rax = pointer
		if cg.platform == PlatformWindows {
			sec.WriteString("    mov rcx, rax\n")
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call free\n")
			sec.WriteString("    add rsp, 32\n")
		} else {
			sec.WriteString("    mov rdi, rax\n")
			sec.WriteString("    call free\n")
		}

	case *parser.CallStatement:
		cg.genCallExpr(n.Call, sec)

	case *parser.FileWriteStatement:
		cg.genFileWrite(n, sec)

	case *parser.StructStatement:
		// Declarations only, no runtime code

	case *parser.StructFieldAssignStatement:
		var structName string
		parts := strings.Split(n.Field, ".")
		if n.Target != nil {
			sec.WriteString(fmt.Sprintf("    ; %s болсын (nested)\n", n.Field))
			cg.genExpression(n.Target, sec)
			sec.WriteString("    push rax\n")
			cg.genExpression(n.Value, sec)
			sec.WriteString("    pop r12\n")
			targetType := cg.tc.Check(n.Target, cg.env)
			structName = string(targetType)[len("ҚҰРЫЛЫМ_"):]
		} else {
			sec.WriteString(fmt.Sprintf("    ; %s.%s болсын\n", n.StructName, n.Field))
			off, ok := cg.getVarOffset(n.StructName)
			if !ok {
				sec.WriteString(fmt.Sprintf("    ; ERROR: undeclared struct '%s'\n", n.StructName))
				return
			}
			t, ok := cg.env.Get(n.StructName)
			if !ok || !strings.HasPrefix(string(t), "ҚҰРЫЛЫМ_") {
				sec.WriteString(fmt.Sprintf("    ; ERROR: '%s' is not a struct\n", n.StructName))
				return
			}
			structName = string(t)[len("ҚҰРЫЛЫМ_"):]

			// Evaluate value
			cg.genExpression(n.Value, sec)
			sec.WriteString(fmt.Sprintf("    mov r12, [rbp%+d]\n", off))
		}
		sec.WriteString("    cmp r12, 0\n")
		sec.WriteString("    je _runtime_null_pointer_error\n")

		currentStructType := structName
		for i, part := range parts {
			structDef, ok := cg.structs[currentStructType]
			if !ok {
				sec.WriteString(fmt.Sprintf("    ; ERROR: struct definition '%s' not found\n", currentStructType))
				return
			}
			fieldIdx := -1
			for idx, f := range structDef.Fields {
				if f == part {
					fieldIdx = idx
					break
				}
			}
			if fieldIdx == -1 {
				sec.WriteString(fmt.Sprintf("    ; ERROR: field '%s' not found in struct '%s'\n", part, currentStructType))
				return
			}
			fieldOffset := fieldIdx * 8

			if i < len(parts)-1 {
				sec.WriteString(fmt.Sprintf("    mov r12, [r12 + %d]\n", fieldOffset))
				sec.WriteString("    cmp r12, 0\n")
				sec.WriteString("    je _runtime_null_pointer_error\n")
				currentStructType = structDef.Types[fieldIdx]
			} else {
				if cg.isStringExpr(n.Value) {
					sec.WriteString(fmt.Sprintf("    mov [r12 + %d], rax\n", fieldOffset))
				} else {
					valType := cg.tc.Check(n.Value, cg.env)
					if valType == typechecker.STRING_TYPE || valType == typechecker.ARRAY_TYPE || strings.HasPrefix(string(valType), "ҚҰРЫЛЫМ_") {
						sec.WriteString(fmt.Sprintf("    mov [r12 + %d], rax\n", fieldOffset))
					} else {
						sec.WriteString(fmt.Sprintf("    movsd [r12 + %d], xmm0\n", fieldOffset))
					}
				}
			}
		}

	case *parser.FunctionStatement:
		// Nested function — already handled in top-level pass, skip here

	case *parser.ExpressionStatement:
		cg.genExpression(n.Expression, sec)
	}
}

// ---------------------------------------------------------------------------
// Variable assignment
// ---------------------------------------------------------------------------

func (cg *CppGenerator) genVarAssign(n *parser.VarAssignStatement, sec *strings.Builder) {
	sec.WriteString(fmt.Sprintf("    ; %s болсын\n", n.Name.Value))
	cg.genExpression(n.Value, sec)
	off := cg.allocVar(n.Name.Value)

	exprType := cg.tc.Check(n.Value, cg.env)
	existingType, ok := cg.env.Get(n.Name.Value)
	if !ok {
		cg.env.Set(n.Name.Value, exprType)
		existingType = exprType
	}

	// Strings, bools, arrays and structs live in rax (pointers or bools); numbers/floats in xmm0.
	// We use the existing declared variable type to decide which register to save.
	if existingType == typechecker.STRING_TYPE || existingType == typechecker.ARRAY_TYPE || strings.HasPrefix(string(existingType), "ҚҰРЫЛЫМ_") {
		sec.WriteString(fmt.Sprintf("    mov [rbp%+d], rax\n", off))
		cg.varIsString[n.Name.Value] = true
	} else if existingType == typechecker.NUMBER_TYPE || existingType == typechecker.INT_TYPE || existingType == typechecker.BOOL_TYPE {
		// Bool, int or float
		if existingType == typechecker.BOOL_TYPE {
			sec.WriteString(fmt.Sprintf("    mov [rbp%+d], rax\n", off))
			cg.varIsString[n.Name.Value] = true
		} else {
			sec.WriteString(fmt.Sprintf("    movsd [rbp%+d], xmm0\n", off))
			cg.varIsString[n.Name.Value] = false
		}
	} else {
		// Fallback switch on value expression type
		switch n.Value.(type) {
		case *parser.StringLiteral, *parser.StrConcatExpression,
			*parser.StrEqExpression,
			*parser.ToStrExpression, *parser.CharAtExpression,
			*parser.BoolLiteral,
			*parser.FileReadExpression,
			*parser.ArrayLiteral:
			sec.WriteString(fmt.Sprintf("    mov [rbp%+d], rax\n", off))
			cg.varIsString[n.Name.Value] = true
		case *parser.NumberLiteral, *parser.PostfixExpression, *parser.CharCodeExpression,
			*parser.StrLenExpression, *parser.LengthExpression:
			sec.WriteString(fmt.Sprintf("    movsd [rbp%+d], xmm0\n", off))
			cg.varIsString[n.Name.Value] = false
		default:
			if exprType == typechecker.STRING_TYPE || exprType == typechecker.ARRAY_TYPE || strings.HasPrefix(string(exprType), "ҚҰРЫЛЫМ_") || cg.isStringExpr(n.Value) {
				sec.WriteString(fmt.Sprintf("    mov [rbp%+d], rax\n", off))
				cg.varIsString[n.Name.Value] = true
			} else {
				sec.WriteString(fmt.Sprintf("    movsd [rbp%+d], xmm0\n", off))
				cg.varIsString[n.Name.Value] = false
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Print statement
// ---------------------------------------------------------------------------

func (cg *CppGenerator) isStringExpr(e parser.Expression) bool {
	switch val := e.(type) {
	case *parser.StringLiteral, *parser.StrConcatExpression,
		*parser.ToStrExpression, *parser.CharAtExpression,
		*parser.FileReadExpression, *parser.InputExpression:
		return true
	case *parser.Identifier:
		return cg.varIsString[val.Value]
	case *parser.CallExpression:
		if sig, ok := cg.funcs[val.Function]; ok {
			return sig.ReturnType == typechecker.STRING_TYPE || sig.ReturnType == typechecker.ARRAY_TYPE || strings.HasPrefix(string(sig.ReturnType), "ҚҰРЫЛЫМ_")
		}
	case *parser.StructFieldAccessExpression:
		t, ok := cg.env.Get(val.StructName)
		if ok && strings.HasPrefix(string(t), "ҚҰРЫЛЫМ_") {
			structName := string(t)[len("ҚҰРЫЛЫМ_"):]
			parts := strings.Split(val.Field, ".")
			currentStructType := structName
			var lastFieldType string
			for _, part := range parts {
				structDef, okDef := cg.structs[currentStructType]
				if !okDef {
					return false
				}
				fieldIdx := -1
				for idx, f := range structDef.Fields {
					if f == part {
						fieldIdx = idx
						break
					}
				}
				if fieldIdx == -1 {
					return false
				}
				lastFieldType = structDef.Types[fieldIdx]
				currentStructType = lastFieldType
			}
			return lastFieldType == "МӘТІН" || lastFieldType == "ТІЗІМ" || (lastFieldType != "САН" && lastFieldType != "БҮТІН" && lastFieldType != "АҚИҚАТ" && lastFieldType != "БАЙТ")
		}
	}
	return false
}

func (cg *CppGenerator) genPrint(n *parser.PrintStatement, sec *strings.Builder) {
	sec.WriteString("    ; жазу\n")

	if cg.isStringExpr(n.Value) {
		// ---- Print a string-returning expression (rax = char*) ----
		cg.genExpression(n.Value, sec)
		if cg.platform == PlatformLinux {
			sec.WriteString("    mov rdi, fmt_str\n")
			sec.WriteString("    mov rsi, rax\n")
			sec.WriteString("    xor rax, rax\n")
			sec.WriteString("    call printf\n")
		} else {
			sec.WriteString("    mov rdx, rax\n")
			sec.WriteString("    lea rcx, [fmt_str]\n")
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call printf\n")
			sec.WriteString("    add rsp, 32\n")
		}
		return
	}

	switch val := n.Value.(type) {
	case *parser.NumberLiteral:
		floatLabel := cg.newFloat(val.Value)
		if cg.platform == PlatformLinux {
			sec.WriteString(fmt.Sprintf("    movsd xmm0, [%s]\n", floatLabel))
			sec.WriteString("    mov rdi, fmt_float\n")
			sec.WriteString("    mov rax, 1\n")
			sec.WriteString("    call printf\n")
		} else {
			sec.WriteString(fmt.Sprintf("    movsd xmm1, [%s]\n", floatLabel))
			sec.WriteString("    movq rdx, xmm1\n")
			sec.WriteString("    lea rcx, [fmt_float]\n")
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call printf\n")
			sec.WriteString("    add rsp, 32\n")
		}

	case *parser.IntLiteral:
		if cg.platform == PlatformLinux {
			sec.WriteString(fmt.Sprintf("    mov rsi, %d\n", val.Value))
			sec.WriteString("    mov rdi, fmt_int\n")
			sec.WriteString("    xor rax, rax\n")
			sec.WriteString("    call printf\n")
		} else {
			sec.WriteString(fmt.Sprintf("    mov rdx, %d\n", val.Value))
			sec.WriteString("    lea rcx, [fmt_int]\n")
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call printf\n")
			sec.WriteString("    add rsp, 32\n")
		}

	case *parser.Identifier:
		off, ok := cg.getVarOffset(val.Value)
		if !ok {
			sec.WriteString(fmt.Sprintf("    ; ERROR: undeclared var %s\n", val.Value))
			return
		}
		if cg.varIsString[val.Value] {
			// String variable: load pointer from rax slot
			sec.WriteString(fmt.Sprintf("    mov rax, [rbp%+d]\n", off))
			if cg.platform == PlatformLinux {
				sec.WriteString("    mov rsi, rax\n")
				sec.WriteString("    lea rdi, [fmt_str]\n")
				sec.WriteString("    xor rax, rax\n")
				sec.WriteString("    call printf\n")
			} else {
				sec.WriteString("    mov rdx, rax\n")
				sec.WriteString("    lea rcx, [fmt_str]\n")
				sec.WriteString("    sub rsp, 32\n")
				sec.WriteString("    call printf\n")
				sec.WriteString("    add rsp, 32\n")
			}
		} else {
			if cg.platform == PlatformLinux {
				sec.WriteString(fmt.Sprintf("    movsd xmm0, [rbp%+d]\n", off))
				sec.WriteString("    mov rdi, fmt_float\n")
				sec.WriteString("    mov rax, 1\n")
				sec.WriteString("    call printf\n")
			} else {
				sec.WriteString(fmt.Sprintf("    movsd xmm1, [rbp%+d]\n", off))
				sec.WriteString("    movq rdx, xmm1\n")
				sec.WriteString("    lea rcx, [fmt_float]\n")
				sec.WriteString("    sub rsp, 32\n")
				sec.WriteString("    call printf\n")
				sec.WriteString("    add rsp, 32\n")
			}
		}

	case *parser.StructFieldAccessExpression:
		cg.genExpression(val, sec)
		if cg.platform == PlatformLinux {
			sec.WriteString("    mov rdi, fmt_float\n")
			sec.WriteString("    mov rax, 1\n")
			sec.WriteString("    call printf\n")
		} else {
			sec.WriteString("    movsd xmm1, xmm0\n")
			sec.WriteString("    movq rdx, xmm1\n")
			sec.WriteString("    lea rcx, [fmt_float]\n")
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call printf\n")
			sec.WriteString("    add rsp, 32\n")
		}

	default:
		// General expression — eval into xmm0/rax and print as number
		cg.genExpression(n.Value, sec)
		if cg.platform == PlatformLinux {
			sec.WriteString("    mov rdi, fmt_float\n")
			sec.WriteString("    mov rax, 1\n")
			sec.WriteString("    call printf\n")
		} else {
			sec.WriteString("    movsd xmm1, xmm0\n")
			sec.WriteString("    movq rdx, xmm1\n")
			sec.WriteString("    lea rcx, [fmt_float]\n")
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call printf\n")
			sec.WriteString("    add rsp, 32\n")
		}
	}
	if cg.platform == PlatformWindows {
		sec.WriteString("    xor rcx, rcx\n")
		sec.WriteString("    sub rsp, 32\n")
		sec.WriteString("    call fflush\n")
		sec.WriteString("    add rsp, 32\n")
	}
}

// ---------------------------------------------------------------------------
// If statement
// ---------------------------------------------------------------------------

func (cg *CppGenerator) genIf(n *parser.IfStatement, sec *strings.Builder) {
	elseLabel := cg.newLabel("else")
	endLabel := cg.newLabel("endif")

	sec.WriteString("    ; егер\n")
	cg.genCondition(n.Condition, sec, elseLabel)

	// Consequence
	for _, stmt := range n.Consequence.Statements {
		cg.genStatement(stmt, sec)
	}

	if n.Alternative != nil {
		sec.WriteString(fmt.Sprintf("    jmp %s\n", endLabel))
	}

	sec.WriteString(elseLabel + ":\n")

	if n.Alternative != nil {
		for _, stmt := range n.Alternative.Statements {
			cg.genStatement(stmt, sec)
		}
		sec.WriteString(endLabel + ":\n")
	}
}

// ---------------------------------------------------------------------------
// While statement
// ---------------------------------------------------------------------------

func (cg *CppGenerator) genWhile(n *parser.WhileStatement, sec *strings.Builder) {
	startLabel := cg.newLabel("while")
	endLabel := cg.newLabel("endwhile")

	cg.loopStartLabels = append(cg.loopStartLabels, startLabel)
	cg.loopEndLabels = append(cg.loopEndLabels, endLabel)

	sec.WriteString("    ; әзірше\n")
	sec.WriteString(startLabel + ":\n")
	cg.genCondition(n.Condition, sec, endLabel)

	for _, stmt := range n.Body.Statements {
		cg.genStatement(stmt, sec)
	}

	sec.WriteString(fmt.Sprintf("    jmp %s\n", startLabel))
	sec.WriteString(endLabel + ":\n")

	cg.loopStartLabels = cg.loopStartLabels[:len(cg.loopStartLabels)-1]
	cg.loopEndLabels = cg.loopEndLabels[:len(cg.loopEndLabels)-1]
}

// ---------------------------------------------------------------------------
// Condition evaluation — jumps to failLabel if condition is false
// ---------------------------------------------------------------------------

func (cg *CppGenerator) genCondition(cond parser.Expression, sec *strings.Builder, failLabel string) {
	pe, ok := cond.(*parser.PostfixExpression)
	if !ok {
		// Simple bool or identifier — evaluate and check
		cg.genExpression(cond, sec)
		sec.WriteString("    test rax, rax\n")
		sec.WriteString(fmt.Sprintf("    jz %s\n", failLabel))
		return
	}

	// Evaluate left and right into xmm registers
	cg.genExpression(pe.Left, sec)
	sec.WriteString("    movsd xmm1, xmm0\n") // left in xmm1
	cg.genExpression(pe.Right, sec)
	// right in xmm0
	// Compare: comisd xmm1, xmm0 -> sets flags based on (xmm1 op xmm0)
	sec.WriteString("    comisd xmm1, xmm0\n")

	switch pe.Operator {
	case "үлкен": // xmm1 > xmm0: jump to fail if <=
		sec.WriteString(fmt.Sprintf("    jbe %s\n", failLabel))
	case "кіші": // xmm1 < xmm0: jump to fail if >=
		sec.WriteString(fmt.Sprintf("    jae %s\n", failLabel))
	case "тең": // xmm1 == xmm0: jump to fail if !=
		sec.WriteString(fmt.Sprintf("    jne %s\n", failLabel))
	case "тең_емес": // xmm1 != xmm0: jump to fail if ==
		sec.WriteString(fmt.Sprintf("    je %s\n", failLabel))
	case "үлкен_тең": // xmm1 >= xmm0: jump to fail if <
		sec.WriteString(fmt.Sprintf("    jb %s\n", failLabel))
	case "кіші_тең": // xmm1 <= xmm0: jump to fail if >
		sec.WriteString(fmt.Sprintf("    ja %s\n", failLabel))
	default:
		// AND/OR: evaluate as bool
		cg.genExpression(cond, sec)
		sec.WriteString("    test rax, rax\n")
		sec.WriteString(fmt.Sprintf("    jz %s\n", failLabel))
	}
}

// ---------------------------------------------------------------------------
// Expression evaluation
// Result: floats/numbers in xmm0, strings in rax (pointer), bools in rax
// ---------------------------------------------------------------------------

func (cg *CppGenerator) genExpression(node parser.Expression, sec *strings.Builder) {
	switch n := node.(type) {

	case *parser.NumberLiteral:
		label := cg.newFloat(n.Value)
		sec.WriteString(fmt.Sprintf("    movsd xmm0, [%s]\n", label))

	case *parser.IntLiteral:
		// Temporary compatibility shim: evaluate into xmm0 as float for math ops.
		sec.WriteString(fmt.Sprintf("    mov rax, %d\n", n.Value))
		sec.WriteString("    cvtsi2sd xmm0, rax\n")

	case *parser.StructCreateExpression:
		structDef, ok := cg.structs[n.StructName]
		if !ok {
			sec.WriteString(fmt.Sprintf("    ; ERROR: struct definition '%s' not found\n", n.StructName))
			return
		}
		numFields := len(structDef.Fields)
		size := numFields * 8
		if size == 0 {
			size = 8
		}
		sec.WriteString(fmt.Sprintf("    ; жасау %s\n", n.StructName))
		if cg.platform == PlatformWindows {
			sec.WriteString(fmt.Sprintf("    mov rcx, %d\n", size))
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call malloc\n")
			sec.WriteString("    add rsp, 32\n")
		} else {
			sec.WriteString(fmt.Sprintf("    mov rdi, %d\n", size))
			sec.WriteString("    call malloc\n")
		}

	case *parser.StructFieldAccessExpression:
		var structName string
		if n.Target != nil {
			sec.WriteString(fmt.Sprintf("    ; %s оқу (nested)\n", n.Field))
			cg.genExpression(n.Target, sec)
			sec.WriteString("    mov r12, rax\n")
			targetType := cg.tc.Check(n.Target, cg.env)
			structName = string(targetType)[len("ҚҰРЫЛЫМ_"):]
		} else {
			sec.WriteString(fmt.Sprintf("    ; %s.%s оқу\n", n.StructName, n.Field))
			off, ok := cg.getVarOffset(n.StructName)
			if !ok {
				sec.WriteString(fmt.Sprintf("    ; ERROR: undeclared struct '%s'\n", n.StructName))
				return
			}
			t, ok := cg.env.Get(n.StructName)
			if !ok || !strings.HasPrefix(string(t), "ҚҰРЫЛЫМ_") {
				sec.WriteString(fmt.Sprintf("    ; ERROR: '%s' is not a struct\n", n.StructName))
				return
			}
			structName = string(t)[len("ҚҰРЫЛЫМ_"):]
			sec.WriteString(fmt.Sprintf("    mov r12, [rbp%+d]\n", off))
		}
		parts := strings.Split(n.Field, ".")
		sec.WriteString("    cmp r12, 0\n")
		sec.WriteString("    je _runtime_null_pointer_error\n")

		currentStructType := structName
		for i, part := range parts {
			structDef, ok := cg.structs[currentStructType]
			if !ok {
				sec.WriteString(fmt.Sprintf("    ; ERROR: struct definition '%s' not found\n", currentStructType))
				return
			}
			fieldIdx := -1
			for idx, f := range structDef.Fields {
				if f == part {
					fieldIdx = idx
					break
				}
			}
			if fieldIdx == -1 {
				sec.WriteString(fmt.Sprintf("    ; ERROR: field '%s' not found in struct '%s'\n", part, currentStructType))
				return
			}
			fieldOffset := fieldIdx * 8

			if i < len(parts)-1 {
				sec.WriteString(fmt.Sprintf("    mov r12, [r12 + %d]\n", fieldOffset))
				sec.WriteString("    cmp r12, 0\n")
				sec.WriteString("    je _runtime_null_pointer_error\n")
				currentStructType = structDef.Types[fieldIdx]
			} else {
				sec.WriteString(fmt.Sprintf("    mov rax, [r12 + %d]\n", fieldOffset))
				sec.WriteString(fmt.Sprintf("    movsd xmm0, [r12 + %d]\n", fieldOffset))
			}
		}

	case *parser.StringLiteral:
		label := cg.newStr(n.Value)
		sec.WriteString(fmt.Sprintf("    lea rax, [%s]\n", label))

	case *parser.BoolLiteral:
		if n.Value {
			sec.WriteString("    mov rax, 1\n")
		} else {
			sec.WriteString("    xor rax, rax\n")
		}

	case *parser.Identifier:
		off, ok := cg.getVarOffset(n.Value)
		if !ok {
			sec.WriteString(fmt.Sprintf("    ; ERROR: undeclared var '%s'\n", n.Value))
			return
		}
		sec.WriteString(fmt.Sprintf("    movsd xmm0, [rbp%+d]\n", off))
		// Also load as int in rax (for bool/int vars)
		sec.WriteString(fmt.Sprintf("    mov rax, [rbp%+d]\n", off))

	case *parser.PostfixExpression:
		cg.genBinaryOp(n, sec)

	case *parser.UnaryExpression:
		if n.Operator == "емес" {
			cg.genExpression(n.Right, sec)
			sec.WriteString("    xor rax, 1\n") // flip boolean
		}

	case *parser.CallExpression:
		cg.genCallExpr(n, sec)

	case *parser.StrConcatExpression:
		// StrConcatExpression using dynamic allocation (malloc) to avoid scratch_buf collision
		cg.genExpression(n.Left, sec)
		sec.WriteString("    sub rsp, 16            ; preserve left ptr on 16-byte aligned stack\n")
		sec.WriteString("    mov [rsp], rax\n")
		cg.genExpression(n.Right, sec)
		sec.WriteString("    sub rsp, 16            ; preserve right ptr on 16-byte aligned stack\n")
		sec.WriteString("    mov [rsp], rax\n")

		if cg.platform == PlatformWindows {
			// strlen(left)
			sec.WriteString("    mov rcx, [rsp+16]\n")
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call strlen\n")
			sec.WriteString("    add rsp, 32\n")
			sec.WriteString("    sub rsp, 16            ; save left_len\n")
			sec.WriteString("    mov [rsp], rax\n")

			// strlen(right)
			sec.WriteString("    mov rcx, [rsp+16]\n")
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call strlen\n")
			sec.WriteString("    add rsp, 32\n")

			// total_size = left_len + right_len + 1
			sec.WriteString("    add rax, [rsp]         ; total = left_len + right_len\n")
			sec.WriteString("    inc rax                ; total + 1\n")

			// malloc(total_size)
			sec.WriteString("    mov rcx, rax\n")
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call malloc\n")
			sec.WriteString("    add rsp, 32\n")
			sec.WriteString("    sub rsp, 16            ; save allocated ptr\n")
			sec.WriteString("    mov [rsp], rax\n")

			// strcpy(dest, left)
			sec.WriteString("    mov rcx, [rsp]         ; dest\n")
			sec.WriteString("    mov rdx, [rsp+48]      ; left_ptr\n")
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call strcpy\n")
			sec.WriteString("    add rsp, 32\n")

			// strcat(dest, right)
			sec.WriteString("    mov rcx, [rsp]         ; dest\n")
			sec.WriteString("    mov rdx, [rsp+32]      ; right_ptr\n")
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call strcat\n")
			sec.WriteString("    add rsp, 32\n")
		} else {
			// Linux: strlen(left)
			sec.WriteString("    mov rdi, [rsp+16]\n")
			sec.WriteString("    call strlen\n")
			sec.WriteString("    sub rsp, 16\n")
			sec.WriteString("    mov [rsp], rax\n")

			// strlen(right)
			sec.WriteString("    mov rdi, [rsp+16]\n")
			sec.WriteString("    call strlen\n")

			// total_size = left_len + right_len + 1
			sec.WriteString("    add rax, [rsp]\n")
			sec.WriteString("    inc rax\n")

			// malloc(total_size)
			sec.WriteString("    mov rdi, rax\n")
			sec.WriteString("    call malloc\n")
			sec.WriteString("    sub rsp, 16\n")
			sec.WriteString("    mov [rsp], rax\n")

			// strcpy(dest, left)
			sec.WriteString("    mov rdi, [rsp]\n")
			sec.WriteString("    mov rsi, [rsp+48]\n")
			sec.WriteString("    call strcpy\n")

			// strcat(dest, right)
			sec.WriteString("    mov rdi, [rsp]\n")
			sec.WriteString("    mov rsi, [rsp+32]\n")
			sec.WriteString("    call strcat\n")
		}

		sec.WriteString("    mov rax, [rsp]         ; return allocated ptr\n")
		sec.WriteString("    add rsp, 64            ; restore stack\n")

	case *parser.StrLenExpression:
		// strlen(str) -> result in rax, then convert to float in xmm0
		cg.genExpression(n.Value, sec)
		if cg.platform == PlatformWindows {
			sec.WriteString("    mov rcx, rax\n")
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call strlen\n")
			sec.WriteString("    add rsp, 32\n")
		} else {
			sec.WriteString("    mov rdi, rax\n")
			sec.WriteString("    call strlen\n")
		}
		sec.WriteString("    cvtsi2sd xmm0, rax\n") // int -> float for Butaq

	case *parser.StrEqExpression:
		// strcmp(left, right) == 0 means equal -> rax = 1/0
		cg.genExpression(n.Left, sec)
		sec.WriteString("    mov r10, rax\n")
		cg.genExpression(n.Right, sec)
		if cg.platform == PlatformWindows {
			sec.WriteString("    mov rcx, r10\n")
			sec.WriteString("    mov rdx, rax\n")
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call strcmp\n")
			sec.WriteString("    add rsp, 32\n")
		} else {
			sec.WriteString("    mov rdi, r10\n")
			sec.WriteString("    mov rsi, rax\n")
			sec.WriteString("    call strcmp\n")
		}
		sec.WriteString("    test rax, rax\n")
		sec.WriteString("    sete al\n")
		sec.WriteString("    movzx rax, al\n")

	case *parser.ToStrExpression:
		// sprintf(numstr_buf, "%g", num) -> rax = ptr to numstr_buf
		// Win64 vararg: float must be in BOTH xmm register AND corresponding int register
		cg.genExpression(n.Value, sec)
		if cg.platform == PlatformWindows {
			// xmm0 = float value; Win64 vararg: float must go in BOTH xmm AND int reg
			sec.WriteString("    movsd xmm2, xmm0\n")      // xmm2 = 3rd arg (the float)
			sec.WriteString("    movq r8, xmm2\n")         // r8   = 3rd arg (int shadow)
			sec.WriteString("    lea rdx, [fmt_numstr]\n") // rdx  = 2nd arg (format)
			sec.WriteString("    lea rcx, [numstr_buf]\n") // rcx  = 1st arg (buffer)
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call sprintf\n")
			sec.WriteString("    add rsp, 32\n")
		} else {
			sec.WriteString("    lea rdi, [numstr_buf]\n")
			sec.WriteString("    lea rsi, [fmt_numstr]\n")
			sec.WriteString("    mov rax, 1\n")
			sec.WriteString("    call sprintf\n")
		}
		sec.WriteString("    lea rax, [numstr_buf]\n")

	case *parser.LengthExpression:
		// Array length (ұзындық) or string length.
		// Determine type from typechecker.
		exprType := cg.tc.Check(n.Value, cg.env)
		cg.genExpression(n.Value, sec)
		if exprType == typechecker.ARRAY_TYPE {
			// Array pointer is in rax, length is at [rax]
			sec.WriteString("    mov r10, [rax]\n")
			sec.WriteString("    cvtsi2sd xmm0, r10\n")
		} else {
			// Assume String
			if cg.platform == PlatformWindows {
				sec.WriteString("    mov rcx, rax\n")
				sec.WriteString("    sub rsp, 32\n")
				sec.WriteString("    call strlen\n")
				sec.WriteString("    add rsp, 32\n")
			} else {
				sec.WriteString("    mov rdi, rax\n")
				sec.WriteString("    call strlen\n")
			}
			sec.WriteString("    cvtsi2sd xmm0, rax\n")
		}

	case *parser.CharCodeExpression:
		cg.genExpression(n.Value, sec)
		// rax = char*
		sec.WriteString("    movzx eax, byte [rax]\n")
		sec.WriteString("    cvtsi2sd xmm0, eax\n")

	case *parser.CharAtExpression:
		cg.genExpression(n.Str, sec)
		sec.WriteString("    mov r10, rax\n") // string ptr
		cg.genExpression(n.Index, sec)
		// xmm0 = index float
		sec.WriteString("    cvttsd2si r11, xmm0\n") // r11 = int index
		sec.WriteString("    movzx eax, byte [r10 + r11]\n")
		sec.WriteString("    lea rdx, [char_buf]\n")
		sec.WriteString("    mov [rdx], al\n")
		sec.WriteString("    mov byte [rdx+1], 0\n")
		sec.WriteString("    mov rax, rdx\n")

	case *parser.FileReadExpression:
		cg.genFileRead(n, sec)

	case *parser.InputExpression:
		// кіру — read one line from stdin via scanf("%[^\n]", input_buf)
		sec.WriteString("    ; кіру — stdin-нен жол оқу\n")
		if cg.platform == PlatformWindows {
			sec.WriteString("    lea rcx, [fmt_input]\n")
			sec.WriteString("    lea rdx, [input_buf]\n")
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call scanf\n")
			sec.WriteString("    add rsp, 32\n")
		} else {
			sec.WriteString("    lea rdi, [fmt_input]\n")
			sec.WriteString("    lea rsi, [input_buf]\n")
			sec.WriteString("    xor rax, rax\n")
			sec.WriteString("    call scanf\n")
		}
		sec.WriteString("    lea rax, [input_buf]\n") // return pointer to input line

	case *parser.ArrayLiteral:
		// Heap-allocate array: length prefix (8 bytes) + each element (8 bytes float64).
		// Layout: [length_i64, element0_f64, element1_f64, ...]
		// rax = pointer to array base (points to length_i64) after malloc.
		nElems := len(n.Elements)
		allocSize := (nElems * 8) + 8
		sec.WriteString(fmt.Sprintf("    ; тізім — %d элемент, malloc(%d)\n", nElems, allocSize))

		if cg.platform == PlatformWindows {
			sec.WriteString(fmt.Sprintf("    mov rcx, %d\n", allocSize))
			// Windows stack align shadow space
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call malloc\n")
			sec.WriteString("    add rsp, 32\n")
		} else {
			sec.WriteString(fmt.Sprintf("    mov rdi, %d\n", allocSize))
			sec.WriteString("    call malloc\n")
		}
		sec.WriteString("    mov r12, rax\n") // r12 = array base
		// Store length
		sec.WriteString(fmt.Sprintf("    mov qword [r12], %d\n", nElems))

		// Fill each element
		for i, elem := range n.Elements {
			sec.WriteString("    push r12\n")
			cg.genExpression(elem, sec)
			sec.WriteString("    pop r12\n")
			t := cg.tc.Check(elem, cg.env)
			if t == typechecker.STRING_TYPE || t == typechecker.ARRAY_TYPE || strings.HasPrefix(string(t), "ҚҰРЫЛЫМ_") {
				sec.WriteString(fmt.Sprintf("    mov [r12 + %d], rax\n", 8+i*8))
			} else {
				sec.WriteString(fmt.Sprintf("    movsd [r12 + %d], xmm0\n", 8+i*8))
			}
		}
		sec.WriteString("    mov rax, r12\n") // return pointer

	case *parser.IndexExpression:
		// arr idx алу — load float element at index
		cg.genExpression(n.Left, sec)
		sec.WriteString("    mov r12, rax\n") // r12 = array base pointer
		sec.WriteString("    cmp r12, 0\n")
		sec.WriteString("    je _runtime_null_pointer_error\n")
		sec.WriteString("    push r12\n")
		cg.genExpression(n.Index, sec)
		sec.WriteString("    pop r12\n")
		// xmm0 = index as float
		sec.WriteString("    cvttsd2si r13, xmm0\n") // r13 = int index
		// Check bounds
		sec.WriteString("    cmp r13, 0\n")
		sec.WriteString("    jl _runtime_array_bounds_error\n")
		sec.WriteString("    cmp r13, [r12]\n")
		sec.WriteString("    jge _runtime_array_bounds_error\n")

		sec.WriteString("    imul r13, 8\n")         // byte offset = index * 8
		sec.WriteString("    add r13, 8\n")          // skip 8-byte length prefix
		sec.WriteString("    movsd xmm0, [r12 + r13]\n")
		// also load as rax for possible pointer use
		sec.WriteString("    movq rax, xmm0\n")
	}
}

// ---------------------------------------------------------------------------
// File I/O
// ---------------------------------------------------------------------------

// genFileRead — reads entire file into scratch_buf, returns rax = ptr to buf
// Strategy: fopen → fseek(END) → ftell → fseek(0) → fread → fclose
func (cg *CppGenerator) genFileRead(n *parser.FileReadExpression, sec *strings.Builder) {
	sec.WriteString("    ; файл_оқу\n")
	// Evaluate path expression → rax = char* path
	cg.genExpression(n.Path, sec)

	if cg.platform == PlatformWindows {
		// fopen(path, "r")
		sec.WriteString("    mov r12, rax\n")      // save path ptr
		sec.WriteString("    mov rcx, r12\n")      // arg1: path
		sec.WriteString("    lea rdx, [file_r]\n") // arg2: mode "r"
		sec.WriteString("    sub rsp, 32\n")
		sec.WriteString("    call fopen\n")
		sec.WriteString("    add rsp, 32\n")
		sec.WriteString("    mov r13, rax\n") // r13 = FILE*
		// Check if file opened
		sec.WriteString("    test r13, r13\n")
		sec.WriteString("    jz .fread_fail\n") // skip read if NULL
		// fseek(FILE*, 0, SEEK_END=2)
		sec.WriteString("    mov rcx, r13\n")
		sec.WriteString("    xor rdx, rdx\n")
		sec.WriteString("    mov r8, 2\n")
		sec.WriteString("    sub rsp, 32\n")
		sec.WriteString("    call fseek\n")
		sec.WriteString("    add rsp, 32\n")
		// ftell(FILE*) → rax = file size
		sec.WriteString("    mov rcx, r13\n")
		sec.WriteString("    sub rsp, 32\n")
		sec.WriteString("    call ftell\n")
		sec.WriteString("    add rsp, 32\n")
		sec.WriteString("    mov r14, rax\n") // r14 = size
		// fseek(FILE*, 0, SEEK_SET=0)
		sec.WriteString("    mov rcx, r13\n")
		sec.WriteString("    xor rdx, rdx\n")
		sec.WriteString("    xor r8, r8\n")
		sec.WriteString("    sub rsp, 32\n")
		sec.WriteString("    call fseek\n")
		sec.WriteString("    add rsp, 32\n")
		// fread(scratch_buf, 1, size, FILE*)
		sec.WriteString("    lea rcx, [scratch_buf]\n")
		sec.WriteString("    mov rdx, 1\n")
		sec.WriteString("    mov r8, r14\n")
		sec.WriteString("    mov r9, r13\n")
		sec.WriteString("    sub rsp, 32\n")
		sec.WriteString("    call fread\n")
		sec.WriteString("    add rsp, 32\n")
		// null-terminate: scratch_buf[size] = 0
		sec.WriteString("    lea rax, [scratch_buf]\n")
		sec.WriteString("    mov byte [rax + r14], 0\n")
		// fclose(FILE*)
		sec.WriteString("    mov rcx, r13\n")
		sec.WriteString("    sub rsp, 32\n")
		sec.WriteString("    call fclose\n")
		sec.WriteString("    add rsp, 32\n")
		sec.WriteString("    lea rax, [scratch_buf]\n")
		sec.WriteString("    jmp .fread_done\n")
		sec.WriteString(".fread_fail:\n")
		sec.WriteString("    xor rax, rax\n") // return NULL on error
		sec.WriteString(".fread_done:\n")
	} else {
		// Linux: same but System V ABI
		sec.WriteString("    mov r12, rax\n")
		sec.WriteString("    mov rdi, r12\n")
		sec.WriteString("    lea rsi, [file_r]\n")
		sec.WriteString("    call fopen\n")
		sec.WriteString("    mov r13, rax\n")
		sec.WriteString("    test r13, r13\n")
		sec.WriteString("    jz .fread_fail\n")
		sec.WriteString("    mov rdi, r13\n")
		sec.WriteString("    xor rsi, rsi\n")
		sec.WriteString("    mov rdx, 2\n")
		sec.WriteString("    call fseek\n")
		sec.WriteString("    mov rdi, r13\n")
		sec.WriteString("    call ftell\n")
		sec.WriteString("    mov r14, rax\n")
		sec.WriteString("    mov rdi, r13\n")
		sec.WriteString("    xor rsi, rsi\n")
		sec.WriteString("    xor rdx, rdx\n")
		sec.WriteString("    call fseek\n")
		sec.WriteString("    lea rdi, [scratch_buf]\n")
		sec.WriteString("    mov rsi, 1\n")
		sec.WriteString("    mov rdx, r14\n")
		sec.WriteString("    mov rcx, r13\n")
		sec.WriteString("    call fread\n")
		sec.WriteString("    lea rax, [scratch_buf]\n")
		sec.WriteString("    mov byte [rax + r14], 0\n")
		sec.WriteString("    mov rdi, r13\n")
		sec.WriteString("    call fclose\n")
		sec.WriteString("    lea rax, [scratch_buf]\n")
		sec.WriteString("    jmp .fread_done\n")
		sec.WriteString(".fread_fail:\n")
		sec.WriteString("    xor rax, rax\n")
		sec.WriteString(".fread_done:\n")
	}
}

// genFileWrite — writes content string to file path
// "path" content файл_жазу
func (cg *CppGenerator) genFileWrite(n *parser.FileWriteStatement, sec *strings.Builder) {
	sec.WriteString("    ; файл_жазу\n")
	// n.Path = file path, n.Content = data to write
	cg.genExpression(n.Path, sec)
	sec.WriteString("    mov r10, rax\n") // r10 = path
	cg.genExpression(n.Content, sec)
	sec.WriteString("    mov r11, rax\n") // r11 = content

	if cg.platform == PlatformWindows {
		// fopen(path, "w")
		sec.WriteString("    mov rcx, r10\n")
		sec.WriteString("    lea rdx, [file_w]\n")
		sec.WriteString("    sub rsp, 32\n")
		sec.WriteString("    call fopen\n")
		sec.WriteString("    add rsp, 32\n")
		sec.WriteString("    mov r14, rax\n") // r14 = FILE*
		sec.WriteString("    test r14, r14\n")
		sec.WriteString("    jz .fwrite_done\n")
		// strlen(content) → r15 = length
		sec.WriteString("    mov rcx, r11\n")
		sec.WriteString("    sub rsp, 32\n")
		sec.WriteString("    call strlen\n")
		sec.WriteString("    add rsp, 32\n")
		sec.WriteString("    mov r15, rax\n")
		// fwrite(content, 1, len, FILE*)
		sec.WriteString("    mov rcx, r11\n")
		sec.WriteString("    mov rdx, 1\n")
		sec.WriteString("    mov r8, r15\n")
		sec.WriteString("    mov r9, r14\n")
		sec.WriteString("    sub rsp, 32\n")
		sec.WriteString("    call fwrite\n")
		sec.WriteString("    add rsp, 32\n")
		// fclose
		sec.WriteString("    mov rcx, r14\n")
		sec.WriteString("    sub rsp, 32\n")
		sec.WriteString("    call fclose\n")
		sec.WriteString("    add rsp, 32\n")
		sec.WriteString(".fwrite_done:\n")
	} else {
		sec.WriteString("    mov rdi, r10\n")
		sec.WriteString("    lea rsi, [file_w]\n")
		sec.WriteString("    call fopen\n")
		sec.WriteString("    mov r14, rax\n")
		sec.WriteString("    test r14, r14\n")
		sec.WriteString("    jz .fwrite_done\n")
		sec.WriteString("    mov rdi, r11\n")
		sec.WriteString("    call strlen\n")
		sec.WriteString("    mov r15, rax\n")
		sec.WriteString("    mov rdi, r11\n")
		sec.WriteString("    mov rsi, 1\n")
		sec.WriteString("    mov rdx, r15\n")
		sec.WriteString("    mov rcx, r14\n")
		sec.WriteString("    call fwrite\n")
		sec.WriteString("    mov rdi, r14\n")
		sec.WriteString("    call fclose\n")
		sec.WriteString(".fwrite_done:\n")
	}
}

// ---------------------------------------------------------------------------
// Binary operations
// ---------------------------------------------------------------------------

func (cg *CppGenerator) genBinaryOp(n *parser.PostfixExpression, sec *strings.Builder) {
	if n.Operator == "және" || n.Operator == "немесе" {
		cg.genExpression(n.Left, sec)
		sec.WriteString("    sub rsp, 8\n")
		sec.WriteString("    mov [rsp], rax\n") // push left boolean (rax)

		cg.genExpression(n.Right, sec)
		sec.WriteString("    mov rcx, [rsp]\n") // pop left into rcx
		sec.WriteString("    add rsp, 8\n")

		if n.Operator == "және" {
			sec.WriteString("    and rax, rcx\n")
		} else {
			sec.WriteString("    or rax, rcx\n")
		}
		// Sync to xmm0 as float (0.0 or 1.0) so it works in prints and variable assignments
		sec.WriteString("    cvtsi2sd xmm0, rax\n")
		return
	}

	// Evaluate left, push to stack, evaluate right, then operate
	cg.genExpression(n.Left, sec)
	sec.WriteString("    sub rsp, 8\n")
	sec.WriteString("    movsd [rsp], xmm0\n") // push left

	cg.genExpression(n.Right, sec)
	sec.WriteString("    movsd xmm1, [rsp]\n") // pop left into xmm1
	sec.WriteString("    add rsp, 8\n")
	// now: xmm1 = left, xmm0 = right

	switch n.Operator {
	case "қосу":
		sec.WriteString("    addsd xmm1, xmm0\n")
		sec.WriteString("    movsd xmm0, xmm1\n")
	case "алу":
		sec.WriteString("    subsd xmm1, xmm0\n")
		sec.WriteString("    movsd xmm0, xmm1\n")
	case "көбейту":
		sec.WriteString("    mulsd xmm1, xmm0\n")
		sec.WriteString("    movsd xmm0, xmm1\n")
	case "бөлу":
		sec.WriteString("    xorpd xmm2, xmm2\n")
		sec.WriteString("    comisd xmm0, xmm2\n")
		sec.WriteString("    je _runtime_divide_by_zero_error\n")
		sec.WriteString("    divsd xmm1, xmm0\n")
		sec.WriteString("    movsd xmm0, xmm1\n")
	case "үлкен", "кіші", "тең", "тең_емес", "үлкен_тең", "кіші_тең":
		// Return bool result in rax and sync to xmm0
		sec.WriteString("    xor rax, rax\n")
		sec.WriteString("    comisd xmm1, xmm0\n")
		switch n.Operator {
		case "үлкен":
			sec.WriteString("    seta al\n")
		case "кіші":
			sec.WriteString("    setb al\n")
		case "тең":
			sec.WriteString("    sete al\n")
		case "тең_емес":
			sec.WriteString("    setne al\n")
		case "үлкен_тең":
			sec.WriteString("    setae al\n")
		case "кіші_тең":
			sec.WriteString("    setbe al\n")
		}
		sec.WriteString("    cvtsi2sd xmm0, rax\n")
	}
}

// ---------------------------------------------------------------------------
// Function call
// ---------------------------------------------------------------------------

var linuxArgRegs = []string{"rdi", "rsi", "rdx", "rcx", "r8", "r9"}
var linuxFloatRegs = []string{"xmm0", "xmm1", "xmm2", "xmm3", "xmm4", "xmm5"}
var winArgRegs = []string{"rcx", "rdx", "r8", "r9"}
var winFloatRegs = []string{"xmm0", "xmm1", "xmm2", "xmm3"}

func (cg *CppGenerator) genCallExpr(n *parser.CallExpression, sec *strings.Builder) {
	// Check standard library built-in functions
	switch n.Function {
	case "мәтін_ұзындығы":
		sec.WriteString("    ; мәтін_ұзындығы\n")
		cg.genExpression(n.Arguments[0], sec) // rax = char*
		if cg.platform == PlatformWindows {
			sec.WriteString("    mov rcx, rax\n")
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call strlen\n")
			sec.WriteString("    add rsp, 32\n")
		} else {
			sec.WriteString("    mov rdi, rax\n")
			sec.WriteString("    call strlen\n")
		}
		// Convert size in rax to double float in xmm0
		sec.WriteString("    cvtsi2sd xmm0, rax\n")
		return

	case "таңба":
		sec.WriteString("    ; таңба\n")
		cg.genExpression(n.Arguments[0], sec) // rax = int code
		sec.WriteString("    mov [char_buf], al\n")
		sec.WriteString("    mov byte [char_buf + 1], 0\n")
		sec.WriteString("    lea rax, [char_buf]\n")
		return

	case "бүтін":
		sec.WriteString("    ; бүтін\n")
		cg.genExpression(n.Arguments[0], sec) // xmm0 = float
		sec.WriteString("    cvttsd2si rax, xmm0\n")
		sec.WriteString("    cvtsi2sd xmm0, rax\n")
		return

	case "кездейсоқ":
		sec.WriteString("    ; кездейсоқ\n")
		if cg.platform == PlatformWindows {
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call rand\n")
			sec.WriteString("    add rsp, 32\n")
		} else {
			sec.WriteString("    call rand\n")
		}
		// Convert rand int in rax to double float in xmm0
		sec.WriteString("    cvtsi2sd xmm0, rax\n")
		return
	}

	// Check if it is a struct constructor
	if structDef, ok := cg.structs[n.Function]; ok {
		numFields := len(structDef.Fields)
		size := numFields * 8
		if size == 0 {
			size = 8
		}
		sec.WriteString(fmt.Sprintf("    ; инициализациялау (constructor) %s\n", n.Function))
		if cg.platform == PlatformWindows {
			sec.WriteString(fmt.Sprintf("    mov rcx, %d\n", size))
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call malloc\n")
			sec.WriteString("    add rsp, 32\n")
		} else {
			sec.WriteString(fmt.Sprintf("    mov rdi, %d\n", size))
			sec.WriteString("    call malloc\n")
		}
		// Save struct pointer on stack
		sec.WriteString("    push rax\n")
		// Evaluate arguments and assign to fields
		for i, arg := range n.Arguments {
			cg.genExpression(arg, sec)
			sec.WriteString("    mov r12, [rsp]\n") // peek struct pointer
			fieldType := structDef.Types[i]
			fieldOffset := i * 8
			if fieldType == "МӘТІН" || fieldType == "ТІЗІМ" || (fieldType != "САН" && fieldType != "БҮТІН" && fieldType != "АҚИҚАТ" && fieldType != "БАЙТ") {
				sec.WriteString(fmt.Sprintf("    mov [r12 + %d], rax\n", fieldOffset))
			} else {
				sec.WriteString(fmt.Sprintf("    movsd [r12 + %d], xmm0\n", fieldOffset))
			}
		}
		sec.WriteString("    pop rax\n") // return struct pointer in rax
		return
	}

	sec.WriteString(fmt.Sprintf("    ; шақыру %s\n", n.Function))

	funcLabel := strings.ReplaceAll(n.Function, ".", "_")
	nArgs := len(n.Arguments)
	if nArgs == 0 {
		if cg.platform == PlatformWindows {
			sec.WriteString("    sub rsp, 32\n")
		}
		sec.WriteString(fmt.Sprintf("    call %s\n", funcLabel))
		if cg.platform == PlatformWindows {
			sec.WriteString("    add rsp, 32\n")
		}
		return
	}

	// Step 1: Evaluate all arguments in order and push each onto the stack.
	// We track which args are string/pointer so we know which register lane to use.
	argIsStr := make([]bool, nArgs)
	for i, arg := range n.Arguments {
		cg.genExpression(arg, sec)
		argIsStr[i] = cg.isStringExpr(arg)
		if argIsStr[i] {
			// Pointer is in rax — push rax
			sec.WriteString("    sub rsp, 8\n")
			sec.WriteString("    mov [rsp], rax\n")
		} else {
			// Float is in xmm0 — push as qword
			sec.WriteString("    sub rsp, 8\n")
			sec.WriteString("    movsd [rsp], xmm0\n")
		}
	}

	// Step 2: Pop arguments in reverse order into the correct ABI registers.
	// Arguments were pushed left-to-right, so the stack is [ arg0, arg1, ..., argN-1 ] (argN-1 at top).
	for i := nArgs - 1; i >= 0; i-- {
		if argIsStr[i] {
			sec.WriteString("    pop rax\n") // pointer
			if cg.platform == PlatformLinux {
				if i < len(linuxArgRegs) {
					sec.WriteString(fmt.Sprintf("    mov %s, rax\n", linuxArgRegs[i]))
				}
			} else {
				if i < len(winArgRegs) {
					sec.WriteString(fmt.Sprintf("    mov %s, rax\n", winArgRegs[i]))
				}
			}
		} else {
			sec.WriteString("    movsd xmm0, [rsp]\n")
			sec.WriteString("    add rsp, 8\n")
			if cg.platform == PlatformLinux {
				if i < len(linuxFloatRegs) {
					sec.WriteString(fmt.Sprintf("    movsd %s, xmm0\n", linuxFloatRegs[i]))
				}
			} else {
				if i < len(winFloatRegs) {
					// Windows x64: float arg must be in BOTH the XMM reg AND the corresponding int reg
					sec.WriteString(fmt.Sprintf("    movsd %s, xmm0\n", winFloatRegs[i]))
					sec.WriteString(fmt.Sprintf("    movq %s, xmm0\n", winArgRegs[i]))
				}
			}
		}
	}

	if cg.platform == PlatformWindows {
		sec.WriteString("    sub rsp, 32\n")
	}
	sec.WriteString(fmt.Sprintf("    call %s\n", funcLabel))
	if cg.platform == PlatformWindows {
		sec.WriteString("    add rsp, 32\n")
	}
}

// ---------------------------------------------------------------------------
// User-defined function definition
// ---------------------------------------------------------------------------

func (cg *CppGenerator) genFunctionDef(fs *parser.FunctionStatement) {
	sec := &cg.funcSec

	// Save outer var offsets
	outerOffsets := cg.varOffsets
	outerOffset := cg.currentOffset
	outerFunc := cg.currentFunc
	outerIsString := cg.varIsString
	outerEnv := cg.env

	// New scope
	cg.varOffsets = make(map[string]int)
	cg.currentOffset = 0
	cg.currentFunc = fs.Name
	cg.varIsString = make(map[string]bool)
	cg.env = typechecker.NewEnclosedTypeEnv(cg.env)

	stackSize := cg.calcStackSize(fs.Body.Statements)
	// Add space for params
	stackSize += len(fs.Parameters) * 8
	stackSize = alignTo16(stackSize)

	funcLabel := strings.ReplaceAll(fs.Name, ".", "_")
	sec.WriteString(fmt.Sprintf("\n; функция %s\n", fs.Name))
	sec.WriteString(funcLabel + ":\n")
	sec.WriteString("    push rbp\n")
	sec.WriteString("    mov rbp, rsp\n")
	if stackSize > 0 {
		sec.WriteString(fmt.Sprintf("    sub rsp, %d\n", stackSize))
	}

	funcs := cg.tc.GetFuncs()
	sig := funcs[fs.Name]

	// Map parameters to stack
	for i, param := range fs.Parameters {
		off := cg.allocVar(param)
		isStr := false
		var pt typechecker.Type = typechecker.UNKNOWN
		if sig != nil && i < len(sig.ParamTypes) {
			pt = sig.ParamTypes[i]
			isStr = pt == typechecker.STRING_TYPE || pt == typechecker.ARRAY_TYPE || strings.HasPrefix(string(pt), "ҚҰРЫЛЫМ_")
		}
		cg.varIsString[param] = isStr
		cg.env.Set(param, pt)

		if isStr {
			// String/pointer arg: lives in integer arg register (rdi, rsi, ...)
			if cg.platform == PlatformLinux && i < len(linuxArgRegs) {
				sec.WriteString(fmt.Sprintf("    mov [rbp%+d], %s  ; param str %s\n", off, linuxArgRegs[i], param))
			} else if cg.platform == PlatformWindows && i < len(winArgRegs) {
				sec.WriteString(fmt.Sprintf("    mov [rbp%+d], %s  ; param str %s\n", off, winArgRegs[i], param))
			}
		} else {
			// Numeric arg: lives in xmm register
			if i < len(linuxFloatRegs) {
				sec.WriteString(fmt.Sprintf("    movsd [rbp%+d], %s  ; param num %s\n", off, linuxFloatRegs[i], param))
			}
		}
	}

	// Generate body
	for _, stmt := range fs.Body.Statements {
		cg.genStatement(stmt, sec)
	}

	// Default return
	sec.WriteString("    mov rsp, rbp\n")
	sec.WriteString("    pop rbp\n")
	sec.WriteString("    ret\n")

	// Restore outer scope
	cg.varOffsets = outerOffsets
	cg.currentOffset = outerOffset
	cg.currentFunc = outerFunc
	cg.varIsString = outerIsString
	cg.env = outerEnv
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// nasmEscapeString converts a Go string to a NASM db string definition.
// Example: "Hello\n" -> `"Hello", 10`
func nasmEscapeString(s string) string {
	var parts []string
	var current strings.Builder

	flushCurrent := func() {
		if current.Len() > 0 {
			parts = append(parts, `"`+current.String()+`"`)
			current.Reset()
		}
	}

	for _, ch := range s {
		switch ch {
		case '\n':
			flushCurrent()
			parts = append(parts, "10")
		case '\r':
			flushCurrent()
			parts = append(parts, "13")
		case '\t':
			flushCurrent()
			parts = append(parts, "9")
		case '"':
			flushCurrent()
			parts = append(parts, "34")
		default:
			current.WriteRune(ch)
		}
	}
	flushCurrent()

	if len(parts) == 0 {
		return `""`
	}
	return strings.Join(parts, ", ")
}
