package codegen

import (
	"butaq/parser"
	"butaq/typechecker"
	"bytes"
	"fmt"
	"strings"
)

type LlvmGenerator struct {
	env      *typechecker.TypeEnv
	tc       *typechecker.TypeChecker
	platform Platform

	// Assembly sections (simulated using Builders)
	globals     strings.Builder
	prototypes  strings.Builder
	functions   strings.Builder
	mainFunc    strings.Builder
	threadFuncs strings.Builder

	// Counters
	regCount     int
	labelCount   int
	strCount     int
	nextThreadID int

	// Variable tracking
	varAllocations map[string]string            // varName -> LLVM pointer
	varTypes       map[string]typechecker.Type  // varName -> Butaq Type
	stringLiterals map[string]string            // string -> global label
	funcs          map[string]*typechecker.FuncSig
	structs        map[string]*parser.StructStatement

	// Function scope
	currentFunc     string
	loopStartLabels []string
	loopEndLabels   []string
}

func NewLlvm(env *typechecker.TypeEnv, tc *typechecker.TypeChecker, platform Platform) *LlvmGenerator {
	lg := &LlvmGenerator{
		env:            env,
		tc:             tc,
		platform:       platform,
		varAllocations: make(map[string]string),
		varTypes:       make(map[string]typechecker.Type),
		stringLiterals: make(map[string]string),
	}
	if tc != nil {
		lg.funcs = tc.GetFuncs()
		lg.structs = tc.GetStructs()
	} else {
		lg.funcs = make(map[string]*typechecker.FuncSig)
		lg.structs = make(map[string]*parser.StructStatement)
	}
	return lg
}

func (lg *LlvmGenerator) getVarType(name string) (typechecker.Type, bool) {
	if t, ok := lg.varTypes[name]; ok {
		return t, true
	}
	if lg.env != nil {
		return lg.env.Get(name)
	}
	return typechecker.UNKNOWN, false
}

func (lg *LlvmGenerator) newReg() string {
	lg.regCount++
	return fmt.Sprintf("%%t%d", lg.regCount)
}

func (lg *LlvmGenerator) newLabel(prefix string) string {
	lg.labelCount++
	return fmt.Sprintf("%s_%d", prefix, lg.labelCount)
}

func (lg *LlvmGenerator) llvmType(t typechecker.Type) string {
	switch t {
	case typechecker.INT_TYPE:
		return "i64"
	case typechecker.BOOL_TYPE:
		return "i1"
	case typechecker.BYTE_TYPE:
		return "i8"
	case typechecker.NUMBER_TYPE:
		return "double"
	case typechecker.STRING_TYPE, typechecker.JSON_TYPE, typechecker.ARRAY_TYPE:
		return "i8*"
	case typechecker.VOID_TYPE:
		return "void"
	default:
		if strings.HasPrefix(string(t), "ҚҰРЫЛЫМ_") || strings.HasPrefix(string(t), "әлсіз_") {
			return "i8*"
		}
		return "i8*"
	}
}

func (lg *LlvmGenerator) isRefType(t typechecker.Type) bool {
	if t == typechecker.STRING_TYPE || t == typechecker.ARRAY_TYPE || t == typechecker.JSON_TYPE {
		return true
	}
	if strings.HasPrefix(string(t), "ҚҰРЫЛЫМ_") {
		return true
	}
	return false
}

func (lg *LlvmGenerator) getOrStr(val string) string {
	if lbl, ok := lg.stringLiterals[val]; ok {
		return lbl
	}
	lg.strCount++
	lbl := fmt.Sprintf("@.str_%d", lg.strCount)
	lg.stringLiterals[val] = lbl

	var escaped bytes.Buffer
	for i := 0; i < len(val); i++ {
		c := val[i]
		if c == '\\' && i+1 < len(val) {
			escaped.WriteByte('\\')
			escaped.WriteByte(val[i+1])
			i++
		} else if c == '\n' {
			escaped.WriteString("\\0A")
		} else if c == '\r' {
			escaped.WriteString("\\0D")
		} else if c == '\t' {
			escaped.WriteString("\\09")
		} else if c == '"' {
			escaped.WriteString("\\22")
		} else if c < 32 || c > 126 {
			escaped.WriteString(fmt.Sprintf("\\%02X", c))
		} else {
			escaped.WriteByte(c)
		}
	}
	escaped.WriteString("\\00")
	escapedStr := escaped.String()
	size := len(val) + 1

	lg.globals.WriteString(fmt.Sprintf("%s = private unnamed_addr constant { i64, i8*, [%d x i8] } { i64 -1, i8* null, [%d x i8] c\"%s\" }, align 8\n", lbl, size, size, escapedStr))
	return lbl
}

func (lg *LlvmGenerator) loadStrLiteral(val string, buf *strings.Builder) string {
	lbl := lg.getOrStr(val)
	size := len(val) + 1
	reg := lg.newReg()
	buf.WriteString(fmt.Sprintf("  %s = getelementptr { i64, i8*, [%d x i8] }, { i64, i8*, [%d x i8] }* %s, i64 0, i32 2, i64 0\n", reg, size, size, lbl))
	return reg
}

func (lg *LlvmGenerator) Generate(program *parser.Program) string {
	// 1. Setup Prototypes
	lg.prototypes.WriteString("; External prototypes\n")
	lg.prototypes.WriteString("declare i8* @malloc(i64)\n")
	lg.prototypes.WriteString("declare void @free(i8*)\n")
	lg.prototypes.WriteString("declare i64 @strlen(i8*)\n")
	lg.prototypes.WriteString("declare i8* @strcpy(i8*, i8*)\n")
	lg.prototypes.WriteString("declare i8* @strcat(i8*, i8*)\n")
	lg.prototypes.WriteString("declare i32 @strcmp(i8*, i8*)\n")
	lg.prototypes.WriteString("declare i32 @printf(i8*, ...)\n")
	lg.prototypes.WriteString("declare i32 @sprintf(i8*, i8*, ...)\n")
	lg.prototypes.WriteString("declare i32 @fflush(i8*)\n")
	lg.prototypes.WriteString("declare i8* @fopen(i8*, i8*)\n")
	lg.prototypes.WriteString("declare i32 @fclose(i8*)\n")
	lg.prototypes.WriteString("declare i64 @fread(i8*, i64, i64, i8*)\n")
	lg.prototypes.WriteString("declare i64 @fwrite(i8*, i64, i64, i8*)\n")
	lg.prototypes.WriteString("declare i64 @fseek(i8*, i64, i32)\n")
	lg.prototypes.WriteString("declare i64 @ftell(i8*)\n")
	lg.prototypes.WriteString("declare i8* @fgets(i8*, i32, i8*)\n")
	lg.prototypes.WriteString("declare i8* @_alloc_ref(i64, void (i8*)*)\n")
	lg.prototypes.WriteString("declare void @_retain(i8*)\n")
	lg.prototypes.WriteString("declare void @_release(i8*)\n")
	lg.prototypes.WriteString("declare void @_weak_assign(i8**, i8*)\n")
	lg.prototypes.WriteString("declare i8* @_weak_load(i8**)\n")
	lg.prototypes.WriteString("declare void @_weak_clear(i8**)\n")
	lg.prototypes.WriteString("declare void @_runtime_divide_by_zero_error_c()\n")
	lg.prototypes.WriteString("declare void @_runtime_null_pointer_error_c()\n")
	lg.prototypes.WriteString("declare void @_runtime_array_bounds_error_c()\n")

	// Standard library
	lg.prototypes.WriteString("declare i8* @builtin_json_parse(i8*)\n")
	lg.prototypes.WriteString("declare i8* @builtin_json_write(i8*)\n")
	lg.prototypes.WriteString("declare double @builtin_json_get_number(i8*, i8*)\n")
	lg.prototypes.WriteString("declare i8* @builtin_json_get_string(i8*, i8*)\n")
	lg.prototypes.WriteString("declare i64 @builtin_json_get_bool(i8*, i8*)\n")
	lg.prototypes.WriteString("declare i8* @builtin_json_get_object(i8*, i8*)\n")
	lg.prototypes.WriteString("declare i8* @builtin_json_get_list(i8*, i8*)\n")
	lg.prototypes.WriteString("declare double @builtin_json_list_size(i8*)\n")
	lg.prototypes.WriteString("declare i8* @builtin_json_list_get(i8*, double)\n")
	lg.prototypes.WriteString("declare i8* @builtin_json_create()\n")
	lg.prototypes.WriteString("declare void @builtin_json_add_number(i8*, i8*, double)\n")
	lg.prototypes.WriteString("declare void @builtin_json_add_string(i8*, i8*, i8*)\n")
	lg.prototypes.WriteString("declare void @builtin_json_add_bool(i8*, i8*, i64)\n")
	lg.prototypes.WriteString("declare void @builtin_json_add_object(i8*, i8*, i8*)\n")
	lg.prototypes.WriteString("declare void @builtin_json_add_list(i8*, i8*, i8*)\n")
	lg.prototypes.WriteString("declare i8* @builtin_md5(i8*)\n")
	lg.prototypes.WriteString("declare i8* @builtin_sha256(i8*)\n")
	lg.prototypes.WriteString("declare i8* @builtin_base64_encode(i8*)\n")
	lg.prototypes.WriteString("declare i8* @builtin_base64_decode(i8*)\n")
	lg.prototypes.WriteString("declare i64 @builtin_thread_join(i64)\n")
	lg.prototypes.WriteString("declare i64 @_thread_spawn(i8* (i8*)*, i8*)\n")
	lg.prototypes.WriteString("declare i8* @builtin_input()\n")
	lg.prototypes.WriteString("declare double @builtin_time_seconds()\n")
	lg.prototypes.WriteString("declare i8* @builtin_time_str(i8*)\n")
	lg.prototypes.WriteString("declare void @builtin_sleep_seconds(double)\n")
	lg.prototypes.WriteString("declare void @builtin_init_args(i32, i8**)\n")
	lg.prototypes.WriteString("declare double @builtin_args_count()\n")
	lg.prototypes.WriteString("declare i8* @builtin_arg_get(double)\n")
	lg.prototypes.WriteString("declare i8* @builtin_num_to_str(double)\n")
	lg.prototypes.WriteString("declare double @builtin_str_to_num(i8*)\n")
	lg.prototypes.WriteString("declare double @builtin_system(i8*)\n")
	lg.prototypes.WriteString("declare double @builtin_file_delete(i8*)\n")
	lg.prototypes.WriteString("declare double @builtin_file_exists(i8*)\n")
	lg.prototypes.WriteString("declare double @builtin_math_pi()\n")
	lg.prototypes.WriteString("declare double @builtin_math_exp(double)\n")
	lg.prototypes.WriteString("declare double @builtin_sqrt(double)\n")

	// 2. Generate Struct Destructors
	lg.genStructDestructors()

	// 3. Collect and pre-declare all user-defined functions
	for _, stmt := range program.Statements {
		if fn, ok := stmt.(*parser.FunctionStatement); ok {
			sig := lg.funcs[fn.Name]
			retType := "double"
			if sig != nil {
				retType = lg.llvmType(sig.ReturnType)
			}
			var params []string
			for _, pT := range sig.ParamTypes {
				params = append(params, lg.llvmType(pT))
			}
			lg.prototypes.WriteString(fmt.Sprintf("declare %s @%s(%s)\n", retType, fn.Name, strings.Join(params, ", ")))
		}
	}

	// 4. Generate Main Function Signature
	lg.mainFunc.WriteString("\ndefine i32 @main() {\n")
	lg.currentFunc = "main"

	// 5. Generate Statements
	for _, stmt := range program.Statements {
		if _, ok := stmt.(*parser.FunctionStatement); ok {
			// Functions are generated out-of-band in lg.functions
			continue
		}
		if _, ok := stmt.(*parser.StructStatement); ok {
			// Struct statements are type declarations, nothing to generate directly
			continue
		}
		lg.genStatement(stmt, &lg.mainFunc)
	}

	// 6. Release Main's Locals & Return 0
	lg.genReleaseLocals(&lg.mainFunc, false)
	lg.mainFunc.WriteString("  ret i32 0\n")
	lg.mainFunc.WriteString("}\n")

	// 7. Generate User-Defined Functions
	for _, stmt := range program.Statements {
		if fn, ok := stmt.(*parser.FunctionStatement); ok {
			lg.genFunction(fn)
		}
	}

	// 8. Combine everything
	var out bytes.Buffer
	out.WriteString(lg.globals.String())
	out.WriteString("\n")
	out.WriteString(lg.prototypes.String())
	out.WriteString("\n")
	out.WriteString(lg.functions.String())
	out.WriteString("\n")
	out.WriteString(lg.threadFuncs.String())
	out.WriteString("\n")
	out.WriteString(lg.mainFunc.String())

	return out.String()
}

