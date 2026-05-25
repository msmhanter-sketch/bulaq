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
	varIsBool   map[string]bool

	// Function context
	currentFunc  string
	funcParams   map[string][]string       // funcName -> param names
	funcParamOff map[string]map[string]int // funcName -> paramName -> rbp offset

	// Known functions from typechecker
	funcs map[string]*typechecker.FuncSig

	// Known structs from typechecker
	structs map[string]*parser.StructStatement

	// Known interfaces from typechecker
	interfaces map[string]*parser.InterfaceStatement

	// Loop label stacks for break and continue
	loopStartLabels []string
	loopEndLabels   []string

	// Global variables declared at top-level
	globals map[string]bool
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
		varIsBool:    make(map[string]bool),
		funcParams:   make(map[string][]string),
		funcParamOff: make(map[string]map[string]int),
		globals:      make(map[string]bool),
	}
	if tc != nil {
		cg.funcs = tc.GetFuncs()
		cg.structs = tc.GetStructs()
		cg.interfaces = tc.GetInterfaces()
	} else {
		cg.funcs = make(map[string]*typechecker.FuncSig)
		cg.structs = make(map[string]*parser.StructStatement)
		cg.interfaces = make(map[string]*parser.InterfaceStatement)
	}
	return cg
}

// builtinFuncMap maps Kazakh builtin names -> C runtime function names.
// Functions NOT in this map are called directly by their Kazakh label (user functions).
var builtinFuncMap = map[string]string{
	// ── String utilities ───────────────────────────────────────────────
	"мәтін_бөлу":       "builtin_str_split_get",
	"мәтін_бөлу_саны":  "builtin_str_split_count",
	"мәтін_ауыстыру":   "builtin_str_replace",
	"мәтін_кіші":       "builtin_str_lower",
	"мәтін_жоғары":     "builtin_str_upper",
	"мәтін_қысқарту":   "builtin_str_trim",
	"мәтін_кесу":       "builtin_str_slice",
	"мәтін_басталады":  "builtin_str_starts_with",
	"мәтін_аяқталады":  "builtin_str_ends_with",
	"мәтін_іздеу":      "builtin_str_index_of",
	// ── Extended math ──────────────────────────────────────────────────
	"абс":         "builtin_math_abs",
	"еден":        "builtin_math_floor",
	"төбе":        "builtin_math_ceil",
	"логарифм":    "builtin_math_log",
	"логарифм2":   "builtin_math_log2",
	"логарифм10":  "builtin_math_log10",
	"ең_кіші":     "builtin_math_min",
	"ең_үлкен":    "builtin_math_max",
	"дөңгелек":    "builtin_math_round",
	"тангенс":     "builtin_math_tan",
	"арктангенс":  "builtin_math_atan",
	"арктангенс2": "builtin_math_atan2",

	"дәреже":    "builtin_pow",
	"синус":     "builtin_sin",
	"косинус":   "builtin_cos",
	"кездейсоқ": "builtin_rand_float",
	// ── Existing builtins already named correctly in C ──────────────────
	"мәтін":          "builtin_num_to_str",
	"сан":            "builtin_str_to_num",
	"түбір":          "builtin_sqrt",
	"пи":             "builtin_math_pi",
	"экспонента":     "builtin_math_exp",
	"жүйе":           "builtin_system",
	"файл_жою":       "builtin_file_delete",
	"файл_бар_ма":    "builtin_file_exists",
	"аргумент_саны":  "builtin_args_count",
	"аргумент":       "builtin_arg_get",
	"мәтін_ұзындығы": "builtin_utf8_str_len",
	"таңба":          "builtin_utf8_char_at",
	"уақыт":          "builtin_time_seconds",
	"уақыт_мәтіні":    "builtin_time_str",
	"ұйықтау":       "builtin_sleep_seconds",
	"жүйе_шығысы":    "builtin_exec_output",
	"мд5":           "builtin_md5",
	"ша256":          "builtin_sha256",
	"б64_кодтау":    "builtin_base64_encode",
	"б64_декодтау":   "builtin_base64_decode",
	"жсон_оқу":       "builtin_json_parse",
	"жсон_жазу":      "builtin_json_write",
	"жсон_сан_алу":    "builtin_json_get_number",
	"жсон_мәтін_алу":  "builtin_json_get_string",
	"жсон_логика_алу": "builtin_json_get_bool",
	"жсон_нысан_алу":  "builtin_json_get_object",
	"жсон_тізім_алу":  "builtin_json_get_list",
	"жсон_тізім_өлшемі": "builtin_json_list_size",
	"жсон_тізім_элементі": "builtin_json_list_get",
	"жсон_жаңа":       "builtin_json_create",
	"жсон_сан_қосу":   "builtin_json_add_number",
	"жсон_мәтін_қосу": "builtin_json_add_string",
	"жсон_логика_қосу": "builtin_json_add_bool",
	"жсон_нысан_қосу": "builtin_json_add_object",
	"жсон_тізім_қосу": "builtin_json_add_list",
	"ағын_күту":      "builtin_thread_join",
	"кіру":           "builtin_input",

	// ── Map / Dictionary ───────────────────────────────────────────────
	"сөздік_жаңа":      "builtin_map_create",
	"сөздік_сан_қою":   "builtin_map_set_number",
	"сөздік_мәтін_қою": "builtin_map_set_string",
	"сөздік_сан_алу":   "builtin_map_get_number",
	"сөздік_мәтін_алу": "builtin_map_get_string",
	"сөздік_бар_ма":    "builtin_map_has_key",
	"сөздік_жою":       "builtin_map_delete_key",
	"сөздік_өлшемі":    "builtin_map_size",

	// ── Mutex / Synchronization ─────────────────────────────────────────
	"мьютекс_жаңа":     "builtin_mutex_create",
	"мьютекс_бекіту":   "builtin_mutex_lock",
	"мьютекс_босату":   "builtin_mutex_unlock",
	"мьютекс_жою":      "builtin_mutex_destroy",

	// ── StringBuilder ───────────────────────────────────────────────────
	"мәтін_жинақтаушы_жаңа":       "builtin_string_builder_create",
	"мәтін_жинақтаушы_қосу_мәтін": "builtin_string_builder_append_string",
	"мәтін_жинақтаушы_қосу_сан":   "builtin_string_builder_append_number",
	"мәтін_жинақтаушы_қосу_таңба": "builtin_string_builder_append_char",
	"мәтін_жинақтаушы_жазу":       "builtin_string_builder_to_string",
	"мәтін_жинақтаушы_жою":       "builtin_string_builder_destroy",
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
		cg.textSec.WriteString("    extern calloc\n")
		cg.textSec.WriteString("    extern strdup\n")
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
		cg.textSec.WriteString("    extern _runtime_clock\n")
		cg.textSec.WriteString("    extern ExitProcess\n")
		cg.textSec.WriteString("    extern _thread_spawn\n")
		cg.textSec.WriteString("    extern builtin_init_args\n")
		cg.textSec.WriteString("    extern builtin_result_ok_num\n")
		cg.textSec.WriteString("    extern builtin_result_ok_ptr\n")
		cg.textSec.WriteString("    extern builtin_result_err\n")
		cg.textSec.WriteString("    extern builtin_result_is_error\n")
		cg.textSec.WriteString("    extern builtin_result_get_num\n")
		cg.textSec.WriteString("    extern builtin_result_get_ptr\n")
		cg.textSec.WriteString("    extern builtin_result_get_err\n")
		// Builtin runtime functions
		for _, cName := range builtinFuncMap {
			cg.textSec.WriteString("    extern " + cName + "\n")
		}
		cg.textSec.WriteString("\n")
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
	cg.dataSec.WriteString("    str_true_val  db \"ақиқат\", 0\n")
	cg.dataSec.WriteString("    str_false_val db \"жалған\", 0\n")
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

	// Generate static vtables for interfaces
	if cg.tc != nil {
		for _, structDef := range cg.structs {
			structType := typechecker.Type("ҚҰРЫЛЫМ_" + structDef.Name)
			for _, interfaceDef := range cg.interfaces {
				interfaceType := typechecker.Type("ИНТЕРФЕЙС_" + interfaceDef.Name)
				if cg.tc.SatisfiesInterface(structType, interfaceType) {
					cg.dataSec.WriteString(fmt.Sprintf("vtable_%s_%s:\n", structDef.Name, interfaceDef.Name))
					for _, method := range interfaceDef.Methods {
						funcLabel := fmt.Sprintf("%s_%s", structDef.Name, method.Name)
						cg.dataSec.WriteString(fmt.Sprintf("    dq %s\n", funcLabel))
					}
				}
			}
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
	if cg.platform == PlatformLinux {
		cg.textSec.WriteString("    mov rdi, [rbp+8]\n")
		cg.textSec.WriteString("    lea rsi, [rbp+16]\n")
		cg.textSec.WriteString("    call builtin_init_args\n")
	} else {
		cg.textSec.WriteString("    sub rsp, 32\n")
		cg.textSec.WriteString("    call builtin_init_args\n")
		cg.textSec.WriteString("    add rsp, 32\n")
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
		out.WriteString("    extern calloc\n")
		out.WriteString("    extern strdup\n")
		out.WriteString("    extern sprintf\n")
		out.WriteString("    extern fopen\n")
		out.WriteString("    extern fclose\n")
		out.WriteString("    extern fread\n")
		out.WriteString("    extern fwrite\n")
		out.WriteString("    extern fseek\n")
		out.WriteString("    extern ftell\n")
		out.WriteString("    extern rand\n")
		out.WriteString("    extern _runtime_clock\n")
		out.WriteString("    extern scanf\n")
		out.WriteString("    extern _thread_spawn\n")
		out.WriteString("    extern builtin_init_args\n")
		out.WriteString("    extern builtin_result_ok_num\n")
		out.WriteString("    extern builtin_result_ok_ptr\n")
		out.WriteString("    extern builtin_result_err\n")
		out.WriteString("    extern builtin_result_is_error\n")
		out.WriteString("    extern builtin_result_get_num\n")
		out.WriteString("    extern builtin_result_get_ptr\n")
		out.WriteString("    extern builtin_result_get_err\n")
		// Builtin runtime functions
		for _, cName := range builtinFuncMap {
			out.WriteString("    extern " + cName + "\n")
		}
		out.WriteString("\n")
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
		cg.collectFromStmt(stmt, seen)
	}
}

func (cg *CppGenerator) collectFromStmt(stmt parser.Statement, seen map[string]bool) {
	if stmt == nil {
		return
	}
	switch s := stmt.(type) {
	case *parser.BlockStatement:
		cg.collectUniqueVarNames(s.Statements, seen)
	case *parser.VarAssignStatement:
		seen[s.Name.Value] = true
		cg.collectFromExpr(s.Value, seen)
	case *parser.StructFieldAssignStatement:
		cg.collectFromExpr(s.Value, seen)
	case *parser.IndexAssignStatement:
		cg.collectFromExpr(s.Index, seen)
		cg.collectFromExpr(s.Value, seen)
	case *parser.FreeStatement:
		cg.collectFromExpr(s.Value, seen)
	case *parser.PrintStatement:
		cg.collectFromExpr(s.Value, seen)
	case *parser.IfStatement:
		cg.collectFromExpr(s.Condition, seen)
		if s.Consequence != nil {
			cg.collectUniqueVarNames(s.Consequence.Statements, seen)
		}
		if s.Alternative != nil {
			cg.collectUniqueVarNames(s.Alternative.Statements, seen)
		}
	case *parser.WhileStatement:
		cg.collectFromExpr(s.Condition, seen)
		if s.Body != nil {
			cg.collectUniqueVarNames(s.Body.Statements, seen)
		}
	case *parser.ReturnStatement:
		cg.collectFromExpr(s.Value, seen)
	case *parser.CallStatement:
		cg.collectFromExpr(s.Call, seen)
	case *parser.ExpressionStatement:
		cg.collectFromExpr(s.Expression, seen)
	case *parser.ThreadStatement:
		cg.collectFromStmt(s.Body, seen)
	case *parser.FileWriteStatement:
		cg.collectFromExpr(s.Path, seen)
		cg.collectFromExpr(s.Content, seen)
	}
}

func (cg *CppGenerator) collectFromExpr(expr parser.Expression, seen map[string]bool) {
	if expr == nil {
		return
	}
	switch e := expr.(type) {
	case *parser.TryErrorExpression:
		seen[e.VarName] = true
		cg.collectFromExpr(e.Left, seen)
		if e.Block != nil {
			cg.collectUniqueVarNames(e.Block.Statements, seen)
		}
	case *parser.PostfixExpression:
		cg.collectFromExpr(e.Left, seen)
		cg.collectFromExpr(e.Right, seen)
	case *parser.UnaryExpression:
		cg.collectFromExpr(e.Right, seen)
	case *parser.CallExpression:
		for _, arg := range e.Arguments {
			cg.collectFromExpr(arg, seen)
		}
	case *parser.StructFieldAccessExpression:
		cg.collectFromExpr(e.Target, seen)
	case *parser.ArrayLiteral:
		for _, el := range e.Elements {
			cg.collectFromExpr(el, seen)
		}
	case *parser.IndexExpression:
		cg.collectFromExpr(e.Left, seen)
		cg.collectFromExpr(e.Index, seen)
	case *parser.LengthExpression:
		cg.collectFromExpr(e.Value, seen)
	case *parser.CharAtExpression:
		cg.collectFromExpr(e.Str, seen)
		cg.collectFromExpr(e.Index, seen)
	case *parser.StrConcatExpression:
		cg.collectFromExpr(e.Left, seen)
		cg.collectFromExpr(e.Right, seen)
	case *parser.StrLenExpression:
		cg.collectFromExpr(e.Value, seen)
	case *parser.StrEqExpression:
		cg.collectFromExpr(e.Left, seen)
		cg.collectFromExpr(e.Right, seen)
	case *parser.ToStrExpression:
		cg.collectFromExpr(e.Value, seen)
	case *parser.CharCodeExpression:
		cg.collectFromExpr(e.Value, seen)
	case *parser.FileReadExpression:
		cg.collectFromExpr(e.Path, seen)
	case *parser.ErrorLiteral:
		cg.collectFromExpr(e.Message, seen)
	}
}


func alignTo16(n int) int {
	if n == 0 {
		return 0
	}
	return ((n + 15) / 16) * 16
}

func (cg *CppGenerator) allocVar(name string) int {
	if cg.currentFunc == "" {
		if !cg.globals[name] {
			cg.globals[name] = true
			cg.bssSec.WriteString(fmt.Sprintf("    global_var_%s resq 1\n", name))
		}
		return 0
	}
	if off, ok := cg.varOffsets[name]; ok {
		return off
	}
	cg.currentOffset -= 8
	cg.varOffsets[name] = cg.currentOffset
	return cg.currentOffset
}

func (cg *CppGenerator) getVarOffset(name string) (int, bool) {
	if cg.globals[name] {
		return 0, true
	}
	off, ok := cg.varOffsets[name]
	return off, ok
}

func (cg *CppGenerator) varRef(name string, off int) string {
	if cg.globals[name] {
		return fmt.Sprintf("[global_var_%s]", name)
	}
	return fmt.Sprintf("[rbp%+d]", off)
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
		// Выясняем тип текущей функции
		var retType typechecker.Type = typechecker.VOID_TYPE
		if cg.currentFunc != "" {
			if sig, ok := cg.funcs[cg.currentFunc]; ok {
				retType = sig.ReturnType
			}
		}

		if typechecker.IsResultType(retType) {
			valType := cg.tc.Check(n.Value, cg.env)
			if !typechecker.IsResultType(valType) {
				// Функция возвращает Result, но значение не является Result -> Оборачиваем!
				underlying := typechecker.GetResultUnderlyingType(retType)
				cg.genExpressionCoerced(n.Value, underlying, sec)
				if underlying == typechecker.NUMBER_TYPE || underlying == typechecker.INT_TYPE || underlying == typechecker.BYTE_TYPE || underlying == typechecker.BOOL_TYPE {
					// Числовые типы: приводим к double в xmm0, если нужно
					if valType == typechecker.BOOL_TYPE || valType == typechecker.INT_TYPE || valType == typechecker.BYTE_TYPE {
						sec.WriteString("    cvtsi2sd xmm0, rax\n")
					}
					// Вызываем builtin_result_ok_num
					if cg.platform == PlatformWindows {
						sec.WriteString("    sub rsp, 32\n")
						sec.WriteString("    call builtin_result_ok_num\n")
						sec.WriteString("    add rsp, 32\n")
					} else {
						sec.WriteString("    call builtin_result_ok_num\n")
					}
				} else {
					// Указатели: передаем rax
					if cg.platform == PlatformWindows {
						sec.WriteString("    mov rcx, rax\n")
						sec.WriteString("    sub rsp, 32\n")
						sec.WriteString("    call builtin_result_ok_ptr\n")
						sec.WriteString("    add rsp, 32\n")
					} else {
						sec.WriteString("    mov rdi, rax\n")
						sec.WriteString("    call builtin_result_ok_ptr\n")
					}
				}
				// Результат (указатель ResultObject) в rax. Дублируем его в xmm0.
				sec.WriteString("    movq xmm0, rax\n")
			} else {
				// Значение уже является Result
				cg.genExpression(n.Value, sec)
				sec.WriteString("    movq xmm0, rax\n")
			}
		} else {
			// Обычный возврат для не-Result функций
			cg.genExpressionCoerced(n.Value, retType, sec)
			if cg.isStringExpr(n.Value) || strings.HasPrefix(string(retType), "ИНТЕРФЕЙС_") {
				sec.WriteString("    movq xmm0, rax\n") // pass string/interface pointer via xmm0
			}
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
		sec.WriteString(fmt.Sprintf("    mov r12, %s\n", cg.varRef(n.Array.Value, off))) // r12 = array ptr
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
			sec.WriteString(fmt.Sprintf("    mov r12, %s\n", cg.varRef(n.StructName, off)))
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

	exprType := cg.tc.Check(n.Value, cg.env)
	existingType, ok := cg.env.Get(n.Name.Value)
	if !ok {
		cg.env.Set(n.Name.Value, exprType)
		existingType = exprType
	}

	cg.genExpressionCoerced(n.Value, existingType, sec)
	off := cg.allocVar(n.Name.Value)

	ref := cg.varRef(n.Name.Value, off)
	if existingType == typechecker.STRING_TYPE || existingType == typechecker.ARRAY_TYPE || existingType == typechecker.JSON_TYPE || strings.HasPrefix(string(existingType), "ҚҰРЫЛЫМ_") || strings.HasPrefix(string(existingType), "ИНТЕРФЕЙС_") {
		sec.WriteString(fmt.Sprintf("    mov %s, rax\n", ref))
		cg.varIsString[n.Name.Value] = true
	} else if existingType == typechecker.NUMBER_TYPE || existingType == typechecker.INT_TYPE || existingType == typechecker.BOOL_TYPE {
		if existingType == typechecker.BOOL_TYPE {
			sec.WriteString(fmt.Sprintf("    mov %s, rax\n", ref))
			cg.varIsBool[n.Name.Value] = true
			cg.varIsString[n.Name.Value] = false
		} else {
			sec.WriteString(fmt.Sprintf("    movsd %s, xmm0\n", ref))
			cg.varIsString[n.Name.Value] = false
		}
	} else {
		switch n.Value.(type) {
		case *parser.StringLiteral, *parser.StrConcatExpression,
			*parser.StrEqExpression,
			*parser.ToStrExpression, *parser.CharAtExpression,
			*parser.FileReadExpression,
			*parser.ArrayLiteral:
			sec.WriteString(fmt.Sprintf("    mov %s, rax\n", ref))
			cg.varIsString[n.Name.Value] = true
		case *parser.BoolLiteral:
			sec.WriteString(fmt.Sprintf("    mov %s, rax\n", ref))
			cg.varIsBool[n.Name.Value] = true
			cg.varIsString[n.Name.Value] = false
		case *parser.NumberLiteral, *parser.PostfixExpression, *parser.CharCodeExpression,
			*parser.StrLenExpression, *parser.LengthExpression:
			sec.WriteString(fmt.Sprintf("    movsd %s, xmm0\n", ref))
			cg.varIsString[n.Name.Value] = false
		default:
			if exprType == typechecker.STRING_TYPE || exprType == typechecker.ARRAY_TYPE || exprType == typechecker.JSON_TYPE || strings.HasPrefix(string(exprType), "ҚҰРЫЛЫМ_") || strings.HasPrefix(string(exprType), "ИНТЕРФЕЙС_") || cg.isStringExpr(n.Value) {
				sec.WriteString(fmt.Sprintf("    mov %s, rax\n", ref))
				cg.varIsString[n.Name.Value] = true
			} else {
				sec.WriteString(fmt.Sprintf("    movsd %s, xmm0\n", ref))
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
		*parser.FileReadExpression, *parser.InputExpression,
		*parser.ErrorLiteral, *parser.TryErrorExpression:
		return true
	case *parser.Identifier:
		if t, ok := cg.env.Get(val.Value); ok {
			if typechecker.IsResultType(t) || strings.HasPrefix(string(t), "ИНТЕРФЕЙС_") {
				return true
			}
		}
		return cg.varIsString[val.Value]
	case *parser.CallExpression:
		if sig, ok := cg.funcs[val.Function]; ok {
			return sig.ReturnType == typechecker.STRING_TYPE || sig.ReturnType == typechecker.ARRAY_TYPE || sig.ReturnType == typechecker.JSON_TYPE || strings.HasPrefix(string(sig.ReturnType), "ҚҰРЫЛЫМ_") || strings.HasPrefix(string(sig.ReturnType), "ИНТЕРФЕЙС_") || typechecker.IsResultType(sig.ReturnType)
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
	default:
		exprType := cg.tc.Check(e, cg.env)
		if typechecker.IsResultType(exprType) || strings.HasPrefix(string(exprType), "ИНТЕРФЕЙС_") {
			return true
		}
	}
	return false
}

func (cg *CppGenerator) genExpressionCoerced(expr parser.Expression, expectedType typechecker.Type, sec *strings.Builder) {
	exprType := cg.tc.Check(expr, cg.env)
	if strings.HasPrefix(string(expectedType), "ИНТЕРФЕЙС_") && strings.HasPrefix(string(exprType), "ҚҰРЫЛЫМ_") {
		structName := string(exprType)[len("ҚҰРЫЛЫМ_"):]
		interfaceName := string(expectedType)[len("ИНТЕРФЕЙС_"):]

		cg.genExpression(expr, sec)

		sec.WriteString("    push rax\n")
		sec.WriteString("    sub rsp, 8             ; align stack\n")

		if cg.platform == PlatformWindows {
			sec.WriteString("    mov rcx, 16\n")
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call malloc\n")
			sec.WriteString("    add rsp, 32\n")
		} else {
			sec.WriteString("    mov rdi, 16\n")
			sec.WriteString("    call malloc\n")
		}

		sec.WriteString("    mov r12, [rsp+8]       ; restore struct ptr\n")
		sec.WriteString("    mov [rax], r12\n")

		vtableName := fmt.Sprintf("vtable_%s_%s", structName, interfaceName)
		sec.WriteString(fmt.Sprintf("    lea r13, [%s]\n", vtableName))
		sec.WriteString("    mov [rax+8], r13\n")

		sec.WriteString("    add rsp, 16\n")
		sec.WriteString("    movq xmm0, rax\n")
	} else {
		cg.genExpression(expr, sec)
	}
}

func (cg *CppGenerator) genPrint(n *parser.PrintStatement, sec *strings.Builder) {
	sec.WriteString("    ; жазу\n")

	exprType := cg.tc.Check(n.Value, cg.env)
	if exprType == typechecker.BOOL_TYPE {
		cg.genExpression(n.Value, sec)
		cg.genPrintBool(sec)
		if cg.platform == PlatformWindows {
			sec.WriteString("    xor rcx, rcx\n")
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call fflush\n")
			sec.WriteString("    add rsp, 32\n")
		}
		return
	}

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
		if cg.platform == PlatformWindows {
			sec.WriteString("    xor rcx, rcx\n")
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call fflush\n")
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
		ref := cg.varRef(val.Value, off)
		if cg.varIsString[val.Value] {
			// String variable: load pointer from rax slot
			sec.WriteString(fmt.Sprintf("    mov rax, %s\n", ref))
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
				sec.WriteString(fmt.Sprintf("    movsd xmm0, %s\n", ref))
				sec.WriteString("    mov rdi, fmt_float\n")
				sec.WriteString("    mov rax, 1\n")
				sec.WriteString("    call printf\n")
			} else {
				sec.WriteString(fmt.Sprintf("    movsd xmm1, %s\n", ref))
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

func (cg *CppGenerator) genPrintBool(sec *strings.Builder) {
	lblTrue := cg.newLabel("bool_true")
	lblEnd := cg.newLabel("bool_end")
	sec.WriteString("    cmp rax, 0\n")
	sec.WriteString(fmt.Sprintf("    jne %s\n", lblTrue))
	// False case:
	sec.WriteString("    lea rax, [str_false_val]\n")
	sec.WriteString(fmt.Sprintf("    jmp %s\n", lblEnd))
	// True case:
	sec.WriteString(fmt.Sprintf("%s:\n", lblTrue))
	sec.WriteString("    lea rax, [str_true_val]\n")
	// End:
	sec.WriteString(fmt.Sprintf("%s:\n", lblEnd))

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

	leaf := cg.getLeafOperand(pe.Right)
	if leaf != "" {
		cg.genExpression(pe.Left, sec)
		sec.WriteString(fmt.Sprintf("    comisd xmm0, %s\n", leaf))
		switch pe.Operator {
		case "үлкен": // xmm0 > leaf: jump to fail if <=
			sec.WriteString(fmt.Sprintf("    jbe %s\n", failLabel))
		case "кіші": // xmm0 < leaf: jump to fail if >=
			sec.WriteString(fmt.Sprintf("    jae %s\n", failLabel))
		case "тең": // xmm0 == leaf: jump to fail if !=
			sec.WriteString(fmt.Sprintf("    jne %s\n", failLabel))
		case "тең_емес": // xmm0 != leaf: jump to fail if ==
			sec.WriteString(fmt.Sprintf("    je %s\n", failLabel))
		case "үлкен_тең": // xmm0 >= leaf: jump to fail if <
			sec.WriteString(fmt.Sprintf("    jb %s\n", failLabel))
		case "кіші_тең": // xmm0 <= leaf: jump to fail if >
			sec.WriteString(fmt.Sprintf("    ja %s\n", failLabel))
		default:
			// AND/OR/etc. fallback: evaluate as bool
			cg.genExpression(cond, sec)
			sec.WriteString("    test rax, rax\n")
			sec.WriteString(fmt.Sprintf("    jz %s\n", failLabel))
		}
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
		label := cg.newFloat(float64(n.Value))
		sec.WriteString(fmt.Sprintf("    movsd xmm0, [%s]\n", label))

	case *parser.ErrorLiteral:
		cg.genExpression(n.Message, sec) // rax = char* error message
		if cg.platform == PlatformWindows {
			sec.WriteString("    mov rcx, rax\n")
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call builtin_result_err\n")
			sec.WriteString("    add rsp, 32\n")
		} else {
			sec.WriteString("    mov rdi, rax\n")
			sec.WriteString("    call builtin_result_err\n")
		}
		// rax = ResultObject*. Duplicate to xmm0.
		sec.WriteString("    movq xmm0, rax\n")

	case *parser.TryErrorExpression:
		successLabel := cg.newLabel("try_ok")
		endLabel := cg.newLabel("try_end")
		leftType := cg.tc.Check(n.Left, cg.env)
		underlying := typechecker.GetResultUnderlyingType(leftType)

		// 1. Evaluate Left (result in rax)
		cg.genExpression(n.Left, sec)
		
		// 2. Save Result pointer on stack and align to 16 bytes
		sec.WriteString("    push rax\n")
		sec.WriteString("    sub rsp, 8             ; align stack to 16 bytes\n")

		// 3. Check if Result is error
		if cg.platform == PlatformWindows {
			sec.WriteString("    mov rcx, rax\n")
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call builtin_result_is_error\n")
			sec.WriteString("    add rsp, 32\n")
		} else {
			sec.WriteString("    mov rdi, rax\n")
			sec.WriteString("    call builtin_result_is_error\n")
		}

		// 4. Branch based on result (1 = error, 0 = success)
		sec.WriteString("    cmp rax, 1\n")
		sec.WriteString(fmt.Sprintf("    jne %s\n", successLabel))

		// 5. Error branch:
		// Retrieve error message: builtin_result_get_err(res)
		sec.WriteString("    mov rax, [rsp+8]\n") // restore Result pointer from stack
		if cg.platform == PlatformWindows {
			sec.WriteString("    mov rcx, rax\n")
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call builtin_result_get_err\n")
			sec.WriteString("    add rsp, 32\n")
		} else {
			sec.WriteString("    mov rdi, rax\n")
			sec.WriteString("    call builtin_result_get_err\n")
		}
		// rax = char* error message. Save it to local variable n.VarName
		off := cg.allocVar(n.VarName)
		ref := cg.varRef(n.VarName, off)
		cg.varIsString[n.VarName] = true
		sec.WriteString(fmt.Sprintf("    mov %s, rax\n", ref))

		// Execute error block statements
		for _, stmt := range n.Block.Statements {
			cg.genStatement(stmt, sec)
		}

		// Restore stack
		sec.WriteString("    add rsp, 16\n")

		// Sync result registers based on the underlying type
		if underlying == typechecker.NUMBER_TYPE || underlying == typechecker.INT_TYPE || underlying == typechecker.BYTE_TYPE || underlying == typechecker.BOOL_TYPE {
			if underlying == typechecker.BOOL_TYPE || underlying == typechecker.INT_TYPE || underlying == typechecker.BYTE_TYPE {
				sec.WriteString("    cvttsd2si rax, xmm0\n")
			}
		} else {
			sec.WriteString("    movq xmm0, rax\n")
		}
		sec.WriteString(fmt.Sprintf("    jmp %s\n", endLabel))

		// 6. Success branch:
		sec.WriteString(successLabel + ":\n")
		sec.WriteString("    mov rax, [rsp+8]\n") // restore Result pointer from stack

		if underlying == typechecker.NUMBER_TYPE || underlying == typechecker.INT_TYPE || underlying == typechecker.BYTE_TYPE || underlying == typechecker.BOOL_TYPE {
			if cg.platform == PlatformWindows {
				sec.WriteString("    mov rcx, rax\n")
				sec.WriteString("    sub rsp, 32\n")
				sec.WriteString("    call builtin_result_get_num\n")
				sec.WriteString("    add rsp, 32\n")
			} else {
				sec.WriteString("    mov rdi, rax\n")
				sec.WriteString("    call builtin_result_get_num\n")
			}
			// Result in xmm0.
			if underlying == typechecker.BOOL_TYPE || underlying == typechecker.INT_TYPE || underlying == typechecker.BYTE_TYPE {
				sec.WriteString("    cvttsd2si rax, xmm0\n")
			}
		} else {
			if cg.platform == PlatformWindows {
				sec.WriteString("    mov rcx, rax\n")
				sec.WriteString("    sub rsp, 32\n")
				sec.WriteString("    call builtin_result_get_ptr\n")
				sec.WriteString("    add rsp, 32\n")
			} else {
				sec.WriteString("    mov rdi, rax\n")
				sec.WriteString("    call builtin_result_get_ptr\n")
			}
			// Result in rax. Duplicate to xmm0.
			sec.WriteString("    movq xmm0, rax\n")
		}

		// Restore stack
		sec.WriteString("    add rsp, 16\n")

		// 7. End label
		sec.WriteString(endLabel + ":\n")

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
			sec.WriteString(fmt.Sprintf("    mov r12, %s\n", cg.varRef(n.StructName, off)))
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
		ref := cg.varRef(n.Value, off)
		sec.WriteString(fmt.Sprintf("    movsd xmm0, %s\n", ref))
		// Also load as int in rax (for bool/int vars)
		sec.WriteString(fmt.Sprintf("    mov rax, %s\n", ref))

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

	case *parser.ThreadStatement:
		threadID := cg.labelCount
		cg.labelCount++
		wrapperLabel := fmt.Sprintf("thread_wrapper_%d", threadID)

		var bodyStmts []parser.Statement
		if block, ok := n.Body.(*parser.BlockStatement); ok {
			bodyStmts = block.Statements
		} else {
			bodyStmts = []parser.Statement{n.Body}
		}

		// Save state
		outerOffsets := cg.varOffsets
		outerOffset := cg.currentOffset
		outerFunc := cg.currentFunc
		outerIsString := cg.varIsString
		outerEnv := cg.env

		cg.varOffsets = make(map[string]int)
		cg.currentOffset = 0
		cg.currentFunc = wrapperLabel
		cg.varIsString = make(map[string]bool)
		cg.env = typechecker.NewEnclosedTypeEnv(cg.env)

		cg.funcSec.WriteString(fmt.Sprintf("\n; thread wrapper %d\n", threadID))
		cg.funcSec.WriteString(wrapperLabel + ":\n")
		cg.funcSec.WriteString("    push rbp\n")
		cg.funcSec.WriteString("    mov rbp, rsp\n")

		stackSize := cg.calcStackSize(bodyStmts)
		stackSize = alignTo16(stackSize)
		if stackSize > 0 {
			cg.funcSec.WriteString(fmt.Sprintf("    sub rsp, %d\n", stackSize))
		}

		for _, stmt := range bodyStmts {
			cg.genStatement(stmt, &cg.funcSec)
		}

		// Thread wrapper return epilogue
		cg.funcSec.WriteString("    xor rax, rax\n")
		if stackSize > 0 {
			cg.funcSec.WriteString(fmt.Sprintf("    add rsp, %d\n", stackSize))
		}
		cg.funcSec.WriteString("    pop rbp\n")
		cg.funcSec.WriteString("    ret\n")

		// Restore state
		cg.varOffsets = outerOffsets
		cg.currentOffset = outerOffset
		cg.currentFunc = outerFunc
		cg.varIsString = outerIsString
		cg.env = outerEnv

		// Call _thread_spawn
		sec.WriteString("    ; spawn thread\n")
		if cg.platform == PlatformWindows {
			sec.WriteString(fmt.Sprintf("    lea rcx, [%s]\n", wrapperLabel))
			sec.WriteString("    xor rdx, rdx\n")
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call _thread_spawn\n")
			sec.WriteString("    add rsp, 32\n")
		} else {
			sec.WriteString(fmt.Sprintf("    lea rdi, [%s]\n", wrapperLabel))
			sec.WriteString("    xor rsi, rsi\n")
			sec.WriteString("    call _thread_spawn\n")
		}
		sec.WriteString("    cvtsi2sd xmm0, rax\n")
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
	// Evaluate path first, push it to stack
	cg.genExpression(n.Path, sec)
	sec.WriteString("    push rax\n")
	// Evaluate content, push it to stack
	cg.genExpression(n.Content, sec)
	sec.WriteString("    push rax\n")

	lblDone := cg.newLabel("fwrite_done")

	if cg.platform == PlatformWindows {
		// fopen(path, "w")
		sec.WriteString("    mov rcx, [rsp+8]\n")
		sec.WriteString("    lea rdx, [file_w]\n")
		sec.WriteString("    sub rsp, 32\n")
		sec.WriteString("    call fopen\n")
		sec.WriteString("    add rsp, 32\n")
		sec.WriteString("    mov r14, rax\n") // r14 = FILE*
		sec.WriteString("    test r14, r14\n")
		sec.WriteString(fmt.Sprintf("    jz %s\n", lblDone))
		// strlen(content)
		sec.WriteString("    mov rcx, [rsp]\n")
		sec.WriteString("    sub rsp, 32\n")
		sec.WriteString("    call strlen\n")
		sec.WriteString("    add rsp, 32\n")
		sec.WriteString("    mov r15, rax\n")
		// fwrite(content, 1, len, FILE*)
		sec.WriteString("    mov rcx, [rsp]\n")
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
		sec.WriteString(fmt.Sprintf("%s:\n", lblDone))
		sec.WriteString("    add rsp, 16\n")
	} else {
		// Linux: System V ABI
		sec.WriteString("    mov rdi, [rsp+8]\n")
		sec.WriteString("    lea rsi, [file_w]\n")
		sec.WriteString("    call fopen\n")
		sec.WriteString("    mov r14, rax\n")
		sec.WriteString("    test r14, r14\n")
		sec.WriteString(fmt.Sprintf("    jz %s\n", lblDone))
		// strlen(content)
		sec.WriteString("    mov rdi, [rsp]\n")
		sec.WriteString("    call strlen\n")
		sec.WriteString("    mov r15, rax\n")
		// fwrite(content, 1, len, FILE*)
		sec.WriteString("    mov rdi, [rsp]\n")
		sec.WriteString("    mov rsi, 1\n")
		sec.WriteString("    mov rdx, r15\n")
		sec.WriteString("    mov rcx, r14\n")
		sec.WriteString("    call fwrite\n")
		// fclose
		sec.WriteString("    mov rdi, r14\n")
		sec.WriteString("    call fclose\n")
		sec.WriteString(fmt.Sprintf("%s:\n", lblDone))
		sec.WriteString("    add rsp, 16\n")
	}
}

// ---------------------------------------------------------------------------
// Binary operations
// ---------------------------------------------------------------------------

func isLeafExpression(e parser.Expression) bool {
	switch e.(type) {
	case *parser.NumberLiteral, *parser.IntLiteral, *parser.Identifier, *parser.BoolLiteral:
		return true
	}
	return false
}

func (cg *CppGenerator) isSimpleExpression(e parser.Expression) bool {
	switch n := e.(type) {
	case *parser.BoolLiteral, *parser.IntLiteral, *parser.NumberLiteral, *parser.Identifier:
		return true
	case *parser.UnaryExpression:
		return n.Operator == "емес" && cg.isSimpleExpression(n.Right)
	}
	return false
}

func (cg *CppGenerator) getLeafOperand(e parser.Expression) string {
	switch n := e.(type) {
	case *parser.NumberLiteral:
		label := cg.newFloat(n.Value)
		return fmt.Sprintf("[%s]", label)
	case *parser.IntLiteral:
		label := cg.newFloat(float64(n.Value))
		return fmt.Sprintf("[%s]", label)
	case *parser.Identifier:
		off, ok := cg.getVarOffset(n.Value)
		if !ok {
			return ""
		}
		return cg.varRef(n.Value, off)
	}
	return ""
}

func (cg *CppGenerator) genSimpleExpression(e parser.Expression, reg string, sec *strings.Builder) {
	isXmm := strings.HasPrefix(reg, "xmm")
	if ue, ok := e.(*parser.UnaryExpression); ok && ue.Operator == "емес" {
		if isXmm {
			// For XMM: evaluate into rax, flip, then convert
			cg.genSimpleExpression(ue.Right, "rax", sec)
			sec.WriteString("    xor rax, 1\n")
			sec.WriteString(fmt.Sprintf("    cvtsi2sd %s, rax\n", reg))
		} else {
			cg.genSimpleExpression(ue.Right, reg, sec)
			sec.WriteString(fmt.Sprintf("    xor %s, 1\n", reg))
		}
		return
	}
	switch n := e.(type) {
	case *parser.BoolLiteral:
		val := 0
		if n.Value {
			val = 1
		}
		if isXmm {
			// XMM register: load a float constant 0.0 or 1.0
			label := cg.newFloat(float64(val))
			sec.WriteString(fmt.Sprintf("    movsd %s, [%s]\n", reg, label))
		} else {
			if val == 0 {
				sec.WriteString(fmt.Sprintf("    xor %s, %s\n", reg, reg))
			} else {
				sec.WriteString(fmt.Sprintf("    mov %s, 1\n", reg))
			}
		}
	case *parser.IntLiteral:
		if isXmm {
			label := cg.newFloat(float64(n.Value))
			sec.WriteString(fmt.Sprintf("    movsd %s, [%s]\n", reg, label))
		} else {
			if n.Value == 0 {
				sec.WriteString(fmt.Sprintf("    xor %s, %s\n", reg, reg))
			} else {
				sec.WriteString(fmt.Sprintf("    mov %s, %d\n", reg, n.Value))
			}
		}
	case *parser.NumberLiteral:
		if isXmm {
			label := cg.newFloat(n.Value)
			sec.WriteString(fmt.Sprintf("    movsd %s, [%s]\n", reg, label))
		} else {
			label := cg.newFloat(n.Value)
			sec.WriteString(fmt.Sprintf("    movsd xmm0, [%s]\n", label))
			sec.WriteString(fmt.Sprintf("    cvttsd2si %s, xmm0\n", reg))
		}
	case *parser.Identifier:
		off, ok := cg.getVarOffset(n.Value)
		if !ok {
			return
		}
		ref := cg.varRef(n.Value, off)
		if isXmm {
			sec.WriteString(fmt.Sprintf("    movsd %s, %s\n", reg, ref))
		} else {
			sec.WriteString(fmt.Sprintf("    mov %s, %s\n", reg, ref))
		}
	default:
		if isXmm {
			cg.genExpression(e, sec)
			if reg != "xmm0" {
				sec.WriteString(fmt.Sprintf("    movsd %s, xmm0\n", reg))
			}
		} else {
			cg.genExpression(e, sec)
			if reg != "rax" {
				sec.WriteString(fmt.Sprintf("    mov %s, rax\n", reg))
			}
		}
	}
}

func (cg *CppGenerator) genBinaryOp(n *parser.PostfixExpression, sec *strings.Builder) {
	if n.Operator == "және" || n.Operator == "немесе" {
		if cg.isSimpleExpression(n.Right) {
			cg.genExpression(n.Left, sec)
			sec.WriteString("    mov rcx, rax\n")
			cg.genSimpleExpression(n.Right, "rax", sec)
		} else {
			cg.genExpression(n.Left, sec)
			sec.WriteString("    sub rsp, 8\n")
			sec.WriteString("    mov [rsp], rax\n") // push left boolean (rax)

			cg.genExpression(n.Right, sec)
			sec.WriteString("    mov rcx, [rsp]\n") // pop left into rcx
			sec.WriteString("    add rsp, 8\n")
		}

		if n.Operator == "және" {
			sec.WriteString("    and rax, rcx\n")
		} else {
			sec.WriteString("    or rax, rcx\n")
		}
		// Sync to xmm0 as float (0.0 or 1.0) so it works in prints and variable assignments
		sec.WriteString("    cvtsi2sd xmm0, rax\n")
		return
	}

	leaf := cg.getLeafOperand(n.Right)
	if leaf != "" {
		cg.genExpression(n.Left, sec) // left value in xmm0
		switch n.Operator {
		case "қосу":
			sec.WriteString(fmt.Sprintf("    addsd xmm0, %s\n", leaf))
		case "алу":
			sec.WriteString(fmt.Sprintf("    subsd xmm0, %s\n", leaf))
		case "көбейту":
			sec.WriteString(fmt.Sprintf("    mulsd xmm0, %s\n", leaf))
		case "бөлу":
			leftType := cg.tc.Check(n.Left, cg.env)
			rightType := cg.tc.Check(n.Right, cg.env)
			if leftType == typechecker.INT_TYPE && rightType == typechecker.INT_TYPE {
				sec.WriteString("    cvttsd2si rax, xmm0\n")
				if il, ok := n.Right.(*parser.IntLiteral); ok {
					if il.Value == 0 {
						sec.WriteString("    jmp _runtime_divide_by_zero_error\n")
					} else {
						sec.WriteString(fmt.Sprintf("    mov rcx, %d\n", il.Value))
					}
				} else {
					sec.WriteString(fmt.Sprintf("    cvttsd2si rcx, %s\n", leaf))
					sec.WriteString("    cmp rcx, 0\n")
					sec.WriteString("    je _runtime_divide_by_zero_error\n")
				}
				sec.WriteString("    cqo\n")
				sec.WriteString("    idiv rcx\n")
				sec.WriteString("    cvtsi2sd xmm0, rax\n")
			} else {
				isZeroLiteral := false
				if il, ok := n.Right.(*parser.IntLiteral); ok && il.Value == 0 {
					isZeroLiteral = true
				} else if fl, ok := n.Right.(*parser.NumberLiteral); ok && fl.Value == 0.0 {
					isZeroLiteral = true
				}
				if isZeroLiteral {
					sec.WriteString("    jmp _runtime_divide_by_zero_error\n")
				} else if _, ok := n.Right.(*parser.IntLiteral); ok {
					// safe literal, skip check
					sec.WriteString(fmt.Sprintf("    divsd xmm0, %s\n", leaf))
				} else if _, ok := n.Right.(*parser.NumberLiteral); ok {
					// safe literal, skip check
					sec.WriteString(fmt.Sprintf("    divsd xmm0, %s\n", leaf))
				} else {
					sec.WriteString(fmt.Sprintf("    movsd xmm2, %s\n", leaf))
					sec.WriteString("    xorpd xmm3, xmm3\n")
					sec.WriteString("    comisd xmm2, xmm3\n")
					sec.WriteString("    je _runtime_divide_by_zero_error\n")
					sec.WriteString(fmt.Sprintf("    divsd xmm0, %s\n", leaf))
				}
			}
		case "үлкен", "кіші", "тең", "тең_емес", "үлкен_тең", "кіші_тең":
			sec.WriteString("    xor rax, rax\n")
			sec.WriteString(fmt.Sprintf("    comisd xmm0, %s\n", leaf))
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
		return
	}

	// Fallback to standard stack-based operations
	if isLeafExpression(n.Right) {
		cg.genExpression(n.Left, sec)
		sec.WriteString("    movsd xmm1, xmm0\n") // move left to xmm1
		cg.genExpression(n.Right, sec)          // evaluate right into xmm0
	} else {
		// Evaluate left, push to stack, evaluate right, then operate
		cg.genExpression(n.Left, sec)
		sec.WriteString("    sub rsp, 8\n")
		sec.WriteString("    movsd [rsp], xmm0\n") // push left

		cg.genExpression(n.Right, sec)
		sec.WriteString("    movsd xmm1, [rsp]\n") // pop left into xmm1
		sec.WriteString("    add rsp, 8\n")
	}
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
		leftType := cg.tc.Check(n.Left, cg.env)
		rightType := cg.tc.Check(n.Right, cg.env)
		if leftType == typechecker.INT_TYPE && rightType == typechecker.INT_TYPE {
			sec.WriteString("    cvttsd2si rax, xmm1\n")
			sec.WriteString("    cvttsd2si rcx, xmm0\n")
			sec.WriteString("    cmp rcx, 0\n")
			sec.WriteString("    je _runtime_divide_by_zero_error\n")
			sec.WriteString("    cqo\n")
			sec.WriteString("    idiv rcx\n")
			sec.WriteString("    cvtsi2sd xmm0, rax\n")
		} else {
			sec.WriteString("    xorpd xmm2, xmm2\n")
			sec.WriteString("    comisd xmm0, xmm2\n")
			sec.WriteString("    je _runtime_divide_by_zero_error\n")
			sec.WriteString("    divsd xmm1, xmm0\n")
			sec.WriteString("    movsd xmm0, xmm1\n")
		}
	case "үлкен", "кіші", "тең", "тең_емес", "үлкен_тең", "кіші_тең":
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

	case "уақыт":
		sec.WriteString("    ; уақыт\n")
		if cg.platform == PlatformWindows {
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call _runtime_clock\n")
			sec.WriteString("    add rsp, 32\n")
		} else {
			sec.WriteString("    call _runtime_clock\n")
		}
		// _runtime_clock returns double in xmm0
		return

	case "ағын_күту":
		sec.WriteString("    ; ағын_күту\n")
		cg.genExpression(n.Arguments[0], sec) // xmm0 = float handle
		if cg.platform == PlatformWindows {
			sec.WriteString("    cvttsd2si rcx, xmm0\n")
			sec.WriteString("    sub rsp, 32\n")
			sec.WriteString("    call builtin_thread_join\n")
			sec.WriteString("    add rsp, 32\n")
		} else {
			sec.WriteString("    cvttsd2si rdi, xmm0\n")
			sec.WriteString("    call builtin_thread_join\n")
		}
		sec.WriteString("    cvtsi2sd xmm0, rax\n")
		return
	}

	// Check if it is a struct constructor
	// Interface constructor/cast: ДыбысШығарғыш(ит) → allocate box {struct_ptr, vtable_ptr}
	if _, ok := cg.interfaces[n.Function]; ok {
		cg.genExpressionCoerced(n.Arguments[0], typechecker.Type("ИНТЕРФЕЙС_"+n.Function), sec)
		// Result: rax = box pointer (16-byte {struct_ptr, vtable_ptr})
		return
	}

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
			expectedType := cg.tc.FieldTypeToType(structDef.Types[i])
			cg.genExpressionCoerced(arg, expectedType, sec)
			sec.WriteString("    mov r12, [rsp]\n") // peek struct pointer
			fieldType := structDef.Types[i]
			fieldOffset := i * 8
			if fieldType == "МӘТІН" || fieldType == "ТІЗІМ" || (fieldType != "САН" && fieldType != "БҮТІН" && fieldType != "АҚИҚАТ" && fieldType != "БАЙТ") || strings.HasPrefix(string(expectedType), "ИНТЕРФЕЙС_") {
				sec.WriteString(fmt.Sprintf("    mov [r12 + %d], rax\n", fieldOffset))
			} else {
				sec.WriteString(fmt.Sprintf("    movsd [r12 + %d], xmm0\n", fieldOffset))
			}
		}
		sec.WriteString("    pop rax\n") // return struct pointer in rax
		return
	}

	parts := strings.Split(n.Function, ".")
	isInterfaceCall := len(parts) == 2 && cg.interfaces[parts[0]] != nil

	if isInterfaceCall {
		interfaceName := parts[0]
		methodName := parts[1]
		interf := cg.interfaces[interfaceName]
		methodIdx := -1
		for idx, m := range interf.Methods {
			if m.Name == methodName {
				methodIdx = idx
				break
			}
		}
		if methodIdx == -1 {
			sec.WriteString(fmt.Sprintf("    ; ERROR: method %s not found in interface %s\n", methodName, interfaceName))
			return
		}

		sec.WriteString(fmt.Sprintf("    ; интерфейстік шақыру %s\n", n.Function))

		nArgs := len(n.Arguments)
		argIsStr := make([]bool, nArgs)
		sig, _ := cg.funcs[n.Function]

		// Step 1: Evaluate arguments in order and push to stack
		for i, arg := range n.Arguments {
			var expectedType typechecker.Type = typechecker.UNKNOWN
			if sig != nil && i < len(sig.ParamTypes) {
				expectedType = sig.ParamTypes[i]
			}
			cg.genExpressionCoerced(arg, expectedType, sec)
			argIsStr[i] = cg.isStringExpr(arg) || strings.HasPrefix(string(expectedType), "ИНТЕРФЕЙС_")
			if argIsStr[i] {
				sec.WriteString("    sub rsp, 8\n")
				sec.WriteString("    mov [rsp], rax\n")
			} else {
				sec.WriteString("    sub rsp, 8\n")
				sec.WriteString("    movsd [rsp], xmm0\n")
			}
		}

		// Step 2: Pop arguments in reverse order into ABI registers
		for i := nArgs - 1; i >= 0; i-- {
			if argIsStr[i] {
				sec.WriteString("    pop rax\n")
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
						sec.WriteString(fmt.Sprintf("    movsd %s, xmm0\n", winFloatRegs[i]))
						sec.WriteString(fmt.Sprintf("    movq %s, xmm0\n", winArgRegs[i]))
					}
				}
			}
		}

		// Step 3: Extract struct ptr and vtable from the first argument (interface box)
		if cg.platform == PlatformLinux {
			sec.WriteString("    mov r12, rdi           ; r12 = box ptr\n")
			sec.WriteString("    mov r13, [r12 + 8]     ; r13 = vtable ptr\n")
			sec.WriteString("    mov r12, [r12]         ; r12 = struct ptr\n")
			sec.WriteString("    mov rdi, r12           ; first arg = struct ptr\n")
		} else {
			sec.WriteString("    mov r12, rcx           ; r12 = box ptr\n")
			sec.WriteString("    mov r13, [r12 + 8]     ; r13 = vtable ptr\n")
			sec.WriteString("    mov r12, [r12]         ; r12 = struct ptr\n")
			sec.WriteString("    mov rcx, r12           ; first arg = struct ptr\n")
		}

		// Step 4: Call method dynamically
		if cg.platform == PlatformWindows {
			sec.WriteString("    sub rsp, 32\n")
		}
		sec.WriteString(fmt.Sprintf("    mov rax, [r13 + %d]    ; load method address\n", methodIdx*8))
		sec.WriteString("    call rax\n")
		if cg.platform == PlatformWindows {
			sec.WriteString("    add rsp, 32\n")
		}
		return
	}

	sec.WriteString(fmt.Sprintf("    ; шақыру %s\n", n.Function))

	// Resolve Kazakh name → C runtime name if it's a builtin
	funcLabel := strings.ReplaceAll(n.Function, ".", "_")
	if cName, ok := builtinFuncMap[n.Function]; ok {
		funcLabel = cName
	}
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

	// Step 1: Populate argIsStr
	argIsStr := make([]bool, nArgs)
	sig, _ := cg.funcs[n.Function]
	for i, arg := range n.Arguments {
		var expectedType typechecker.Type = typechecker.UNKNOWN
		if sig != nil && i < len(sig.ParamTypes) {
			expectedType = sig.ParamTypes[i]
		}
		argIsStr[i] = cg.isStringExpr(arg) || strings.HasPrefix(string(expectedType), "ИНТЕРФЕЙС_")
	}

	maxRegArgs := 6
	if cg.platform == PlatformWindows {
		maxRegArgs = 4
	}

	complexCount := 0
	complexIdx := -1
	for i, arg := range n.Arguments {
		if !cg.isSimpleExpression(arg) {
			complexCount++
			complexIdx = i
		}
	}
	isStackless := nArgs <= maxRegArgs && complexCount <= 1

	if isStackless {
		getTargetReg := func(i int, isStr bool) (string, string) {
			if cg.platform == PlatformLinux {
				if isStr {
					return linuxArgRegs[i], ""
				} else {
					return linuxFloatRegs[i], ""
				}
			} else {
				if isStr {
					return winArgRegs[i], ""
				} else {
					return winFloatRegs[i], winArgRegs[i]
				}
			}
		}

		// Evaluate complex argument first if there is one
		if complexIdx != -1 {
			var expectedType typechecker.Type = typechecker.UNKNOWN
			if sig != nil && complexIdx < len(sig.ParamTypes) {
				expectedType = sig.ParamTypes[complexIdx]
			}
			cg.genExpressionCoerced(n.Arguments[complexIdx], expectedType, sec)
			reg1, reg2 := getTargetReg(complexIdx, argIsStr[complexIdx])
			if argIsStr[complexIdx] {
				if reg1 != "rax" {
					sec.WriteString(fmt.Sprintf("    mov %s, rax\n", reg1))
				}
			} else {
				if reg1 != "xmm0" {
					sec.WriteString(fmt.Sprintf("    movsd %s, xmm0\n", reg1))
				}
				if reg2 != "" {
					sec.WriteString(fmt.Sprintf("    movq %s, xmm0\n", reg2))
				}
			}
		}

		// Evaluate simple arguments in reverse order (excluding complexIdx)
		for i := nArgs - 1; i >= 0; i-- {
			if i == complexIdx {
				continue
			}
			reg1, reg2 := getTargetReg(i, argIsStr[i])
			if argIsStr[i] {
				cg.genSimpleExpression(n.Arguments[i], reg1, sec)
			} else {
				cg.genSimpleExpression(n.Arguments[i], reg1, sec)
				if reg2 != "" {
					sec.WriteString(fmt.Sprintf("    movq %s, %s\n", reg2, reg1))
				}
			}
		}
	} else {
		// Fallback to stack-based parameter evaluation
		for i, arg := range n.Arguments {
			var expectedType typechecker.Type = typechecker.UNKNOWN
			if sig != nil && i < len(sig.ParamTypes) {
				expectedType = sig.ParamTypes[i]
			}
			cg.genExpressionCoerced(arg, expectedType, sec)
			if argIsStr[i] {
				sec.WriteString("    sub rsp, 8\n")
				sec.WriteString("    mov [rsp], rax\n")
			} else {
				sec.WriteString("    sub rsp, 8\n")
				sec.WriteString("    movsd [rsp], xmm0\n")
			}
		}
		for i := nArgs - 1; i >= 0; i-- {
			if argIsStr[i] {
				sec.WriteString("    pop rax\n")
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
						sec.WriteString(fmt.Sprintf("    movsd %s, xmm0\n", winFloatRegs[i]))
						sec.WriteString(fmt.Sprintf("    movq %s, xmm0\n", winArgRegs[i]))
					}
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
