package typechecker

import (
	"butaq/parser"
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type Type string

const (
	NUMBER_TYPE Type = "САН"
	STRING_TYPE Type = "МӘТІН"
	BOOL_TYPE   Type = "АҚИҚАТ"
	INT_TYPE    Type = "БҮТІН"
	BYTE_TYPE   Type = "БАЙТ"
	ARRAY_TYPE  Type = "ТІЗІМ"
	STRUCT_TYPE Type = "ҚҰРЫЛЫМ"
	VOID_TYPE   Type = "БОС"
	UNKNOWN     Type = "БЕЛГІСІЗ"
)

// ---------------------------------------------------------------------------
// Type Environment (scoped)
// ---------------------------------------------------------------------------

type TypeEnv struct {
	store map[string]Type
	outer *TypeEnv
}

func NewTypeEnv() *TypeEnv {
	return &TypeEnv{store: make(map[string]Type)}
}

func NewEnclosedTypeEnv(outer *TypeEnv) *TypeEnv {
	env := NewTypeEnv()
	env.outer = outer
	return env
}

func (e *TypeEnv) Get(name string) (Type, bool) {
	t, ok := e.store[name]
	if !ok && e.outer != nil {
		return e.outer.Get(name)
	}
	return t, ok
}

func (e *TypeEnv) Set(name string, val Type) {
	e.store[name] = val
}

// SetLocal sets only in the current scope (not outer)
func (e *TypeEnv) SetLocal(name string, val Type) {
	e.store[name] = val
}

// ---------------------------------------------------------------------------
// Function signatures
// ---------------------------------------------------------------------------

type FuncSig struct {
	Params     []string
	ParamTypes []Type
	ReturnType Type
}

// ---------------------------------------------------------------------------
// TypeChecker
// ---------------------------------------------------------------------------

type TypeChecker struct {
	globalEnv   *TypeEnv
	funcs       map[string]*FuncSig
	structs     map[string]*parser.StructStatement
	Errors      []string
	currentFunc string // tracks which function we are checking
}

func New() *TypeChecker {
	tc := &TypeChecker{
		globalEnv: NewTypeEnv(),
		funcs:     make(map[string]*FuncSig),
		structs:   make(map[string]*parser.StructStatement),
		Errors:    []string{},
	}

	tc.funcs["мәтін_ұзындығы"] = &FuncSig{
		Params:     []string{"мәтін"},
		ParamTypes: []Type{STRING_TYPE},
		ReturnType: NUMBER_TYPE,
	}
	tc.funcs["таңба"] = &FuncSig{
		Params:     []string{"код"},
		ParamTypes: []Type{INT_TYPE},
		ReturnType: STRING_TYPE,
	}
	tc.funcs["бүтін"] = &FuncSig{
		Params:     []string{"сан"},
		ParamTypes: []Type{NUMBER_TYPE},
		ReturnType: INT_TYPE,
	}
	tc.funcs["кездейсоқ"] = &FuncSig{
		Params:     []string{},
		ParamTypes: []Type{},
		ReturnType: NUMBER_TYPE,
	}

	return tc
}

// GetStructs returns the struct definitions (used by codegen)
func (tc *TypeChecker) GetStructs() map[string]*parser.StructStatement {
	return tc.structs
}

func (tc *TypeChecker) errorf(format string, args ...interface{}) {
	tc.Errors = append(tc.Errors, fmt.Sprintf("Тип қатесі: "+format, args...))
}

// GetFuncs returns the function signature map (used by codegen)
func (tc *TypeChecker) GetFuncs() map[string]*FuncSig {
	return tc.funcs
}

// ---------------------------------------------------------------------------
// Check entry point
// ---------------------------------------------------------------------------

func (tc *TypeChecker) Check(node parser.Node, env *TypeEnv) Type {
	switch node := node.(type) {

	case *parser.Program:
		// First pass: register all function signatures and structs
		for _, stmt := range node.Statements {
			if fs, ok := stmt.(*parser.FunctionStatement); ok {
				tc.registerFunction(fs, env)
			} else if ss, ok := stmt.(*parser.StructStatement); ok {
				tc.structs[ss.Name] = ss
			}
		}
		// Second pass: check everything
		for _, stmt := range node.Statements {
			tc.Check(stmt, env)
		}
		return UNKNOWN

	// --- Literals ---
	case *parser.NumberLiteral:
		return NUMBER_TYPE
	case *parser.IntLiteral:
		return INT_TYPE
	case *parser.StringLiteral:
		return STRING_TYPE
	case *parser.BoolLiteral:
		return BOOL_TYPE

	// --- Identifier ---
	case *parser.Identifier:
		t, ok := env.Get(node.Value)
		if !ok {
			// Check functions
			if _, fok := tc.funcs[node.Value]; fok {
				return VOID_TYPE
			}
			tc.errorf("'%s' айнымалысы жарияланбаған (undeclared variable)", node.Value)
			return UNKNOWN
		}
		return t

	// --- Unary NOT ---
	case *parser.UnaryExpression:
		rightType := tc.Check(node.Right, env)
		if node.Operator == "емес" {
			if rightType != BOOL_TYPE && rightType != UNKNOWN {
				tc.errorf("'емес' операторы АҚИҚАТ типін талап етеді, бірақ %s берілді", rightType)
			}
			return BOOL_TYPE
		}
		return UNKNOWN

	// --- Binary operations ---
	case *parser.PostfixExpression:
		leftType := tc.Check(node.Left, env)
		rightType := tc.Check(node.Right, env)

		if leftType == UNKNOWN || rightType == UNKNOWN {
			return UNKNOWN
		}

		switch node.Operator {
		case "қосу", "алу", "көбейту", "бөлу":
			// String concatenation with қосу
			if node.Operator == "қосу" && leftType == STRING_TYPE && rightType == STRING_TYPE {
				return STRING_TYPE
			}
			if leftType != NUMBER_TYPE && leftType != INT_TYPE && leftType != BYTE_TYPE {
				tc.errorf("'%s' арифметика операторы сандық тип талап етеді, бірақ %s берілді", node.Operator, leftType)
				return UNKNOWN
			}
			if rightType != NUMBER_TYPE && rightType != INT_TYPE && rightType != BYTE_TYPE {
				tc.errorf("'%s' арифметика операторы сандық тип талап етеді, бірақ %s берілді", node.Operator, rightType)
				return UNKNOWN
			}
			if leftType == NUMBER_TYPE || rightType == NUMBER_TYPE {
				return NUMBER_TYPE
			}
			return leftType

		case "үлкен", "кіші", "тең", "тең_емес", "үлкен_тең", "кіші_тең":
			isLeftNum := (leftType == NUMBER_TYPE || leftType == INT_TYPE)
			isRightNum := (rightType == NUMBER_TYPE || rightType == INT_TYPE)
			if isLeftNum && isRightNum {
				return BOOL_TYPE
			}
			if leftType != rightType {
				tc.errorf("'%s' салыстыру операторы үшін типтер сәйкес болуы керек, бірақ %s және %s", node.Operator, leftType, rightType)
			}
			return BOOL_TYPE

		case "және", "немесе":
			if leftType != BOOL_TYPE {
				tc.errorf("'%s' логикалық оператор АҚИҚАТ типін талап етеді, бірақ %s берілді (сол жақ)", node.Operator, leftType)
			}
			if rightType != BOOL_TYPE {
				tc.errorf("'%s' логикалық оператор АҚИҚАТ типін талап етеді, бірақ %s берілді (оң жақ)", node.Operator, rightType)
			}
			return BOOL_TYPE
		}
		return UNKNOWN

	// --- Struct Operations ---
	case *parser.StructStatement:
		return VOID_TYPE

	case *parser.StructCreateExpression:
		if _, ok := tc.structs[node.StructName]; !ok {
			tc.errorf("'%s' құрылымы табылған жоқ", node.StructName)
		}
		return Type("ҚҰРЫЛЫМ_" + node.StructName)

	case *parser.StructFieldAccessExpression:
		var structName string
		if node.Target != nil {
			targetType := tc.Check(node.Target, env)
			if targetType == UNKNOWN {
				return UNKNOWN
			}
			if !strings.HasPrefix(string(targetType), "ҚҰРЫЛЫМ_") {
				tc.errorf("өріс алу қатесі: объект құрылым емес: %s", targetType)
				return UNKNOWN
			}
			structName = string(targetType)[len("ҚҰРЫЛЫМ_"):]
		} else {
			t, ok := env.Get(node.StructName)
			if !ok {
				tc.errorf("'%s' айнымалысы жарияланбаған", node.StructName)
				return UNKNOWN
			}
			if !strings.HasPrefix(string(t), "ҚҰРЫЛЫМ_") {
				tc.errorf("'%s' айнымалысы құрылым емес", node.StructName)
				return UNKNOWN
			}
			structName = string(t)[len("ҚҰРЫЛЫМ_"):]
		}
		parts := strings.Split(node.Field, ".")

		var currentStructType = structName
		var fieldTypeStr string

		for _, part := range parts {
			structDef, ok := tc.structs[currentStructType]
			if !ok {
				tc.errorf("'%s' құрылымы табылған жоқ", currentStructType)
				return UNKNOWN
			}
			fieldIdx := -1
			for idx, f := range structDef.Fields {
				if f == part {
					fieldIdx = idx
					break
				}
			}
			if fieldIdx == -1 {
				tc.errorf("'%s' құрылымында '%s' өрісі жоқ", currentStructType, part)
				return UNKNOWN
			}
			fieldTypeStr = structDef.Types[fieldIdx]
			currentStructType = fieldTypeStr
		}
		switch fieldTypeStr {
		case "БҮТІН":
			return INT_TYPE
		case "САН":
			return NUMBER_TYPE
		case "МӘТІН":
			return STRING_TYPE
		case "АҚИҚАТ":
			return BOOL_TYPE
		case "БАЙТ":
			return BYTE_TYPE
		default:
			return Type("ҚҰРЫЛЫМ_" + fieldTypeStr)
		}

	case *parser.StructFieldAssignStatement:
		valType := tc.Check(node.Value, env)
		var structName string
		if node.Target != nil {
			targetType := tc.Check(node.Target, env)
			if targetType == UNKNOWN {
				return VOID_TYPE
			}
			if !strings.HasPrefix(string(targetType), "ҚҰРЫЛЫМ_") {
				tc.errorf("өріске меншіктеу қатесі: объект құрылым емес: %s", targetType)
				return VOID_TYPE
			}
			structName = string(targetType)[len("ҚҰРЫЛЫМ_"):]
		} else {
			t, ok := env.Get(node.StructName)
			if !ok {
				tc.errorf("'%s' айнымалысы жарияланбаған", node.StructName)
				return VOID_TYPE
			}
			if !strings.HasPrefix(string(t), "ҚҰРЫЛЫМ_") {
				tc.errorf("'%s' айнымалысы құрылым емес", node.StructName)
				return VOID_TYPE
			}
			structName = string(t)[len("ҚҰРЫЛЫМ_"):]
		}
		parts := strings.Split(node.Field, ".")

		var currentStructType = structName
		var fieldTypeStr string

		for _, part := range parts {
			structDef, ok := tc.structs[currentStructType]
			if !ok {
				tc.errorf("'%s' құрылымы табылған жоқ", currentStructType)
				return VOID_TYPE
			}
			fieldIdx := -1
			for idx, f := range structDef.Fields {
				if f == part {
					fieldIdx = idx
					break
				}
			}
			if fieldIdx == -1 {
				tc.errorf("'%s' құрылымында '%s' өрісі жоқ", currentStructType, part)
				return VOID_TYPE
			}
			fieldTypeStr = structDef.Types[fieldIdx]
			currentStructType = fieldTypeStr
		}

		var expectedType Type
		switch fieldTypeStr {
		case "БҮТІН":
			expectedType = INT_TYPE
		case "САН":
			expectedType = NUMBER_TYPE
		case "МӘТІН":
			expectedType = STRING_TYPE
		case "АҚИҚАТ":
			expectedType = BOOL_TYPE
		case "БАЙТ":
			expectedType = BYTE_TYPE
		default:
			expectedType = Type("ҚҰРЫЛЫМ_" + fieldTypeStr)
		}
		if valType != expectedType && valType != UNKNOWN {
			if expectedType == NUMBER_TYPE && valType == INT_TYPE {
				// Promotion allowed
			} else if strings.HasPrefix(string(expectedType), "ҚҰРЫЛЫМ_") && valType == INT_TYPE {
				// Null pointer assignment allowed
			} else {
				tc.errorf("меншіктеу қатесі: '%s.%s' өрісі %s типін күтеді, бірақ %s берілді", structName, node.Field, expectedType, valType)
			}
		}
		return VOID_TYPE

	// --- Array literal ---
	case *parser.ArrayLiteral:
		// All elements must be same type
		if len(node.Elements) == 0 {
			return Type("ТІЗІМ_БЕЛГІСІЗ")
		}
		firstType := tc.Check(node.Elements[0], env)
		for i, el := range node.Elements[1:] {
			t := tc.Check(el, env)
			if t != firstType {
				tc.errorf("тізім элементтерінің типтері сәйкес емес: %d индексте %s күтілді, бірақ %s табылды", i+1, firstType, t)
			}
		}
		return Type("ТІЗІМ_" + string(firstType))

	// --- Array index ---
	case *parser.IndexExpression:
		arrType := tc.Check(node.Left, env)
		idxType := tc.Check(node.Index, env)
		if !strings.HasPrefix(string(arrType), "ТІЗІМ") && arrType != STRING_TYPE {
			tc.errorf("индекстеу тізім немесе мәтін типін талап етеді, бірақ %s берілді", arrType)
		}
		if idxType != NUMBER_TYPE && idxType != INT_TYPE {
			tc.errorf("индекс сандық тип болуы керек, бірақ %s берілді", idxType)
		}
		if arrType == STRING_TYPE {
			return STRING_TYPE
		}
		if strings.HasPrefix(string(arrType), "ТІЗІМ_") {
			return Type(string(arrType)[len("ТІЗІМ_"):])
		}
		return UNKNOWN

	// --- Length ---
	case *parser.LengthExpression:
		valType := tc.Check(node.Value, env)
		if !strings.HasPrefix(string(valType), "ТІЗІМ") && valType != STRING_TYPE {
			tc.errorf("'ұзындық' тізім немесе мәтін типін талап етеді, бірақ %s берілді", valType)
		}
		return NUMBER_TYPE

	// --- Char at ---
	case *parser.CharAtExpression:
		strType := tc.Check(node.Str, env)
		idxType := tc.Check(node.Index, env)
		if strType != STRING_TYPE && strType != UNKNOWN {
			tc.errorf("'символ' мәтін типін талап етеді, бірақ %s берілді", strType)
		}
		if idxType != NUMBER_TYPE && idxType != INT_TYPE && idxType != UNKNOWN {
			tc.errorf("'символ' индексі сандық болуы керек, бірақ %s берілді", idxType)
		}
		return STRING_TYPE // In Butaq, a char is just a 1-character string! (we used char_buf as string)

	case *parser.CharCodeExpression:
		strType := tc.Check(node.Value, env)
		if strType != STRING_TYPE && strType != UNKNOWN {
			tc.errorf("'таңба_коды' мәтін типін талап етеді, бірақ %s берілді", strType)
		}
		return NUMBER_TYPE

	// --- Variable assignment ---
	case *parser.VarAssignStatement:
		valType := tc.Check(node.Value, env)
		existingType, ok := env.Get(node.Name.Value)
		if !ok {
			env.Set(node.Name.Value, valType)
		} else {
			if existingType != valType && valType != UNKNOWN {
				if strings.HasPrefix(string(existingType), "ҚҰРЫЛЫМ_") && valType == INT_TYPE {
					// Null pointer assignment allowed
				} else {
					tc.errorf("'%s' айнымалысының типін өзгертуге болмайды (%s -> %s)", node.Name.Value, existingType, valType)
				}
			}
		}
		return valType

	// --- Print ---
	case *parser.PrintStatement:
		tc.Check(node.Value, env)
		return VOID_TYPE

	// --- Return ---
	case *parser.ReturnStatement:
		ret := tc.Check(node.Value, env)
		// Propagate return type into the current function's signature
		if tc.currentFunc != "" {
			if sig, ok := tc.funcs[tc.currentFunc]; ok {
				if sig.ReturnType == UNKNOWN && ret != UNKNOWN {
					sig.ReturnType = ret
				}
			}
		}
		return ret

	// --- Call expression ---
	case *parser.CallExpression:
		if strings.Contains(node.Function, ".") {
			parts := strings.Split(node.Function, ".")
			if len(parts) == 2 {
				varName := parts[0]
				methodName := parts[1]
				t, ok := env.Get(varName)
				if ok && strings.HasPrefix(string(t), "ҚҰРЫЛЫМ_") {
					structName := string(t)[len("ҚҰРЫЛЫМ_"):]
					node.Arguments = append([]parser.Expression{&parser.Identifier{Value: varName}}, node.Arguments...)
					node.Function = structName + "." + methodName
				}
			}
		}
		sig, ok := tc.funcs[node.Function]
		if !ok {
			// Check if it is a struct name (constructor)
			if strDef, okStruct := tc.structs[node.Function]; okStruct {
				if len(node.Arguments) != len(strDef.Fields) {
					tc.errorf("'%s' құрылымын инициализациялау үшін %d аргумент қажет, бірақ %d берілді", node.Function, len(strDef.Fields), len(node.Arguments))
				}
				for i, arg := range node.Arguments {
					argType := tc.Check(arg, env)
					if i < len(strDef.Types) {
						expectedType := tc.fieldTypeToType(strDef.Types[i])
						if argType != expectedType && argType != UNKNOWN {
							if expectedType == NUMBER_TYPE && argType == INT_TYPE {
								// Promotion allowed
							} else if strings.HasPrefix(string(expectedType), "ҚҰРЫЛЫМ_") && argType == INT_TYPE {
								// Null pointer initializer allowed
							} else {
								tc.errorf("'%s' құрылымының '%s' өрісі %s типін күтеді, бірақ %s берілді", node.Function, strDef.Fields[i], expectedType, argType)
							}
						}
					}
				}
				return Type("ҚҰРЫЛЫМ_" + node.Function)
			}
			tc.errorf("'%s' функциясы табылмады", node.Function)
			return UNKNOWN
		}
		if len(node.Arguments) != len(sig.Params) {
			tc.errorf("'%s' функциясы %d аргумент талап етеді, бірақ %d берілді", node.Function, len(sig.Params), len(node.Arguments))
		}
		for i, arg := range node.Arguments {
			argType := tc.Check(arg, env)
			if i < len(sig.ParamTypes) {
				if sig.ParamTypes[i] == UNKNOWN || sig.ParamTypes[i] == "" {
					sig.ParamTypes[i] = argType
				}
			}
		}
		return sig.ReturnType

	// --- Call statement ---
	case *parser.CallStatement:
		tc.Check(node.Call, env)
		return VOID_TYPE

	// --- Block ---
	case *parser.BlockStatement:
		blockEnv := NewEnclosedTypeEnv(env)
		for _, stmt := range node.Statements {
			tc.Check(stmt, blockEnv)
		}
		// Propagate updated variable types back to outer env
		// (needed for while-loops that mutate outer variables)
		for name, t := range blockEnv.store {
			if _, exists := env.Get(name); exists {
				env.Set(name, t)
			}
		}
		return UNKNOWN

	// --- If ---
	case *parser.IfStatement:
		condType := tc.Check(node.Condition, env)
		if condType != BOOL_TYPE && condType != UNKNOWN {
			tc.errorf("'егер' шарты АҚИҚАТ болуы керек, бірақ %s берілді", condType)
		}
		tc.Check(node.Consequence, env)
		if node.Alternative != nil {
			tc.Check(node.Alternative, env)
		}
		return UNKNOWN

	// --- While ---
	case *parser.WhileStatement:
		condType := tc.Check(node.Condition, env)
		if condType != BOOL_TYPE && condType != UNKNOWN {
			tc.errorf("'әзірше' шарты АҚИҚАТ болуы керек, бірақ %s берілді", condType)
		}
		tc.Check(node.Body, env)
		return UNKNOWN

	// --- Function definition ---
	case *parser.FunctionStatement:
		sig := tc.funcs[node.Name]
		funcEnv := NewEnclosedTypeEnv(tc.globalEnv)
		for i, param := range node.Parameters {
			pt := UNKNOWN
			if sig != nil && i < len(sig.ParamTypes) {
				pt = sig.ParamTypes[i]
			}
			funcEnv.Set(param, pt)
		}
		// Track current function for return type inference
		prevFunc := tc.currentFunc
		tc.currentFunc = node.Name
		tc.Check(node.Body, funcEnv)
		tc.currentFunc = prevFunc
		// Infer param types from how the function body uses them
		if sig != nil {
			for i, param := range node.Parameters {
				if sig.ParamTypes[i] == UNKNOWN {
					if t, ok := funcEnv.Get(param); ok && t != UNKNOWN {
						sig.ParamTypes[i] = t
					}
				}
			}
		}
		return VOID_TYPE

	// --- String operations (not covered above) ---
	case *parser.StrConcatExpression:
		tc.Check(node.Left, env)
		tc.Check(node.Right, env)
		return STRING_TYPE

	case *parser.StrEqExpression:
		tc.Check(node.Left, env)
		tc.Check(node.Right, env)
		return BOOL_TYPE

	case *parser.StrLenExpression:
		tc.Check(node.Value, env)
		return NUMBER_TYPE

	case *parser.ToStrExpression:
		tc.Check(node.Value, env)
		return STRING_TYPE

	case *parser.FileReadExpression:
		tc.Check(node.Path, env)
		return STRING_TYPE

	case *parser.InputExpression:
		return STRING_TYPE

	case *parser.FileWriteStatement:
		return VOID_TYPE

	case *parser.BreakStatement:
		return VOID_TYPE

	case *parser.ContinueStatement:
		return VOID_TYPE

	// --- Expression statement ---
	case *parser.ExpressionStatement:
		return tc.Check(node.Expression, env)
	}

	return UNKNOWN
}

func (tc *TypeChecker) fieldTypeToType(ft string) Type {
	switch ft {
	case "БҮТІН":
		return INT_TYPE
	case "САН":
		return NUMBER_TYPE
	case "МӘТІН":
		return STRING_TYPE
	case "АҚИҚАТ":
		return BOOL_TYPE
	case "БАЙТ":
		return BYTE_TYPE
	default:
		return Type("ҚҰРЫЛЫМ_" + ft)
	}
}

// ---------------------------------------------------------------------------
// Function registration (first pass)
// ---------------------------------------------------------------------------

func (tc *TypeChecker) registerFunction(fs *parser.FunctionStatement, env *TypeEnv) {
	paramTypes := make([]Type, len(fs.Parameters))
	for i := range fs.Parameters {
		paramTypes[i] = UNKNOWN // inferred later
	}
	if strings.Contains(fs.Name, ".") {
		parts := strings.Split(fs.Name, ".")
		if len(parts) == 2 && len(fs.Parameters) > 0 && fs.Parameters[0] == "өзі" {
			paramTypes[0] = Type("ҚҰРЫЛЫМ_" + parts[0])
		}
	}
	sig := &FuncSig{
		Params:     fs.Parameters,
		ParamTypes: paramTypes,
		ReturnType: UNKNOWN, // inferred from body
	}
	tc.funcs[fs.Name] = sig
	env.Set(fs.Name, VOID_TYPE)
}