func (lg *LlvmGenerator) genStructDestructors() {
	for name, structDef := range lg.structs {
		lg.prototypes.WriteString(fmt.Sprintf("define void @_destructor_struct_%s(i8* %%ptr) {\n", name))
		lg.prototypes.WriteString("  %self = bitcast i8* %ptr to i8*\n")
		for i, fieldType := range structDef.Types {
			offset := i * 8
			if lg.isRefType(typechecker.Type(fieldType)) {
				// release ref type
				fieldPtrReg := lg.newReg()
				lg.prototypes.WriteString(fmt.Sprintf("  %s = getelementptr i8, i8* %%self, i64 %d\n", fieldPtrReg, offset))
				castReg := lg.newReg()
				lg.prototypes.WriteString(fmt.Sprintf("  %s = bitcast i8* %s to i8**\n", castReg, fieldPtrReg))
				loadReg := lg.newReg()
				lg.prototypes.WriteString(fmt.Sprintf("  %s = load i8*, i8** %s\n", loadReg, castReg))
				lg.prototypes.WriteString(fmt.Sprintf("  call void @_release(i8* %s)\n", loadReg))
			} else if strings.HasPrefix(fieldType, "әлсіз_") {
				// weak clear
				fieldPtrReg := lg.newReg()
				lg.prototypes.WriteString(fmt.Sprintf("  %s = getelementptr i8, i8* %%self, i64 %d\n", fieldPtrReg, offset))
				castReg := lg.newReg()
				lg.prototypes.WriteString(fmt.Sprintf("  %s = bitcast i8* %s to i8**\n", castReg, fieldPtrReg))
				lg.prototypes.WriteString(fmt.Sprintf("  call void @_weak_clear(i8** %s)\n", castReg))
			}
		}
		lg.prototypes.WriteString("  ret void\n")
		lg.prototypes.WriteString("}\n\n")
	}
}

func (lg *LlvmGenerator) genReleaseLocals(buf *strings.Builder, saveRet bool) {
	// We release all local variables of reference types
	for name, allocReg := range lg.varAllocations {
		t := lg.varTypes[name]
		if lg.isRefType(t) {
			loadReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = load i8*, i8** %s\n", loadReg, allocReg))
			buf.WriteString(fmt.Sprintf("  call void @_release(i8* %s)\n", loadReg))
		}
	}
}

func (lg *LlvmGenerator) genFunction(fn *parser.FunctionStatement) {
	sig := lg.funcs[fn.Name]
	retTypeStr := "double"
	var retType typechecker.Type = typechecker.NUMBER_TYPE
	if sig != nil {
		retType = sig.ReturnType
		retTypeStr = lg.llvmType(retType)
	}

	var params []string
	for i, paramName := range fn.Parameters {
		pType := typechecker.NUMBER_TYPE
		if sig != nil && i < len(sig.ParamTypes) {
			pType = sig.ParamTypes[i]
		}
		params = append(params, fmt.Sprintf("%s %%.param.%s", lg.llvmType(pType), paramName))
	}

	lg.functions.WriteString(fmt.Sprintf("\ndefine %s @%s(%s) {\n", retTypeStr, fn.Name, strings.Join(params, ", ")))

	// Save environment and allocations
	oldVarAllocations := lg.varAllocations
	oldVarTypes := lg.varTypes
	lg.varAllocations = make(map[string]string)
	lg.varTypes = make(map[string]typechecker.Type)
	lg.currentFunc = fn.Name

	// Allocate space for parameters on stack and store parameter values
	for i, paramName := range fn.Parameters {
		pType := typechecker.NUMBER_TYPE
		if sig != nil && i < len(sig.ParamTypes) {
			pType = sig.ParamTypes[i]
		}
		tStr := lg.llvmType(pType)
		allocReg := lg.newReg()
		lg.functions.WriteString(fmt.Sprintf("  %s = alloca %s\n", allocReg, tStr))
		lg.functions.WriteString(fmt.Sprintf("  store %s %%.param.%s, %s* %s\n", tStr, paramName, tStr, allocReg))
		lg.varAllocations[paramName] = allocReg
		lg.varTypes[paramName] = pType

		// Retain reference-type parameters passed in
		if lg.isRefType(pType) {
			loadReg := lg.newReg()
			lg.functions.WriteString(fmt.Sprintf("  %s = load i8*, i8** %s\n", loadReg, allocReg))
			lg.functions.WriteString(fmt.Sprintf("  call void @_retain(i8* %s)\n", loadReg))
		}
	}

	// Generate Body
	for _, stmt := range fn.Body.Statements {
		lg.genStatement(stmt, &lg.functions)
	}

	// Default return if not terminated
	if retType == typechecker.VOID_TYPE {
		lg.genReleaseLocals(&lg.functions, false)
		lg.functions.WriteString("  ret void\n")
	} else if retType == typechecker.NUMBER_TYPE {
		lg.genReleaseLocals(&lg.functions, false)
		lg.functions.WriteString("  ret double 0.0\n")
	} else if retType == typechecker.INT_TYPE {
		lg.genReleaseLocals(&lg.functions, false)
		lg.functions.WriteString("  ret i64 0\n")
	} else if retType == typechecker.BOOL_TYPE {
		lg.genReleaseLocals(&lg.functions, false)
		lg.functions.WriteString("  ret i1 0\n")
	} else {
		lg.genReleaseLocals(&lg.functions, false)
		lg.functions.WriteString("  ret i8* null\n")
	}

	lg.functions.WriteString("}\n")

	// Restore
	lg.varAllocations = oldVarAllocations
	lg.varTypes = oldVarTypes
}

