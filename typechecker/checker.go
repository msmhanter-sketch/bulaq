package typechecker

import (
	"butaq/parser"
	"fmt"
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
	globalEnv *TypeEnv
	funcs     map[string]*FuncSig
	Errors    []string
}

func New() *TypeChecker {
	return &TypeChecker{
		globalEnv: NewTypeEnv(),
		funcs:     make(map[string]*FuncSig),
		Errors:    []string{},
	}
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
		// First pass: register all function signatures
		for _, stmt := range node.Statements {
			if fs, ok := stmt.(*parser.FunctionStatement); ok {
				tc.registerFunction(fs, env)
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
			if leftType != rightType {
				tc.errorf("'%s' типтер сәйкес емес: %s және %s", node.Operator, leftType, rightType)
				return UNKNOWN
			}
			return leftType

		case "үлкен", "кіші", "тең", "тең_емес", "үлкен_тең", "кіші_тең":
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

	// --- Array literal ---
	case *parser.ArrayLiteral:
		// All elements must be same type
		if len(node.Elements) == 0 {
			return ARRAY_TYPE
		}
		firstType := tc.Check(node.Elements[0], env)
		for i, el := range node.Elements[1:] {
			t := tc.Check(el, env)
			if t != firstType {
				tc.errorf("тізім элементтерінің типтері сәйкес емес: %d индексте %s күтілді, бірақ %s табылды", i+1, firstType, t)
			}
		}
		return ARRAY_TYPE

	// --- Array index ---
	case *parser.IndexExpression:
		arrType := tc.Check(node.Left, env)
		idxType := tc.Check(node.Index, env)
		if arrType != ARRAY_TYPE && arrType != STRING_TYPE {
			tc.errorf("индекстеу тізім немесе мәтін типін талап етеді, бірақ %s берілді", arrType)
		}
		if idxType != NUMBER_TYPE && idxType != INT_TYPE {
			tc.errorf("индекс сандық тип болуы керек, бірақ %s берілді", idxType)
		}
		return UNKNOWN // element type unknown without generics

	// --- Length ---
	case *parser.LengthExpression:
		valType := tc.Check(node.Value, env)
		if valType != ARRAY_TYPE && valType != STRING_TYPE {
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
				tc.errorf("'%s' айнымалысының типін өзгертуге болмайды (%s -> %s)", node.Name.Value, existingType, valType)
			}
		}
		return valType

	// --- Print ---
	case *parser.PrintStatement:
		tc.Check(node.Value, env)
		return VOID_TYPE

	// --- Return ---
	case *parser.ReturnStatement:
		return tc.Check(node.Value, env)

	// --- Call expression ---
	case *parser.CallExpression:
		sig, ok := tc.funcs[node.Function]
		if !ok {
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
		// Propagate variable declarations to outer env for variables assigned in blocks
		// (needed for while loops updating outer variables)
		for name, t := range blockEnv.store {
			if _, exists := env.Get(name); !exists {
				// new variable declared inside block - keep it local
				_ = t
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
		tc.Check(node.Body, funcEnv)
		return VOID_TYPE

	// --- Expression statement ---
	case *parser.ExpressionStatement:
		return tc.Check(node.Expression, env)
	}

	return UNKNOWN
}

// ---------------------------------------------------------------------------
// Function registration (first pass)
// ---------------------------------------------------------------------------

func (tc *TypeChecker) registerFunction(fs *parser.FunctionStatement, env *TypeEnv) {
	paramTypes := make([]Type, len(fs.Parameters))
	for i := range fs.Parameters {
		paramTypes[i] = UNKNOWN // inferred later
	}
	sig := &FuncSig{
		Params:     fs.Parameters,
		ParamTypes: paramTypes,
		ReturnType: UNKNOWN, // inferred from body
	}
	tc.funcs[fs.Name] = sig
	env.Set(fs.Name, VOID_TYPE)
}