func (lg *LlvmGenerator) genStatement(node parser.Statement, buf *strings.Builder) {
	switch n := node.(type) {
	case *parser.BlockStatement:
		for _, s := range n.Statements {
			lg.genStatement(s, buf)
		}

	case *parser.ExpressionStatement:
		lg.genExpression(n.Expression, buf)

	case *parser.VarAssignStatement:
		valReg, valType := lg.genExpression(n.Value, buf)
		existingType, ok := lg.varTypes[n.Name.Value]
		if !ok {
			lg.varTypes[n.Name.Value] = valType
			existingType = valType
			// Create alloca
			allocReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = alloca %s\n", allocReg, lg.llvmType(existingType)))
			// Initialize with null or zero
			if lg.isRefType(existingType) {
				buf.WriteString(fmt.Sprintf("  store i8* null, i8** %s\n", allocReg))
			} else if existingType == typechecker.NUMBER_TYPE {
				buf.WriteString(fmt.Sprintf("  store double 0.0, double* %s\n", allocReg))
			} else if existingType == typechecker.INT_TYPE {
				buf.WriteString(fmt.Sprintf("  store i64 0, i64* %s\n", allocReg))
			} else if existingType == typechecker.BOOL_TYPE {
				buf.WriteString(fmt.Sprintf("  store i1 0, i1* %s\n", allocReg))
			}
			lg.varAllocations[n.Name.Value] = allocReg
		}

		allocReg := lg.varAllocations[n.Name.Value]
		tStr := lg.llvmType(existingType)

		if lg.isRefType(existingType) {
			// Retain new val
			buf.WriteString(fmt.Sprintf("  call void @_retain(i8* %s)\n", valReg))
			// Release old val
			oldVal := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = load i8*, i8** %s\n", oldVal, allocReg))
			buf.WriteString(fmt.Sprintf("  call void @_release(i8* %s)\n", oldVal))
			// Store new
			buf.WriteString(fmt.Sprintf("  store i8* %s, i8** %s\n", valReg, allocReg))
		} else {
			// Store value directly
			buf.WriteString(fmt.Sprintf("  store %s %s, %s* %s\n", tStr, valReg, tStr, allocReg))
		}

	case *parser.StructFieldAssignStatement:
		// target.field = value
		var targetReg string
		var structName string
		if n.Target != nil {
			targetReg, _ = lg.genExpression(n.Target, buf)
			targetType := lg.tc.Check(n.Target, lg.env)
			structName = strings.TrimPrefix(string(targetType), "ҚҰРЫЛЫМ_")
		} else {
			allocReg := lg.varAllocations[n.StructName]
			targetReg = lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = load i8*, i8** %s\n", targetReg, allocReg))
			t, _ := lg.getVarType(n.StructName)
			structName = strings.TrimPrefix(string(t), "ҚҰРЫЛЫМ_")
		}

		// Null check
		nullCond := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = icmp eq i8* %s, null\n", nullCond, targetReg))
		lblErr := lg.newLabel("null_err")
		lblOk := lg.newLabel("null_ok")
		buf.WriteString(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s\n", nullCond, lblErr, lblOk))

		// Null error block
		buf.WriteString(fmt.Sprintf("\n%s:\n", lblErr))
		buf.WriteString("  call void @_runtime_null_pointer_error_c()\n")
		buf.WriteString("  unreachable\n")

		// OK block
		buf.WriteString(fmt.Sprintf("\n%s:\n", lblOk))

		parts := strings.Split(n.Field, ".")
		currentStructType := structName
		var currentReg = targetReg

		for i, part := range parts {
			structDef := lg.structs[currentStructType]
			fieldIdx := -1
			for idx, fName := range structDef.Fields {
				if fName == part {
					fieldIdx = idx
					break
				}
			}
			offset := fieldIdx * 8
			fieldPtrReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = getelementptr i8, i8* %s, i64 %d\n", fieldPtrReg, currentReg, offset))

			if i < len(parts)-1 {
				// Dereference this field to get the next struct pointer
				nextStructPtrReg := lg.newReg()
				fieldPtrCast := lg.newReg()
				buf.WriteString(fmt.Sprintf("  %s = bitcast i8* %s to i8**\n", fieldPtrCast, fieldPtrReg))
				buf.WriteString(fmt.Sprintf("  %s = load i8*, i8** %s\n", nextStructPtrReg, fieldPtrCast))
				
				// Null check next struct
				nullCondNext := lg.newReg()
				buf.WriteString(fmt.Sprintf("  %s = icmp eq i8* %s, null\n", nullCondNext, nextStructPtrReg))
				lblErrNext := lg.newLabel("null_err")
				lblOkNext := lg.newLabel("null_ok")
				buf.WriteString(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s\n", nullCondNext, lblErrNext, lblOkNext))

				buf.WriteString(fmt.Sprintf("\n%s:\n", lblErrNext))
				buf.WriteString("  call void @_runtime_null_pointer_error_c()\n")
				buf.WriteString("  unreachable\n")

				buf.WriteString(fmt.Sprintf("\n%s:\n", lblOkNext))
				
				currentReg = nextStructPtrReg
				currentStructType = structDef.Types[fieldIdx]
			} else {
				// Last part: assign the value
				valReg, _ := lg.genExpression(n.Value, buf)
				fieldTypeStr := structDef.Types[fieldIdx]

				if lg.isRefType(typechecker.Type(fieldTypeStr)) {
					// Release old field value, retain new
					fieldPtrCast := lg.newReg()
					buf.WriteString(fmt.Sprintf("  %s = bitcast i8* %s to i8**\n", fieldPtrCast, fieldPtrReg))
					buf.WriteString(fmt.Sprintf("  call void @_retain(i8* %s)\n", valReg))
					oldVal := lg.newReg()
					buf.WriteString(fmt.Sprintf("  %s = load i8*, i8** %s\n", oldVal, fieldPtrCast))
					buf.WriteString(fmt.Sprintf("  call void @_release(i8* %s)\n", oldVal))
					buf.WriteString(fmt.Sprintf("  store i8* %s, i8** %s\n", valReg, fieldPtrCast))
				} else if strings.HasPrefix(fieldTypeStr, "әлсіз_") {
					// weak assign
					fieldPtrCast := lg.newReg()
					buf.WriteString(fmt.Sprintf("  %s = bitcast i8* %s to i8**\n", fieldPtrCast, fieldPtrReg))
					buf.WriteString(fmt.Sprintf("  call void @_weak_assign(i8** %s, i8* %s)\n", fieldPtrCast, valReg))
				} else {
					// primitive
					tStr := lg.llvmType(typechecker.Type(fieldTypeStr))
					fieldPtrCast := lg.newReg()
					buf.WriteString(fmt.Sprintf("  %s = bitcast i8* %s to %s*\n", fieldPtrCast, fieldPtrReg, tStr))
					buf.WriteString(fmt.Sprintf("  store %s %s, %s* %s\n", tStr, valReg, tStr, fieldPtrCast))
				}
			}
		}

	case *parser.IndexAssignStatement:
		// arr idx val тізім_қой
		arrReg, _ := lg.genExpression(n.Array, buf)
		// Null check
		nullCond := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = icmp eq i8* %s, null\n", nullCond, arrReg))
		lblErr := lg.newLabel("null_err")
		lblOk := lg.newLabel("null_ok")
		buf.WriteString(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s\n", nullCond, lblErr, lblOk))

		// Null error block
		buf.WriteString(fmt.Sprintf("\n%s:\n", lblErr))
		buf.WriteString("  call void @_runtime_null_pointer_error_c()\n")
		buf.WriteString("  unreachable\n")

		// OK block
		buf.WriteString(fmt.Sprintf("\n%s:\n", lblOk))

		idxReg, idxType := lg.genExpression(n.Index, buf)
		var idxRegInt string
		if idxType == typechecker.NUMBER_TYPE {
			idxRegInt = lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = fptosi double %s to i64\n", idxRegInt, idxReg))
		} else {
			idxRegInt = idxReg
		}

		// Bounds check
		lenPtr := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = bitcast i8* %s to i64*\n", lenPtr, arrReg))
		lenVal := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = load i64, i64* %s\n", lenVal, lenPtr))

		lowCond := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = icmp slt i64 %s, 0\n", lowCond, idxRegInt))
		highCond := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = icmp sge i64 %s, %s\n", highCond, idxRegInt, lenVal))
		boundsCond := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = or i1 %s, %s\n", boundsCond, lowCond, highCond))

		lblBoundsErr := lg.newLabel("bounds_err")
		lblBoundsOk := lg.newLabel("bounds_ok")
		buf.WriteString(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s\n", boundsCond, lblBoundsErr, lblBoundsOk))

		buf.WriteString(fmt.Sprintf("\n%s:\n", lblBoundsErr))
		buf.WriteString("  call void @_runtime_array_bounds_error_c()\n")
		buf.WriteString("  unreachable\n")

		buf.WriteString(fmt.Sprintf("\n%s:\n", lblBoundsOk))

		// Calculate index address: elements start at user_ptr + 8
		valReg, valType := lg.genExpression(n.Value, buf)
		baseOffset := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = getelementptr i8, i8* %s, i64 8\n", baseOffset, arrReg))

		if valType == typechecker.NUMBER_TYPE {
			castPtr := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = bitcast i8* %s to double*\n", castPtr, baseOffset))
			targetPtr := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = getelementptr double, double* %s, i64 %s\n", targetPtr, castPtr, idxRegInt))
			buf.WriteString(fmt.Sprintf("  store double %s, double* %s\n", valReg, targetPtr))
		} else {
			// ref type or int
			castPtr := lg.newReg()
			tStr := lg.llvmType(valType)
			buf.WriteString(fmt.Sprintf("  %s = bitcast i8* %s to %s*\n", castPtr, baseOffset, tStr))
			targetPtr := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = getelementptr %s, %s* %s, i64 %s\n", targetPtr, tStr, tStr, castPtr, idxRegInt))

			if lg.isRefType(valType) {
				buf.WriteString(fmt.Sprintf("  call void @_retain(i8* %s)\n", valReg))
				oldVal := lg.newReg()
				buf.WriteString(fmt.Sprintf("  %s = load i8*, i8** %s\n", oldVal, targetPtr))
				buf.WriteString(fmt.Sprintf("  call void @_release(i8* %s)\n", oldVal))
			}
			buf.WriteString(fmt.Sprintf("  store %s %s, %s* %s\n", tStr, valReg, tStr, targetPtr))
		}

	case *parser.FreeStatement:
		valReg, valType := lg.genExpression(n.Value, buf)
		if lg.isRefType(valType) {
			buf.WriteString(fmt.Sprintf("  call void @_release(i8* %s)\n", valReg))
		}

	case *parser.PrintStatement:
		valReg, valType := lg.genExpression(n.Value, buf)
		var formatLabel string
		if valType == typechecker.STRING_TYPE {
			formatLabel = lg.getOrStr("%s\n")
			formatPtr := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = getelementptr { i64, i8*, [4 x i8] }, { i64, i8*, [4 x i8] }* %s, i64 0, i32 2, i64 0\n", formatPtr, formatLabel))
			buf.WriteString(fmt.Sprintf("  call i32 (i8*, ...) @printf(i8* %s, i8* %s)\n", formatPtr, valReg))
		} else if valType == typechecker.INT_TYPE {
			formatLabel = lg.getOrStr("%lld\n")
			formatPtr := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = getelementptr { i64, i8*, [6 x i8] }, { i64, i8*, [6 x i8] }* %s, i64 0, i32 2, i64 0\n", formatPtr, formatLabel))
			buf.WriteString(fmt.Sprintf("  call i32 (i8*, ...) @printf(i8* %s, i64 %s)\n", formatPtr, valReg))
		} else if valType == typechecker.BOOL_TYPE {
			// print true/false or 1/0
			formatLabel = lg.getOrStr("%d\n")
			formatPtr := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = getelementptr { i64, i8*, [4 x i8] }, { i64, i8*, [4 x i8] }* %s, i64 0, i32 2, i64 0\n", formatPtr, formatLabel))
			extReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = zext i1 %s to i32\n", extReg, valReg))
			buf.WriteString(fmt.Sprintf("  call i32 (i8*, ...) @printf(i8* %s, i32 %s)\n", formatPtr, extReg))
		} else if valType == typechecker.NUMBER_TYPE {
			formatLabel = lg.getOrStr("%g\n")
			formatPtr := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = getelementptr { i64, i8*, [4 x i8] }, { i64, i8*, [4 x i8] }* %s, i64 0, i32 2, i64 0\n", formatPtr, formatLabel))
			buf.WriteString(fmt.Sprintf("  call i32 (i8*, ...) @printf(i8* %s, double %s)\n", formatPtr, valReg))
		} else {
			// Fallback (e.g. pointer)
			formatLabel = lg.getOrStr("%p\n")
			formatPtr := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = getelementptr { i64, i8*, [4 x i8] }, { i64, i8*, [4 x i8] }* %s, i64 0, i32 2, i64 0\n", formatPtr, formatLabel))
			buf.WriteString(fmt.Sprintf("  call i32 (i8*, ...) @printf(i8* %s, i8* %s)\n", formatPtr, valReg))
		}

	case *parser.IfStatement:
		condReg, _ := lg.genExpression(n.Condition, buf)
		lblTrue := lg.newLabel("if_true")
		lblFalse := lg.newLabel("if_false")
		lblEnd := lg.newLabel("if_end")

		if n.Alternative != nil {
			buf.WriteString(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s\n", condReg, lblTrue, lblFalse))
		} else {
			buf.WriteString(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s\n", condReg, lblTrue, lblEnd))
		}

		// True block
		buf.WriteString(fmt.Sprintf("\n%s:\n", lblTrue))
		for _, s := range n.Consequence.Statements {
			lg.genStatement(s, buf)
		}
		buf.WriteString(fmt.Sprintf("  br label %%%s\n", lblEnd))

		// False block
		if n.Alternative != nil {
			buf.WriteString(fmt.Sprintf("\n%s:\n", lblFalse))
			for _, s := range n.Alternative.Statements {
				lg.genStatement(s, buf)
			}
			buf.WriteString(fmt.Sprintf("  br label %%%s\n", lblEnd))
		}

		// End block
		buf.WriteString(fmt.Sprintf("\n%s:\n", lblEnd))

	case *parser.WhileStatement:
		lblCond := lg.newLabel("while_cond")
		lblBody := lg.newLabel("while_body")
		lblEnd := lg.newLabel("while_end")

		lg.loopStartLabels = append(lg.loopStartLabels, lblCond)
		lg.loopEndLabels = append(lg.loopEndLabels, lblEnd)

		buf.WriteString(fmt.Sprintf("  br label %%%s\n", lblCond))

		// Condition block
		buf.WriteString(fmt.Sprintf("\n%s:\n", lblCond))
		condReg, _ := lg.genExpression(n.Condition, buf)
		buf.WriteString(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s\n", condReg, lblBody, lblEnd))

		// Body block
		buf.WriteString(fmt.Sprintf("\n%s:\n", lblBody))
		for _, s := range n.Body.Statements {
			lg.genStatement(s, buf)
		}
		buf.WriteString(fmt.Sprintf("  br label %%%s\n", lblCond))

		// End block
		buf.WriteString(fmt.Sprintf("\n%s:\n", lblEnd))

		lg.loopStartLabels = lg.loopStartLabels[:len(lg.loopStartLabels)-1]
		lg.loopEndLabels = lg.loopEndLabels[:len(lg.loopEndLabels)-1]

	case *parser.BreakStatement:
		if len(lg.loopEndLabels) > 0 {
			target := lg.loopEndLabels[len(lg.loopEndLabels)-1]
			buf.WriteString(fmt.Sprintf("  br label %%%s\n", target))
		}

	case *parser.ContinueStatement:
		if len(lg.loopStartLabels) > 0 {
			target := lg.loopStartLabels[len(lg.loopStartLabels)-1]
			buf.WriteString(fmt.Sprintf("  br label %%%s\n", target))
		}

	case *parser.ReturnStatement:
		valReg, valType := lg.genExpression(n.Value, buf)
		lg.genReleaseLocals(buf, false)
		if valType == typechecker.VOID_TYPE {
			buf.WriteString("  ret void\n")
		} else {
			buf.WriteString(fmt.Sprintf("  ret %s %s\n", lg.llvmType(valType), valReg))
		}

	case *parser.CallStatement:
		lg.genExpression(n.Call, buf)

	case *parser.FileWriteStatement:
		// "path" content файл_жазу
		pathReg, _ := lg.genExpression(n.Path, buf)
		contentReg, _ := lg.genExpression(n.Content, buf)

		modeLabel := lg.getOrStr("wb")
		modePtr := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = getelementptr { i64, i8*, [3 x i8] }, { i64, i8*, [3 x i8] }* %s, i64 0, i32 2, i64 0\n", modePtr, modeLabel))

		filePtr := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = call i8* @fopen(i8* %s, i8* %s)\n", filePtr, pathReg, modePtr))

		nullCond := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = icmp eq i8* %s, null\n", nullCond, filePtr))
		lblOk := lg.newLabel("write_ok")
		lblFail := lg.newLabel("write_fail")
		buf.WriteString(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s\n", nullCond, lblFail, lblOk))

		buf.WriteString(fmt.Sprintf("\n%s:\n", lblOk))
		lenReg := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = call i64 @strlen(i8* %s)\n", lenReg, contentReg))
		buf.WriteString(fmt.Sprintf("  call i64 @fwrite(i8* %s, i64 1, i64 %s, i8* %s)\n", contentReg, lenReg, filePtr))
		buf.WriteString(fmt.Sprintf("  call i32 @fclose(i8* %s)\n", filePtr))
		buf.WriteString(fmt.Sprintf("  br label %%%s\n", lblFail))

		buf.WriteString(fmt.Sprintf("\n%s:\n", lblFail))
	}
}

func (lg *LlvmGenerator) genExpression(node parser.Expression, buf *strings.Builder) (string, typechecker.Type) {
	switch n := node.(type) {
	case *parser.IntLiteral:
		return fmt.Sprintf("%d", n.Value), typechecker.INT_TYPE

	case *parser.BoolLiteral:
		if n.Value {
			return "true", typechecker.BOOL_TYPE
		}
		return "false", typechecker.BOOL_TYPE

	case *parser.NumberLiteral:
		return fmt.Sprintf("%f", n.Value), typechecker.NUMBER_TYPE

	case *parser.StringLiteral:
		reg := lg.loadStrLiteral(n.Value, buf)
		return reg, typechecker.STRING_TYPE

	case *parser.Identifier:
		allocReg, ok := lg.varAllocations[n.Value]
		t := lg.varTypes[n.Value]
		if !ok {
			// Maybe it is a global/builtin constant
			return "0", typechecker.UNKNOWN
		}
		loadReg := lg.newReg()
		tStr := lg.llvmType(t)
		buf.WriteString(fmt.Sprintf("  %s = load %s, %s* %s\n", loadReg, tStr, tStr, allocReg))
		if lg.isRefType(t) {
			buf.WriteString(fmt.Sprintf("  call void @_retain(i8* %s)\n", loadReg))
		}
		return loadReg, t

	case *parser.PostfixExpression:
		// Binary Operator
		leftReg, leftType := lg.genExpression(n.Left, buf)
		rightReg, rightType := lg.genExpression(n.Right, buf)

		if leftType == typechecker.INT_TYPE && rightType == typechecker.INT_TYPE {
			switch n.Operator {
			case "қосу":
				reg := lg.newReg()
				buf.WriteString(fmt.Sprintf("  %s = add i64 %s, %s\n", reg, leftReg, rightReg))
				return reg, typechecker.INT_TYPE
			case "алу":
				reg := lg.newReg()
				buf.WriteString(fmt.Sprintf("  %s = sub i64 %s, %s\n", reg, leftReg, rightReg))
				return reg, typechecker.INT_TYPE
			case "көбейту":
				reg := lg.newReg()
				buf.WriteString(fmt.Sprintf("  %s = mul i64 %s, %s\n", reg, leftReg, rightReg))
				return reg, typechecker.INT_TYPE
			case "бөлу":
				// division by zero check
				zeroCond := lg.newReg()
				buf.WriteString(fmt.Sprintf("  %s = icmp eq i64 %s, 0\n", zeroCond, rightReg))
				lblOk := lg.newLabel("div_ok")
				lblErr := lg.newLabel("div_err")
				buf.WriteString(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s\n", zeroCond, lblErr, lblOk))
				buf.WriteString(fmt.Sprintf("\n%s:\n", lblErr))
				buf.WriteString("  call void @_runtime_divide_by_zero_error_c()\n")
				buf.WriteString("  unreachable\n")
				buf.WriteString(fmt.Sprintf("\n%s:\n", lblOk))
				reg := lg.newReg()
				buf.WriteString(fmt.Sprintf("  %s = sdiv i64 %s, %s\n", reg, leftReg, rightReg))
				return reg, typechecker.INT_TYPE
			case "және":
				reg := lg.newReg()
				buf.WriteString(fmt.Sprintf("  %s = and i64 %s, %s\n", reg, leftReg, rightReg))
				return reg, typechecker.INT_TYPE
			case "немесе":
				reg := lg.newReg()
				buf.WriteString(fmt.Sprintf("  %s = or i64 %s, %s\n", reg, leftReg, rightReg))
				return reg, typechecker.INT_TYPE
			case "жылжыту_сол":
				reg := lg.newReg()
				buf.WriteString(fmt.Sprintf("  %s = shl i64 %s, %s\n", reg, leftReg, rightReg))
				return reg, typechecker.INT_TYPE
			case "жылжыту_оң":
				reg := lg.newReg()
				buf.WriteString(fmt.Sprintf("  %s = ashr i64 %s, %s\n", reg, leftReg, rightReg))
				return reg, typechecker.INT_TYPE
			case "тең", "тең_емес", "үлкен", "кіші", "үлкен_тең", "кіші_тең":
				opCode := "eq"
				switch n.Operator {
				case "тең":
					opCode = "eq"
				case "тең_емес":
					opCode = "ne"
				case "үлкен":
					opCode = "sgt"
				case "кіші":
					opCode = "slt"
				case "үлкен_тең":
					opCode = "sge"
				case "кіші_тең":
					opCode = "sle"
				}
				reg := lg.newReg()
				buf.WriteString(fmt.Sprintf("  %s = icmp %s i64 %s, %s\n", reg, opCode, leftReg, rightReg))
				return reg, typechecker.BOOL_TYPE
			}
		}

		if leftType == typechecker.BOOL_TYPE && rightType == typechecker.BOOL_TYPE {
			if n.Operator == "және" {
				reg := lg.newReg()
				buf.WriteString(fmt.Sprintf("  %s = and i1 %s, %s\n", reg, leftReg, rightReg))
				return reg, typechecker.BOOL_TYPE
			} else if n.Operator == "немесе" {
				reg := lg.newReg()
				buf.WriteString(fmt.Sprintf("  %s = or i1 %s, %s\n", reg, leftReg, rightReg))
				return reg, typechecker.BOOL_TYPE
			}
		}

		// Fallback to double (Number)
		leftRegD := leftReg
		if leftType == typechecker.INT_TYPE {
			leftRegD = lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = sitofp i64 %s to double\n", leftRegD, leftReg))
		} else if leftType == typechecker.BOOL_TYPE {
			ext := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = zext i1 %s to i64\n", ext, leftReg))
			leftRegD = lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = sitofp i64 %s to double\n", leftRegD, ext))
		}

		rightRegD := rightReg
		if rightType == typechecker.INT_TYPE {
			rightRegD = lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = sitofp i64 %s to double\n", rightRegD, rightReg))
		} else if rightType == typechecker.BOOL_TYPE {
			ext := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = zext i1 %s to i64\n", ext, rightReg))
			rightRegD = lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = sitofp i64 %s to double\n", rightRegD, ext))
		}

		switch n.Operator {
		case "қосу":
			reg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = fadd double %s, %s\n", reg, leftRegD, rightRegD))
			return reg, typechecker.NUMBER_TYPE
		case "алу":
			reg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = fsub double %s, %s\n", reg, leftRegD, rightRegD))
			return reg, typechecker.NUMBER_TYPE
		case "көбейту":
			reg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = fmul double %s, %s\n", reg, leftRegD, rightRegD))
			return reg, typechecker.NUMBER_TYPE
		case "бөлу":
			// division by zero check
			zeroCond := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = fcmp oeq double %s, 0.0\n", zeroCond, rightRegD))
			lblOk := lg.newLabel("div_ok")
			lblErr := lg.newLabel("div_err")
			buf.WriteString(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s\n", zeroCond, lblErr, lblOk))
			buf.WriteString(fmt.Sprintf("\n%s:\n", lblErr))
			buf.WriteString("  call void @_runtime_divide_by_zero_error_c()\n")
			buf.WriteString("  unreachable\n")
			buf.WriteString(fmt.Sprintf("\n%s:\n", lblOk))
			reg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = fdiv double %s, %s\n", reg, leftRegD, rightRegD))
			return reg, typechecker.NUMBER_TYPE
		case "тең", "тең_емес", "үлкен", "кіші", "үлкен_тең", "кіші_тең":
			opCode := "oeq"
			switch n.Operator {
			case "тең":
				opCode = "oeq"
			case "тең_емес":
				opCode = "une"
			case "үлкен":
				opCode = "ogt"
			case "кіші":
				opCode = "olt"
			case "үлкен_тең":
				opCode = "oge"
			case "кіші_тең":
				opCode = "ole"
			}
			reg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = fcmp %s double %s, %s\n", reg, opCode, leftRegD, rightRegD))
			return reg, typechecker.BOOL_TYPE
		}
		return "0.0", typechecker.NUMBER_TYPE

	case *parser.UnaryExpression:
		// емес (Logical NOT)
		rightReg, _ := lg.genExpression(n.Right, buf)
		reg := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = xor i1 %s, true\n", reg, rightReg))
		return reg, typechecker.BOOL_TYPE

	case *parser.CallExpression:
		// Standard calls or user function calls
		sig := lg.funcs[n.Function]

		// Evaluate arguments
		var argRegs []string
		var argTypes []typechecker.Type
		for _, argExpr := range n.Arguments {
			argReg, argType := lg.genExpression(argExpr, buf)
			argRegs = append(argRegs, argReg)
			argTypes = append(argTypes, argType)
		}

		// Built-ins or User function
		switch n.Function {
		case "жсон_оқу":
			resReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = call i8* @builtin_json_parse(i8* %s)\n", resReg, argRegs[0]))
			return resReg, typechecker.JSON_TYPE
		case "жсон_жазу":
			resReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = call i8* @builtin_json_write(i8* %s)\n", resReg, argRegs[0]))
			return resReg, typechecker.STRING_TYPE
		case "жсон_сан_алу":
			resReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = call double @builtin_json_get_number(i8* %s, i8* %s)\n", resReg, argRegs[0], argRegs[1]))
			return resReg, typechecker.NUMBER_TYPE
		case "жсон_мәтін_алу":
			resReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = call i8* @builtin_json_get_string(i8* %s, i8* %s)\n", resReg, argRegs[0], argRegs[1]))
			return resReg, typechecker.STRING_TYPE
		case "жсон_логика_алу":
			resReg := lg.newReg()
			boolValVal := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = call i64 @builtin_json_get_bool(i8* %s, i8* %s)\n", boolValVal, argRegs[0], argRegs[1]))
			buf.WriteString(fmt.Sprintf("  %s = icmp ne i64 %s, 0\n", resReg, boolValVal))
			return resReg, typechecker.BOOL_TYPE
		case "жсон_нысан_алу":
			resReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = call i8* @builtin_json_get_object(i8* %s, i8* %s)\n", resReg, argRegs[0], argRegs[1]))
			return resReg, typechecker.JSON_TYPE
		case "жсон_тізім_алу":
			resReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = call i8* @builtin_json_get_list(i8* %s, i8* %s)\n", resReg, argRegs[0], argRegs[1]))
			return resReg, typechecker.JSON_TYPE
		case "жсон_тізім_өлшемі":
			resReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = call double @builtin_json_list_size(i8* %s)\n", resReg, argRegs[0]))
			return resReg, typechecker.NUMBER_TYPE
		case "жсон_тізім_элементі":
			resReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = call i8* @builtin_json_list_get(i8* %s, double %s)\n", resReg, argRegs[0], argRegs[1]))
			return resReg, typechecker.JSON_TYPE
		case "жсон_жаңа":
			resReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = call i8* @builtin_json_create()\n", resReg))
			return resReg, typechecker.JSON_TYPE
		case "жсон_сан_қосу":
			buf.WriteString(fmt.Sprintf("  call void @builtin_json_add_number(i8* %s, i8* %s, double %s)\n", argRegs[0], argRegs[1], argRegs[2]))
			return "0.0", typechecker.VOID_TYPE
		case "жсон_мәтін_қосу":
			buf.WriteString(fmt.Sprintf("  call void @builtin_json_add_string(i8* %s, i8* %s, i8* %s)\n", argRegs[0], argRegs[1], argRegs[2]))
			return "0.0", typechecker.VOID_TYPE
		case "жсон_логика_қосу":
			ext := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = zext i1 %s to i64\n", ext, argRegs[2]))
			buf.WriteString(fmt.Sprintf("  call void @builtin_json_add_bool(i8* %s, i8* %s, i64 %s)\n", argRegs[0], argRegs[1], ext))
			return "0.0", typechecker.VOID_TYPE
		case "жсон_нысан_қосу":
			buf.WriteString(fmt.Sprintf("  call void @builtin_json_add_object(i8* %s, i8* %s, i8* %s)\n", argRegs[0], argRegs[1], argRegs[2]))
			return "0.0", typechecker.VOID_TYPE
		case "жсон_тізім_қосу":
			buf.WriteString(fmt.Sprintf("  call void @builtin_json_add_list(i8* %s, i8* %s, i8* %s)\n", argRegs[0], argRegs[1], argRegs[2]))
			return "0.0", typechecker.VOID_TYPE
		case "мд5":
			resReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = call i8* @builtin_md5(i8* %s)\n", resReg, argRegs[0]))
			return resReg, typechecker.STRING_TYPE
		case "ша256":
			resReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = call i8* @builtin_sha256(i8* %s)\n", resReg, argRegs[0]))
			return resReg, typechecker.STRING_TYPE
		case "б64_кодтау":
			resReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = call i8* @builtin_base64_encode(i8* %s)\n", resReg, argRegs[0]))
			return resReg, typechecker.STRING_TYPE
		case "б64_декодтау":
			resReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = call i8* @builtin_base64_decode(i8* %s)\n", resReg, argRegs[0]))
			return resReg, typechecker.STRING_TYPE
		case "мәтін":
			resReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = call i8* @builtin_num_to_str(double %s)\n", resReg, argRegs[0]))
			return resReg, typechecker.STRING_TYPE
		case "сан":
			resReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = call double @builtin_str_to_num(i8* %s)\n", resReg, argRegs[0]))
			return resReg, typechecker.NUMBER_TYPE
		case "түбір":
			resReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = call double @builtin_sqrt(double %s)\n", resReg, argRegs[0]))
			return resReg, typechecker.NUMBER_TYPE
		case "пи":
			resReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = call double @builtin_math_pi()\n", resReg))
			return resReg, typechecker.NUMBER_TYPE
		case "экспонента":
			resReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = call double @builtin_math_exp(double %s)\n", resReg, argRegs[0]))
			return resReg, typechecker.NUMBER_TYPE
		case "жүйе":
			resReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = call double @builtin_system(i8* %s)\n", resReg, argRegs[0]))
			return resReg, typechecker.NUMBER_TYPE
		case "файл_жою":
			resReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = call double @builtin_file_delete(i8* %s)\n", resReg, argRegs[0]))
			return resReg, typechecker.NUMBER_TYPE
		case "файл_бар_ма":
			resReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = call double @builtin_file_exists(i8* %s)\n", resReg, argRegs[0]))
			return resReg, typechecker.NUMBER_TYPE
		case "аргумент_саны":
			resReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = call double @builtin_args_count()\n", resReg))
			return resReg, typechecker.NUMBER_TYPE
		case "аргумент":
			resReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = call i8* @builtin_arg_get(double %s)\n", resReg, argRegs[0]))
			return resReg, typechecker.STRING_TYPE
		case "уақыт":
			resReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = call double @builtin_time_seconds()\n", resReg))
			return resReg, typechecker.NUMBER_TYPE
		case "уақыт_мәтіні":
			resReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = call i8* @builtin_time_str(i8* %s)\n", resReg, argRegs[0]))
			return resReg, typechecker.STRING_TYPE
		case "ұйықтау":
			buf.WriteString(fmt.Sprintf("  call void @builtin_sleep_seconds(double %s)\n", argRegs[0]))
			return "0.0", typechecker.VOID_TYPE
		case "ағын_күту":
			// joins thread
			intHandle := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = fptosi double %s to i64\n", intHandle, argRegs[0]))
			resReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = call i64 @builtin_thread_join(i64 %s)\n", resReg, intHandle))
			// convert to double
			doubleReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = sitofp i64 %s to double\n", doubleReg, resReg))
			return doubleReg, typechecker.NUMBER_TYPE

		default:
			// User defined function call
			var typedArgs []string
			for idx, aReg := range argRegs {
				pT := typechecker.NUMBER_TYPE
				if sig != nil && idx < len(sig.ParamTypes) {
					pT = sig.ParamTypes[idx]
				}
				typedArgs = append(typedArgs, fmt.Sprintf("%s %s", lg.llvmType(pT), aReg))
			}
			retType := typechecker.NUMBER_TYPE
			retTypeStr := "double"
			if sig != nil {
				retType = sig.ReturnType
				retTypeStr = lg.llvmType(retType)
			}
			if retType == typechecker.VOID_TYPE {
				buf.WriteString(fmt.Sprintf("  call void @%s(%s)\n", n.Function, strings.Join(typedArgs, ", ")))
				return "0.0", typechecker.VOID_TYPE
			} else {
				resReg := lg.newReg()
				buf.WriteString(fmt.Sprintf("  %s = call %s @%s(%s)\n", resReg, retTypeStr, n.Function, strings.Join(typedArgs, ", ")))
				return resReg, retType
			}
		}

	case *parser.StructCreateExpression:
		// Allocates struct and sets destructor callback
		structDef := lg.structs[n.StructName]
		size := len(structDef.Fields) * 8

		destructorFnName := fmt.Sprintf("@_destructor_struct_%s", n.StructName)
		castDestructor := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = bitcast void (i8*)* %s to void (i8*)*\n", castDestructor, destructorFnName))

		resReg := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = call i8* @_alloc_ref(i64 %d, void (i8*)* %s)\n", resReg, size, castDestructor))
		return resReg, typechecker.Type("ҚҰРЫЛЫМ_" + n.StructName)

	case *parser.StructFieldAccessExpression:
		// target.field
		var targetReg string
		var structName string
		if n.Target != nil {
			targetReg, _ = lg.genExpression(n.Target, buf)
			targetType := lg.tc.Check(n.Target, lg.env)
			structName = strings.TrimPrefix(string(targetType), "ҚҰРЫЛЫМ_")
		} else {
			allocReg := lg.varAllocations[n.StructName]
			targetReg = lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = load i8*, i8** %s\n", targetReg, allocReg))
			t, _ := lg.getVarType(n.StructName)
			structName = strings.TrimPrefix(string(t), "ҚҰРЫЛЫМ_")
		}

		// Null check
		nullCond := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = icmp eq i8* %s, null\n", nullCond, targetReg))
		lblErr := lg.newLabel("null_err")
		lblOk := lg.newLabel("null_ok")
		buf.WriteString(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s\n", nullCond, lblErr, lblOk))

		// Null error block
		buf.WriteString(fmt.Sprintf("\n%s:\n", lblErr))
		buf.WriteString("  call void @_runtime_null_pointer_error_c()\n")
		buf.WriteString("  unreachable\n")

		// OK block
		buf.WriteString(fmt.Sprintf("\n%s:\n", lblOk))

		parts := strings.Split(n.Field, ".")
		currentStructType := structName
		var currentReg = targetReg
		var finalReg = targetReg
		var finalType string

		for i, part := range parts {
			structDef := lg.structs[currentStructType]
			fieldIdx := -1
			for idx, fName := range structDef.Fields {
				if fName == part {
					fieldIdx = idx
					break
				}
			}
			offset := fieldIdx * 8
			fieldPtrReg := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = getelementptr i8, i8* %s, i64 %d\n", fieldPtrReg, currentReg, offset))

			fieldTypeStr := structDef.Types[fieldIdx]
			finalType = fieldTypeStr

			if i < len(parts)-1 {
				// Dereference this field to get the next struct pointer
				nextStructPtrReg := lg.newReg()
				fieldPtrCast := lg.newReg()
				buf.WriteString(fmt.Sprintf("  %s = bitcast i8* %s to i8**\n", fieldPtrCast, fieldPtrReg))
				buf.WriteString(fmt.Sprintf("  %s = load i8*, i8** %s\n", nextStructPtrReg, fieldPtrCast))
				
				// Null check next struct
				nullCondNext := lg.newReg()
				buf.WriteString(fmt.Sprintf("  %s = icmp eq i8* %s, null\n", nullCondNext, nextStructPtrReg))
				lblErrNext := lg.newLabel("null_err")
				lblOkNext := lg.newLabel("null_ok")
				buf.WriteString(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s\n", nullCondNext, lblErrNext, lblOkNext))

				buf.WriteString(fmt.Sprintf("\n%s:\n", lblErrNext))
				buf.WriteString("  call void @_runtime_null_pointer_error_c()\n")
				buf.WriteString("  unreachable\n")

				buf.WriteString(fmt.Sprintf("\n%s:\n", lblOkNext))
				
				currentReg = nextStructPtrReg
				currentStructType = fieldTypeStr
			} else {
				// Last field
				if lg.isRefType(typechecker.Type(fieldTypeStr)) {
					fieldPtrCast := lg.newReg()
					buf.WriteString(fmt.Sprintf("  %s = bitcast i8* %s to i8**\n", fieldPtrCast, fieldPtrReg))
					loadReg := lg.newReg()
					buf.WriteString(fmt.Sprintf("  %s = load i8*, i8** %s\n", loadReg, fieldPtrCast))
					buf.WriteString(fmt.Sprintf("  call void @_retain(i8* %s)\n", loadReg))
					finalReg = loadReg
				} else if strings.HasPrefix(fieldTypeStr, "әлсіз_") {
					fieldPtrCast := lg.newReg()
					buf.WriteString(fmt.Sprintf("  %s = bitcast i8* %s to i8**\n", fieldPtrCast, fieldPtrReg))
					loadReg := lg.newReg()
					buf.WriteString(fmt.Sprintf("  %s = call i8* @_weak_load(i8** %s)\n", loadReg, fieldPtrCast))
					// _weak_load returns a retained strong pointer (or NULL)
					finalReg = loadReg
					finalType = "ҚҰРЫЛЫМ_" + fieldTypeStr[len("әлсіз_"):]
				} else {
					tStr := lg.llvmType(typechecker.Type(fieldTypeStr))
					fieldPtrCast := lg.newReg()
					buf.WriteString(fmt.Sprintf("  %s = bitcast i8* %s to %s*\n", fieldPtrCast, fieldPtrReg, tStr))
					loadReg := lg.newReg()
					buf.WriteString(fmt.Sprintf("  %s = load %s, %s* %s\n", loadReg, tStr, tStr, fieldPtrCast))
					finalReg = loadReg
				}
			}
		}

		return finalReg, typechecker.Type(finalType)

	case *parser.ArrayLiteral:
		// Evaluates elements, allocates buffer, and fills
		nElems := len(n.Elements)
		allocSize := 8 + nElems*8

		resReg := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = call i8* @_alloc_ref(i64 %d, void (i8*)* null)\n", resReg, allocSize))

		// Store array length in the first 8 bytes of payload
		lenPtr := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = bitcast i8* %s to i64*\n", lenPtr, resReg))
		buf.WriteString(fmt.Sprintf("  store i64 %d, i64* %s\n", nElems, lenPtr))

		// Get pointer to elements base (user_ptr + 8)
		baseOffset := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = getelementptr i8, i8* %s, i64 8\n", baseOffset, resReg))

		for idx, elemExpr := range n.Elements {
			elemReg, elemType := lg.genExpression(elemExpr, buf)
			tStr := lg.llvmType(elemType)

			elemPtrCast := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = bitcast i8* %s to %s*\n", elemPtrCast, baseOffset, tStr))

			elemPtr := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = getelementptr %s, %s* %s, i64 %d\n", elemPtr, tStr, tStr, elemPtrCast, idx))

			if lg.isRefType(elemType) {
				buf.WriteString(fmt.Sprintf("  call void @_retain(i8* %s)\n", elemReg))
			}
			buf.WriteString(fmt.Sprintf("  store %s %s, %s* %s\n", tStr, elemReg, tStr, elemPtr))
		}
		return resReg, typechecker.ARRAY_TYPE

	case *parser.IndexExpression:
		// arr idx алу
		arrReg, _ := lg.genExpression(n.Left, buf)
		// Null check
		nullCond := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = icmp eq i8* %s, null\n", nullCond, arrReg))
		lblErr := lg.newLabel("null_err")
		lblOk := lg.newLabel("null_ok")
		buf.WriteString(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s\n", nullCond, lblErr, lblOk))

		// Null error block
		buf.WriteString(fmt.Sprintf("\n%s:\n", lblErr))
		buf.WriteString("  call void @_runtime_null_pointer_error_c()\n")
		buf.WriteString("  unreachable\n")

		// OK block
		buf.WriteString(fmt.Sprintf("\n%s:\n", lblOk))

		idxReg, idxType := lg.genExpression(n.Index, buf)
		var idxRegInt string
		if idxType == typechecker.NUMBER_TYPE {
			idxRegInt = lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = fptosi double %s to i64\n", idxRegInt, idxReg))
		} else {
			idxRegInt = idxReg
		}

		// Bounds check
		lenPtr := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = bitcast i8* %s to i64*\n", lenPtr, arrReg))
		lenVal := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = load i64, i64* %s\n", lenVal, lenPtr))

		lowCond := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = icmp slt i64 %s, 0\n", lowCond, idxRegInt))
		highCond := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = icmp sge i64 %s, %s\n", highCond, idxRegInt, lenVal))
		boundsCond := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = or i1 %s, %s\n", boundsCond, lowCond, highCond))

		lblBoundsErr := lg.newLabel("bounds_err")
		lblBoundsOk := lg.newLabel("bounds_ok")
		buf.WriteString(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s\n", boundsCond, lblBoundsErr, lblBoundsOk))

		buf.WriteString(fmt.Sprintf("\n%s:\n", lblBoundsErr))
		buf.WriteString("  call void @_runtime_array_bounds_error_c()\n")
		buf.WriteString("  unreachable\n")

		buf.WriteString(fmt.Sprintf("\n%s:\n", lblBoundsOk))

		baseOffset := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = getelementptr i8, i8* %s, i64 8\n", baseOffset, arrReg))

		arrType := lg.tc.Check(n.Left, lg.env)
		// Determine array element type
		elemType := typechecker.NUMBER_TYPE
		if strings.HasPrefix(string(arrType), "ТІЗІМ_") {
			elemType = typechecker.Type(strings.TrimPrefix(string(arrType), "ТІЗІМ_"))
		}

		tStr := lg.llvmType(elemType)
		elemPtrCast := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = bitcast i8* %s to %s*\n", elemPtrCast, baseOffset, tStr))
		elemPtr := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = getelementptr %s, %s* %s, i64 %s\n", elemPtr, tStr, tStr, elemPtrCast, idxRegInt))

		loadReg := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = load %s, %s* %s\n", loadReg, tStr, tStr, elemPtr))

		if lg.isRefType(elemType) {
			buf.WriteString(fmt.Sprintf("  call void @_retain(i8* %s)\n", loadReg))
		}

		return loadReg, elemType

	case *parser.LengthExpression:
		// Array length
		arrReg, _ := lg.genExpression(n.Value, buf)
		// Null check
		nullCond := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = icmp eq i8* %s, null\n", nullCond, arrReg))
		lblErr := lg.newLabel("null_err")
		lblOk := lg.newLabel("null_ok")
		buf.WriteString(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s\n", nullCond, lblErr, lblOk))

		// Null error block
		buf.WriteString(fmt.Sprintf("\n%s:\n", lblErr))
		buf.WriteString("  call void @_runtime_null_pointer_error_c()\n")
		buf.WriteString("  unreachable\n")

		// OK block
		buf.WriteString(fmt.Sprintf("\n%s:\n", lblOk))

		lenPtr := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = bitcast i8* %s to i64*\n", lenPtr, arrReg))
		lenVal := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = load i64, i64* %s\n", lenVal, lenPtr))
		// Convert to double
		doubleReg := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = sitofp i64 %s to double\n", doubleReg, lenVal))
		return doubleReg, typechecker.NUMBER_TYPE

	case *parser.CharAtExpression:
		// str idx символ
		strReg, _ := lg.genExpression(n.Str, buf)
		idxReg, idxType := lg.genExpression(n.Index, buf)
		var idxRegInt string
		if idxType == typechecker.NUMBER_TYPE {
			idxRegInt = lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = fptosi double %s to i64\n", idxRegInt, idxReg))
		} else {
			idxRegInt = idxReg
		}

		// Allocate new string for char (2 bytes: char + null terminator)
		resReg := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = call i8* @_alloc_ref(i64 2, void (i8*)* null)\n", resReg))

		// Load character
		charPtr := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = getelementptr i8, i8* %s, i64 %s\n", charPtr, strReg, idxRegInt))
		charVal := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = load i8, i8* %s\n", charVal, charPtr))

		// Store character at resReg
		buf.WriteString(fmt.Sprintf("  store i8 %s, i8* %s\n", charVal, resReg))
		// Null terminator
		nullTermPtr := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = getelementptr i8, i8* %s, i64 1\n", nullTermPtr, resReg))
		buf.WriteString(fmt.Sprintf("  store i8 0, i8* %s\n", nullTermPtr))

		return resReg, typechecker.STRING_TYPE

	case *parser.StrConcatExpression:
		// str1 str2 біріктіру
		leftReg, _ := lg.genExpression(n.Left, buf)
		rightReg, _ := lg.genExpression(n.Right, buf)

		lenLeft := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = call i64 @strlen(i8* %s)\n", lenLeft, leftReg))
		lenRight := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = call i64 @strlen(i8* %s)\n", lenRight, rightReg))

		totalLen := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = add i64 %s, %s\n", totalLen, lenLeft, lenRight))
		allocLen := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = add i64 %s, 1\n", allocLen, totalLen))

		resReg := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = call i8* @_alloc_ref(i64 %s, void (i8*)* null)\n", resReg, allocLen))

		buf.WriteString(fmt.Sprintf("  call i8* @strcpy(i8* %s, i8* %s)\n", resReg, leftReg))
		buf.WriteString(fmt.Sprintf("  call i8* @strcat(i8* %s, i8* %s)\n", resReg, rightReg))

		return resReg, typechecker.STRING_TYPE

	case *parser.StrLenExpression:
		// str ұзындық_жол
		strReg, _ := lg.genExpression(n.Value, buf)
		resReg := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = call i64 @strlen(i8* %s)\n", resReg, strReg))
		doubleReg := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = sitofp i64 %s to double\n", doubleReg, resReg))
		return doubleReg, typechecker.NUMBER_TYPE

	case *parser.StrEqExpression:
		// str1 str2 мәтін_тең
		leftReg, _ := lg.genExpression(n.Left, buf)
		rightReg, _ := lg.genExpression(n.Right, buf)

		cmpRes := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = call i32 @strcmp(i8* %s, i8* %s)\n", cmpRes, leftReg, rightReg))
		eqCond := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = icmp eq i32 %s, 0\n", eqCond, cmpRes))
		return eqCond, typechecker.BOOL_TYPE

	case *parser.ToStrExpression:
		// num санды_мәтін
		valReg, valType := lg.genExpression(n.Value, buf)

		resReg := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = call i8* @_alloc_ref(i64 64, void (i8*)* null)\n", resReg))

		if valType == typechecker.INT_TYPE {
			formatLabel := lg.getOrStr("%lld")
			formatPtr := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = getelementptr { i64, i8*, [5 x i8] }, { i64, i8*, [5 x i8] }* %s, i64 0, i32 2, i64 0\n", formatPtr, formatLabel))
			buf.WriteString(fmt.Sprintf("  call i32 (i8*, i8*, ...) @sprintf(i8* %s, i8* %s, i64 %s)\n", resReg, formatPtr, valReg))
		} else if valType == typechecker.BOOL_TYPE {
			ext := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = zext i1 %s to i64\n", ext, valReg))
			formatLabel := lg.getOrStr("%lld")
			formatPtr := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = getelementptr { i64, i8*, [5 x i8] }, { i64, i8*, [5 x i8] }* %s, i64 0, i32 2, i64 0\n", formatPtr, formatLabel))
			buf.WriteString(fmt.Sprintf("  call i32 (i8*, i8*, ...) @sprintf(i8* %s, i8* %s, i64 %s)\n", resReg, formatPtr, ext))
		} else {
			// float / double
			formatLabel := lg.getOrStr("%g")
			formatPtr := lg.newReg()
			buf.WriteString(fmt.Sprintf("  %s = getelementptr { i64, i8*, [3 x i8] }, { i64, i8*, [3 x i8] }* %s, i64 0, i32 2, i64 0\n", formatPtr, formatLabel))
			buf.WriteString(fmt.Sprintf("  call i32 (i8*, i8*, ...) @sprintf(i8* %s, i8* %s, double %s)\n", resReg, formatPtr, valReg))
		}
		return resReg, typechecker.STRING_TYPE

	case *parser.CharCodeExpression:
		// str таңба_коды
		strReg, _ := lg.genExpression(n.Value, buf)
		nullCond := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = icmp eq i8* %s, null\n", nullCond, strReg))
		lblErr := lg.newLabel("null_err")
		lblOk := lg.newLabel("null_ok")
		buf.WriteString(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s\n", nullCond, lblErr, lblOk))

		buf.WriteString(fmt.Sprintf("\n%s:\n", lblErr))
		buf.WriteString("  call void @_runtime_null_pointer_error_c()\n")
		buf.WriteString("  unreachable\n")

		buf.WriteString(fmt.Sprintf("\n%s:\n", lblOk))

		charVal := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = load i8, i8* %s\n", charVal, strReg))
		extVal := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = zext i8 %s to i64\n", extVal, charVal))
		doubleReg := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = sitofp i64 %s to double\n", doubleReg, extVal))
		return doubleReg, typechecker.NUMBER_TYPE

	case *parser.FileReadExpression:
		// "path" файл_оқу
		pathReg, _ := lg.genExpression(n.Path, buf)

		modeLabel := lg.getOrStr("rb")
		modePtr := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = getelementptr { i64, i8*, [3 x i8] }, { i64, i8*, [3 x i8] }* %s, i64 0, i32 2, i64 0\n", modePtr, modeLabel))

		filePtr := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = call i8* @fopen(i8* %s, i8* %s)\n", filePtr, pathReg, modePtr))

		nullCond := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = icmp eq i8* %s, null\n", nullCond, filePtr))
		lblOk := lg.newLabel("read_ok")
		lblFail := lg.newLabel("read_fail")
		lblDone := lg.newLabel("read_done")
		buf.WriteString(fmt.Sprintf("  br i1 %s, label %%%s, label %%%s\n", nullCond, lblFail, lblOk))

		// Fail block
		buf.WriteString(fmt.Sprintf("\n%s:\n", lblFail))
		buf.WriteString(fmt.Sprintf("  br label %%%s\n", lblDone))

		// Ok block
		buf.WriteString(fmt.Sprintf("\n%s:\n", lblOk))
		// seek to end
		buf.WriteString(fmt.Sprintf("  call i64 @fseek(i8* %s, i64 0, i32 2)\n", filePtr))
		sizeReg := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = call i64 @ftell(i8* %s)\n", sizeReg, filePtr))
		buf.WriteString(fmt.Sprintf("  call i64 @fseek(i8* %s, i64 0, i32 0)\n", filePtr))

		allocSize := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = add i64 %s, 1\n", allocSize, sizeReg))
		bufReg := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = call i8* @_alloc_ref(i64 %s, void (i8*)* null)\n", bufReg, allocSize))

		buf.WriteString(fmt.Sprintf("  call i64 @fread(i8* %s, i64 1, i64 %s, i8* %s)\n", bufReg, sizeReg, filePtr))
		// null-terminate
		nullTerm := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = getelementptr i8, i8* %s, i64 %s\n", nullTerm, bufReg, sizeReg))
		buf.WriteString(fmt.Sprintf("  store i8 0, i8* %s\n", nullTerm))
		buf.WriteString(fmt.Sprintf("  call i32 @fclose(i8* %s)\n", filePtr))
		buf.WriteString(fmt.Sprintf("  br label %%%s\n", lblDone))

		// Done block
		buf.WriteString(fmt.Sprintf("\n%s:\n", lblDone))
		phiReg := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = phi i8* [ null, %%%s ], [ %s, %%%s ]\n", phiReg, lblFail, bufReg, lblOk))

		return phiReg, typechecker.STRING_TYPE

	case *parser.ThreadStatement:
		// Spawn thread: spawn thread wrapper function
		threadID := lg.nextThreadID
		lg.nextThreadID++

		var bodyStmts []parser.Statement
		if block, ok := n.Body.(*parser.BlockStatement); ok {
			bodyStmts = block.Statements
		} else if exprStmt, ok := n.Body.(*parser.ExpressionStatement); ok {
			bodyStmts = []parser.Statement{exprStmt}
		} else {
			bodyStmts = []parser.Statement{n.Body}
		}

		wrapperLabel := fmt.Sprintf("_thread_wrapper_%d", threadID)

		// Create thread wrapper function
		lg.threadFuncs.WriteString(fmt.Sprintf("\ndefine i8* @%s(i8* %%arg) {\n", wrapperLabel))

		// Save state
		oldVarAllocations := lg.varAllocations
		oldVarTypes := lg.varTypes
		oldFunc := lg.currentFunc
		lg.varAllocations = make(map[string]string)
		lg.varTypes = make(map[string]typechecker.Type)
		lg.currentFunc = wrapperLabel

		for _, stmt := range bodyStmts {
			lg.genStatement(stmt, &lg.threadFuncs)
		}

		lg.genReleaseLocals(&lg.threadFuncs, false)
		lg.threadFuncs.WriteString("  ret i8* null\n")
		lg.threadFuncs.WriteString("}\n")

		// Restore state
		lg.varAllocations = oldVarAllocations
		lg.varTypes = oldVarTypes
		lg.currentFunc = oldFunc

		// Call spawn
		fnPtrReg := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = bitcast i8* (i8*)* @%s to i8* (i8*)*\n", fnPtrReg, wrapperLabel))
		spawnReg := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = call i64 @_thread_spawn(i8* (i8*)* %s, i8* null)\n", spawnReg, fnPtrReg))
		// Convert handle to double and return in xmm0 equivalent
		doubleReg := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = sitofp i64 %s to double\n", doubleReg, spawnReg))
		return doubleReg, typechecker.NUMBER_TYPE

	case *parser.InputExpression:
		// кіру -> reads a line from stdin
		resReg := lg.newReg()
		buf.WriteString(fmt.Sprintf("  %s = call i8* @builtin_input()\n", resReg))
		return resReg, typechecker.STRING_TYPE
	}

	return "0.0", typechecker.NUMBER_TYPE
}
