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
	JSON_TYPE   Type = "ЖСОН"
	RESULT_TYPE Type = "НӘТИЖЕ"
)

func IsResultType(t Type) bool {
	return t == "НӘТИЖЕ" || strings.HasPrefix(string(t), "НӘТИЖЕ_")
}

func GetResultUnderlyingType(t Type) Type {
	if t == "НӘТИЖЕ" {
		return UNKNOWN
	}
	if strings.HasPrefix(string(t), "НӘТИЖЕ_") {
		return Type(strings.TrimPrefix(string(t), "НӘТИЖЕ_"))
	}
	return UNKNOWN
}

func MakeResultType(underlying Type) Type {
	return Type("НӘТИЖЕ_" + string(underlying))
}

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
// TypeError & TypeChecker
// ---------------------------------------------------------------------------

type TypeError struct {
	Message string
	Line    int
	Col     int
}

func (e TypeError) Error() string {
	return fmt.Sprintf("Тип қатесі (%d:%d): %s", e.Line, e.Col, e.Message)
}

type TypeChecker struct {
	globalEnv   *TypeEnv
	funcs       map[string]*FuncSig
	structs     map[string]*parser.StructStatement
	interfaces  map[string]*parser.InterfaceStatement
	Errors      []TypeError
	currentFunc string // tracks which function we are checking
	curLine     int
	curCol      int
}

func New() *TypeChecker {
	tc := &TypeChecker{
		globalEnv:  NewTypeEnv(),
		funcs:      make(map[string]*FuncSig),
		structs:    make(map[string]*parser.StructStatement),
		interfaces: make(map[string]*parser.InterfaceStatement),
		Errors:     []TypeError{},
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
	tc.funcs["жүйе"] = &FuncSig{
		Params:     []string{"команда"},
		ParamTypes: []Type{STRING_TYPE},
		ReturnType: NUMBER_TYPE,
	}
	tc.funcs["файл_жою"] = &FuncSig{
		Params:     []string{"жол"},
		ParamTypes: []Type{STRING_TYPE},
		ReturnType: NUMBER_TYPE,
	}
	tc.funcs["аргумент_саны"] = &FuncSig{
		Params:     []string{},
		ParamTypes: []Type{},
		ReturnType: NUMBER_TYPE,
	}
	tc.funcs["аргумент"] = &FuncSig{
		Params:     []string{"индекс"},
		ParamTypes: []Type{NUMBER_TYPE},
		ReturnType: STRING_TYPE,
	}
	tc.funcs["файл_бар_ма"] = &FuncSig{
		Params:     []string{"жол"},
		ParamTypes: []Type{STRING_TYPE},
		ReturnType: NUMBER_TYPE,
	}
	tc.funcs["түбір"] = &FuncSig{
		Params:     []string{"сан"},
		ParamTypes: []Type{NUMBER_TYPE},
		ReturnType: NUMBER_TYPE,
	}
	tc.funcs["дәреже"] = &FuncSig{
		Params:     []string{"негіз", "дәреже"},
		ParamTypes: []Type{NUMBER_TYPE, NUMBER_TYPE},
		ReturnType: NUMBER_TYPE,
	}
	tc.funcs["синус"] = &FuncSig{
		Params:     []string{"сан"},
		ParamTypes: []Type{NUMBER_TYPE},
		ReturnType: NUMBER_TYPE,
	}
	tc.funcs["косинус"] = &FuncSig{
		Params:     []string{"сан"},
		ParamTypes: []Type{NUMBER_TYPE},
		ReturnType: NUMBER_TYPE,
	}
	tc.funcs["мәтін"] = &FuncSig{
		Params:     []string{"сан"},
		ParamTypes: []Type{NUMBER_TYPE},
		ReturnType: STRING_TYPE,
	}
	tc.funcs["сан"] = &FuncSig{
		Params:     []string{"мәтін"},
		ParamTypes: []Type{STRING_TYPE},
		ReturnType: NUMBER_TYPE,
	}
	tc.funcs["уақыт"] = &FuncSig{
		Params:     []string{},
		ParamTypes: []Type{},
		ReturnType: NUMBER_TYPE,
	}
	tc.funcs["уақыт_мәтіні"] = &FuncSig{
		Params:     []string{"пішім"},
		ParamTypes: []Type{STRING_TYPE},
		ReturnType: STRING_TYPE,
	}
	tc.funcs["ұйықтау"] = &FuncSig{
		Params:     []string{"секунд"},
		ParamTypes: []Type{NUMBER_TYPE},
		ReturnType: VOID_TYPE,
	}
	tc.funcs["жүйе_шығысы"] = &FuncSig{
		Params:     []string{"команда"},
		ParamTypes: []Type{STRING_TYPE},
		ReturnType: STRING_TYPE,
	}

	// JSON functions
	tc.funcs["жсон_оқу"] = &FuncSig{
		Params:     []string{"мәтін"},
		ParamTypes: []Type{STRING_TYPE},
		ReturnType: JSON_TYPE,
	}
	tc.funcs["жсон_жазу"] = &FuncSig{
		Params:     []string{"объект"},
		ParamTypes: []Type{JSON_TYPE},
		ReturnType: STRING_TYPE,
	}
	tc.funcs["жсон_сан_алу"] = &FuncSig{
		Params:     []string{"объект", "кілт"},
		ParamTypes: []Type{JSON_TYPE, STRING_TYPE},
		ReturnType: NUMBER_TYPE,
	}
	tc.funcs["жсон_мәтін_алу"] = &FuncSig{
		Params:     []string{"объект", "кілт"},
		ParamTypes: []Type{JSON_TYPE, STRING_TYPE},
		ReturnType: STRING_TYPE,
	}
	tc.funcs["жсон_логика_алу"] = &FuncSig{
		Params:     []string{"объект", "кілт"},
		ParamTypes: []Type{JSON_TYPE, STRING_TYPE},
		ReturnType: BOOL_TYPE,
	}
	tc.funcs["жсон_нысан_алу"] = &FuncSig{
		Params:     []string{"объект", "кілт"},
		ParamTypes: []Type{JSON_TYPE, STRING_TYPE},
		ReturnType: JSON_TYPE,
	}
	tc.funcs["жсон_тізім_алу"] = &FuncSig{
		Params:     []string{"объект", "кілт"},
		ParamTypes: []Type{JSON_TYPE, STRING_TYPE},
		ReturnType: JSON_TYPE,
	}
	tc.funcs["жсон_тізім_өлшемі"] = &FuncSig{
		Params:     []string{"тізім"},
		ParamTypes: []Type{JSON_TYPE},
		ReturnType: NUMBER_TYPE,
	}
	tc.funcs["жсон_тізім_элементі"] = &FuncSig{
		Params:     []string{"тізім", "индекс"},
		ParamTypes: []Type{JSON_TYPE, NUMBER_TYPE},
		ReturnType: JSON_TYPE,
	}
	tc.funcs["жсон_жаңа"] = &FuncSig{
		Params:     []string{},
		ParamTypes: []Type{},
		ReturnType: JSON_TYPE,
	}
	tc.funcs["жсон_сан_қосу"] = &FuncSig{
		Params:     []string{"объект", "кілт", "мән"},
		ParamTypes: []Type{JSON_TYPE, STRING_TYPE, NUMBER_TYPE},
		ReturnType: VOID_TYPE,
	}
	tc.funcs["жсон_мәтін_қосу"] = &FuncSig{
		Params:     []string{"объект", "кілт", "мән"},
		ParamTypes: []Type{JSON_TYPE, STRING_TYPE, STRING_TYPE},
		ReturnType: VOID_TYPE,
	}
	tc.funcs["жсон_логика_қосу"] = &FuncSig{
		Params:     []string{"объект", "кілт", "мән"},
		ParamTypes: []Type{JSON_TYPE, STRING_TYPE, BOOL_TYPE},
		ReturnType: VOID_TYPE,
	}
	tc.funcs["жсон_нысан_қосу"] = &FuncSig{
		Params:     []string{"объект", "кілт", "мән"},
		ParamTypes: []Type{JSON_TYPE, STRING_TYPE, JSON_TYPE},
		ReturnType: VOID_TYPE,
	}
	tc.funcs["жсон_тізім_қосу"] = &FuncSig{
		Params:     []string{"объект", "кілт", "мән"},
		ParamTypes: []Type{JSON_TYPE, STRING_TYPE, JSON_TYPE},
		ReturnType: VOID_TYPE,
	}

	// Crypto functions
	tc.funcs["мд5"] = &FuncSig{
		Params:     []string{"мәтін"},
		ParamTypes: []Type{STRING_TYPE},
		ReturnType: STRING_TYPE,
	}
	tc.funcs["ша256"] = &FuncSig{
		Params:     []string{"мәтін"},
		ParamTypes: []Type{STRING_TYPE},
		ReturnType: STRING_TYPE,
	}
	tc.funcs["б64_кодтау"] = &FuncSig{
		Params:     []string{"мәтін"},
		ParamTypes: []Type{STRING_TYPE},
		ReturnType: STRING_TYPE,
	}
	tc.funcs["б64_декодтау"] = &FuncSig{
		Params:     []string{"мәтін"},
		ParamTypes: []Type{STRING_TYPE},
		ReturnType: STRING_TYPE,
	}

	// Thread function
	tc.funcs["ағын_күту"] = &FuncSig{
		Params:     []string{"ағын"},
		ParamTypes: []Type{NUMBER_TYPE},
		ReturnType: NUMBER_TYPE,
	}

	// ── New String functions ──────────────────────────────────────────────
	tc.funcs["мәтін_бөлу"] = &FuncSig{
		Params:     []string{"мәтін", "бөлгіш", "индекс"},
		ParamTypes: []Type{STRING_TYPE, STRING_TYPE, NUMBER_TYPE},
		ReturnType: STRING_TYPE,
	}
	tc.funcs["мәтін_бөлу_саны"] = &FuncSig{
		Params:     []string{"мәтін", "бөлгіш"},
		ParamTypes: []Type{STRING_TYPE, STRING_TYPE},
		ReturnType: NUMBER_TYPE,
	}
	tc.funcs["мәтін_ауыстыру"] = &FuncSig{
		Params:     []string{"мәтін", "ескі", "жаңа"},
		ParamTypes: []Type{STRING_TYPE, STRING_TYPE, STRING_TYPE},
		ReturnType: STRING_TYPE,
	}
	tc.funcs["мәтін_кіші"] = &FuncSig{
		Params:     []string{"мәтін"},
		ParamTypes: []Type{STRING_TYPE},
		ReturnType: STRING_TYPE,
	}
	tc.funcs["мәтін_жоғары"] = &FuncSig{
		Params:     []string{"мәтін"},
		ParamTypes: []Type{STRING_TYPE},
		ReturnType: STRING_TYPE,
	}
	tc.funcs["мәтін_қысқарту"] = &FuncSig{
		Params:     []string{"мәтін"},
		ParamTypes: []Type{STRING_TYPE},
		ReturnType: STRING_TYPE,
	}
	tc.funcs["мәтін_кесу"] = &FuncSig{
		Params:     []string{"мәтін", "бастап", "дейін"},
		ParamTypes: []Type{STRING_TYPE, NUMBER_TYPE, NUMBER_TYPE},
		ReturnType: STRING_TYPE,
	}
	tc.funcs["мәтін_басталады"] = &FuncSig{
		Params:     []string{"мәтін", "алдыңғы"},
		ParamTypes: []Type{STRING_TYPE, STRING_TYPE},
		ReturnType: NUMBER_TYPE,
	}
	tc.funcs["мәтін_аяқталады"] = &FuncSig{
		Params:     []string{"мәтін", "соңғы"},
		ParamTypes: []Type{STRING_TYPE, STRING_TYPE},
		ReturnType: NUMBER_TYPE,
	}
	tc.funcs["мәтін_іздеу"] = &FuncSig{
		Params:     []string{"мәтін", "іздеу"},
		ParamTypes: []Type{STRING_TYPE, STRING_TYPE},
		ReturnType: NUMBER_TYPE,
	}

	// ── Extended Math ──────────────────────────────────────────────────────
	for _, fn := range []string{"абс", "еден", "төбе", "логарифм", "логарифм2", "логарифм10",
		"дөңгелек", "тангенс", "арктангенс", "экспонента"} {
		tc.funcs[fn] = &FuncSig{
			Params: []string{"сан"}, ParamTypes: []Type{NUMBER_TYPE}, ReturnType: NUMBER_TYPE,
		}
	}
	tc.funcs["ең_кіші"] = &FuncSig{
		Params: []string{"а", "б"}, ParamTypes: []Type{NUMBER_TYPE, NUMBER_TYPE}, ReturnType: NUMBER_TYPE,
	}
	tc.funcs["ең_үлкен"] = &FuncSig{
		Params: []string{"а", "б"}, ParamTypes: []Type{NUMBER_TYPE, NUMBER_TYPE}, ReturnType: NUMBER_TYPE,
	}
	tc.funcs["арктангенс2"] = &FuncSig{
		Params: []string{"y", "x"}, ParamTypes: []Type{NUMBER_TYPE, NUMBER_TYPE}, ReturnType: NUMBER_TYPE,
	}
	tc.funcs["пи"] = &FuncSig{
		Params: []string{}, ParamTypes: []Type{}, ReturnType: NUMBER_TYPE,
	}

	// ── Map / Dictionary ───────────────────────────────────────────────────
	tc.funcs["сөздік_жаңа"] = &FuncSig{
		Params: []string{}, ParamTypes: []Type{}, ReturnType: JSON_TYPE, // reuse JSON_TYPE for void* map
	}
	tc.funcs["сөздік_сан_қою"] = &FuncSig{
		Params: []string{"сөздік", "кілт", "мән"}, ParamTypes: []Type{JSON_TYPE, STRING_TYPE, NUMBER_TYPE}, ReturnType: VOID_TYPE,
	}
	tc.funcs["сөздік_мәтін_қою"] = &FuncSig{
		Params: []string{"сөздік", "кілт", "мән"}, ParamTypes: []Type{JSON_TYPE, STRING_TYPE, STRING_TYPE}, ReturnType: VOID_TYPE,
	}
	tc.funcs["сөздік_сан_алу"] = &FuncSig{
		Params: []string{"сөздік", "кілт"}, ParamTypes: []Type{JSON_TYPE, STRING_TYPE}, ReturnType: NUMBER_TYPE,
	}
	tc.funcs["сөздік_мәтін_алу"] = &FuncSig{
		Params: []string{"сөздік", "кілт"}, ParamTypes: []Type{JSON_TYPE, STRING_TYPE}, ReturnType: STRING_TYPE,
	}
	tc.funcs["сөздік_бар_ма"] = &FuncSig{
		Params: []string{"сөздік", "кілт"}, ParamTypes: []Type{JSON_TYPE, STRING_TYPE}, ReturnType: NUMBER_TYPE,
	}
	tc.funcs["сөздік_жою"] = &FuncSig{
		Params: []string{"сөздік", "кілт"}, ParamTypes: []Type{JSON_TYPE, STRING_TYPE}, ReturnType: VOID_TYPE,
	}
	tc.funcs["сөздік_өлшемі"] = &FuncSig{
		Params: []string{"сөздік"}, ParamTypes: []Type{JSON_TYPE}, ReturnType: NUMBER_TYPE,
	}

	// ── Mutex / Synchronization ─────────────────────────────────────────────
	tc.funcs["мьютекс_жаңа"] = &FuncSig{
		Params: []string{}, ParamTypes: []Type{}, ReturnType: JSON_TYPE,
	}
	tc.funcs["мьютекс_бекіту"] = &FuncSig{
		Params: []string{"мьютекс"}, ParamTypes: []Type{JSON_TYPE}, ReturnType: VOID_TYPE,
	}
	tc.funcs["мьютекс_босату"] = &FuncSig{
		Params: []string{"мьютекс"}, ParamTypes: []Type{JSON_TYPE}, ReturnType: VOID_TYPE,
	}
	tc.funcs["мьютекс_жою"] = &FuncSig{
		Params: []string{"мьютекс"}, ParamTypes: []Type{JSON_TYPE}, ReturnType: VOID_TYPE,
	}

	// ── StringBuilder ───────────────────────────────────────────────────────
	tc.funcs["мәтін_жинақтаушы_жаңа"] = &FuncSig{
		Params: []string{}, ParamTypes: []Type{}, ReturnType: JSON_TYPE,
	}
	tc.funcs["мәтін_жинақтаушы_қосу_мәтін"] = &FuncSig{
		Params: []string{"жинақтаушы", "мәтін"}, ParamTypes: []Type{JSON_TYPE, STRING_TYPE}, ReturnType: VOID_TYPE,
	}
	tc.funcs["мәтін_жинақтаушы_қосу_сан"] = &FuncSig{
		Params: []string{"жинақтаушы", "сан"}, ParamTypes: []Type{JSON_TYPE, NUMBER_TYPE}, ReturnType: VOID_TYPE,
	}
	tc.funcs["мәтін_жинақтаушы_қосу_таңба"] = &FuncSig{
		Params: []string{"жинақтаушы", "код"}, ParamTypes: []Type{JSON_TYPE, NUMBER_TYPE}, ReturnType: VOID_TYPE,
	}
	tc.funcs["мәтін_жинақтаушы_жазу"] = &FuncSig{
		Params: []string{"жинақтаушы"}, ParamTypes: []Type{JSON_TYPE}, ReturnType: STRING_TYPE,
	}
	tc.funcs["мәтін_жинақтаушы_жою"] = &FuncSig{
		Params: []string{"жинақтаушы"}, ParamTypes: []Type{JSON_TYPE}, ReturnType: VOID_TYPE,
	}

	return tc
}

// GetStructs returns the struct definitions (used by codegen)
func (tc *TypeChecker) GetStructs() map[string]*parser.StructStatement {
	return tc.structs
}

// GetInterfaces returns the interface definitions (used by codegen)
func (tc *TypeChecker) GetInterfaces() map[string]*parser.InterfaceStatement {
	return tc.interfaces
}

func (tc *TypeChecker) ErrorStrings() []string {
	var errs []string
	for _, err := range tc.Errors {
		errs = append(errs, err.Error())
	}
	return errs
}

func (tc *TypeChecker) errorf(format string, args ...interface{}) {
	tc.Errors = append(tc.Errors, TypeError{
		Message: fmt.Sprintf(format, args...),
		Line:    tc.curLine,
		Col:     tc.curCol,
	})
}

// GetFuncs returns the function signature map (used by codegen)
func (tc *TypeChecker) GetFuncs() map[string]*FuncSig {
	return tc.funcs
}

func (tc *TypeChecker) GetGlobalEnv() *TypeEnv {
	return tc.globalEnv
}

// ---------------------------------------------------------------------------
// Check entry point
// ---------------------------------------------------------------------------

func (tc *TypeChecker) Check(node parser.Node, env *TypeEnv) Type {
	if node == nil {
		return UNKNOWN
	}
	oldLine, oldCol := tc.curLine, tc.curCol
	l, c := node.Position()
	if l > 0 {
		tc.curLine, tc.curCol = l, c
	}
	defer func() {
		tc.curLine, tc.curCol = oldLine, oldCol
	}()

	switch node := node.(type) {

	case *parser.Program:
		// First pass: register all function signatures, structs, and interfaces
		for _, stmt := range node.Statements {
			if fs, ok := stmt.(*parser.FunctionStatement); ok {
				tc.registerFunction(fs, env)
			} else if ss, ok := stmt.(*parser.StructStatement); ok {
				tc.structs[ss.Name] = ss
			} else if is, ok := stmt.(*parser.InterfaceStatement); ok {
				tc.interfaces[is.Name] = is
			}
		}
		// Second pass: check everything (populates signature and variable types)
		for _, stmt := range node.Statements {
			tc.Check(stmt, env)
		}
		// Clear intermediate errors from the inference pass
		tc.Errors = []TypeError{}
		// Clear global env store to allow clean type re-inference without type mismatch errors
		env.store = make(map[string]Type)
		for _, stmt := range node.Statements {
			if fs, ok := stmt.(*parser.FunctionStatement); ok {
				tc.registerFunction(fs, env)
			}
		}
		// Third pass: final check with resolved parameter and return types
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
			if rightType == INT_TYPE {
				return INT_TYPE
			}
			if rightType != BOOL_TYPE && rightType != UNKNOWN {
				tc.errorf("'емес' операторы АҚИҚАТ немесе БҮТІН типін талап етеді, бірақ %s берілді", rightType)
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
			// Allow comparing struct pointers to 0 (null)
			isLeftStruct := strings.HasPrefix(string(leftType), "ҚҰРЫЛЫМ_")
			isRightStruct := strings.HasPrefix(string(rightType), "ҚҰРЫЛЫМ_")
			if (isLeftStruct && rightType == INT_TYPE) || (isRightStruct && leftType == INT_TYPE) {
				if node.Operator == "тең" || node.Operator == "тең_емес" {
					return BOOL_TYPE
				}
			}
			if leftType != rightType {
				tc.errorf("'%s' салыстыру операторы үшін типтер сәйкес болуы керек, бірақ %s және %s", node.Operator, leftType, rightType)
			}
			return BOOL_TYPE

		case "және", "немесе":
			if leftType == INT_TYPE && rightType == INT_TYPE {
				return INT_TYPE
			}
			if leftType != BOOL_TYPE {
				tc.errorf("'%s' логикалық оператор АҚИҚАТ типін талап етеді, бірақ %s берілді (сол жақ)", node.Operator, leftType)
			}
			if rightType != BOOL_TYPE {
				tc.errorf("'%s' логикалық оператор АҚИҚАТ типін талап етеді, бірақ %s берілді (оң жақ)", node.Operator, rightType)
			}
			return BOOL_TYPE

		case "жылжыту_сол", "жылжыту_оң":
			if leftType != INT_TYPE {
				tc.errorf("'%s' операторы сол жақтан бүтін сан талап етеді, бірақ %s берілді", node.Operator, leftType)
			}
			if rightType != INT_TYPE {
				tc.errorf("'%s' операторы оң жақтан бүтін сан талап етеді, бірақ %s берілді", node.Operator, rightType)
			}
			return INT_TYPE
		}
		return UNKNOWN

	// --- Struct Operations ---
	case *parser.StructStatement:
		return VOID_TYPE

	case *parser.InterfaceStatement:
		tc.interfaces[node.Name] = node
		for _, method := range node.Methods {
			funcName := node.Name + "." + method.Name
			paramTypes := []Type{Type("ИНТЕРФЕЙС_" + node.Name)}
			params := []string{"өзі"}
			for _, p := range method.Parameters {
				paramTypes = append(paramTypes, tc.fieldTypeToType(p))
				params = append(params, "арг")
			}
			retType := tc.fieldTypeToType(method.ReturnType)
			if method.ReturnType == "" {
				retType = VOID_TYPE
			}
			tc.funcs[funcName] = &FuncSig{
				Params:     params,
				ParamTypes: paramTypes,
				ReturnType: retType,
			}
		}
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
			if strings.HasPrefix(fieldTypeStr, "әлсіз_") {
				return Type("ҚҰРЫЛЫМ_" + fieldTypeStr[len("әлсіз_"):])
			}
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
			if strings.HasPrefix(fieldTypeStr, "әлсіз_") {
				expectedType = Type("ҚҰРЫЛЫМ_" + fieldTypeStr[len("әлсіз_"):])
			} else {
				expectedType = Type("ҚҰРЫЛЫМ_" + fieldTypeStr)
			}
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
				if strings.HasPrefix(string(existingType), "ИНТЕРФЕЙС_") && strings.HasPrefix(string(valType), "ҚҰРЫЛЫМ_") && tc.satisfiesInterface(valType, existingType) {
					// Allowed interface assignment
				} else if strings.HasPrefix(string(existingType), "ҚҰРЫЛЫМ_") && valType == INT_TYPE {
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
				if sig.ReturnType == UNKNOWN {
					sig.ReturnType = ret
				} else {
					current := sig.ReturnType
					if IsResultType(current) {
						underlying := GetResultUnderlyingType(current)
						if IsResultType(ret) {
							retUnderlying := GetResultUnderlyingType(ret)
							if underlying == UNKNOWN && retUnderlying != UNKNOWN {
								sig.ReturnType = MakeResultType(retUnderlying)
							}
						} else {
							// Return non-result value in a result function -> allowed
							if underlying == UNKNOWN {
								sig.ReturnType = MakeResultType(ret)
							} else if underlying != ret && ret != UNKNOWN {
								if strings.HasPrefix(string(underlying), "ИНТЕРФЕЙС_") && strings.HasPrefix(string(ret), "ҚҰРЫЛЫМ_") && tc.satisfiesInterface(ret, underlying) {
									// Allowed interface return in result
								} else {
									tc.errorf("типтер сәйкес емес: функция %s типті нәтиже күтеді, бірақ %s берілді", underlying, ret)
								}
							}
						}
					} else {
						// Current type is not Result
						if IsResultType(ret) {
							// We return a Result, so function now returns Result<current>
							sig.ReturnType = MakeResultType(current)
						} else if current != ret && ret != UNKNOWN {
							if strings.HasPrefix(string(current), "ИНТЕРФЕЙС_") && strings.HasPrefix(string(ret), "ҚҰРЫЛЫМ_") && tc.satisfiesInterface(ret, current) {
								// Allowed interface return
							} else {
								tc.errorf("типтер сәйкес емес: функция %s типті мән қайтаруы керек, бірақ %s берілді", current, ret)
							}
						}
					}
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
				} else if ok && strings.HasPrefix(string(t), "ИНТЕРФЕЙС_") {
					interfaceName := string(t)[len("ИНТЕРФЕЙС_"):]
					node.Arguments = append([]parser.Expression{&parser.Identifier{Value: varName}}, node.Arguments...)
					node.Function = interfaceName + "." + methodName
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
			// Интерфейс конструкторы/каст: ДыбысШығарғыш(ит) → ИНТЕРФЕЙС_ДыбысШығарғыш
			if _, okInterf := tc.interfaces[node.Function]; okInterf {
				if len(node.Arguments) != 1 {
					tc.errorf("'%s' интерфейс конструкторы 1 аргумент талап етеді, бірақ %d берілді", node.Function, len(node.Arguments))
					return UNKNOWN
				}
				argType := tc.Check(node.Arguments[0], env)
				if !tc.satisfiesInterface(argType, Type("ИНТЕРФЕЙС_"+node.Function)) {
					// During type inference passes, struct type may not be resolved yet — allow UNKNOWN
					if argType != UNKNOWN {
						tc.errorf("'%s' типі '%s' интерфейсін қанағаттандырмайды", argType, node.Function)
					}
					return Type("ИНТЕРФЕЙС_" + node.Function)
				}
				return Type("ИНТЕРФЕЙС_" + node.Function)
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
				expectedType := sig.ParamTypes[i]
				if expectedType == UNKNOWN || expectedType == "" {
					sig.ParamTypes[i] = argType
				} else if expectedType != argType && argType != UNKNOWN {
					if strings.HasPrefix(string(expectedType), "ИНТЕРФЕЙС_") && strings.HasPrefix(string(argType), "ҚҰРЫЛЫМ_") && tc.satisfiesInterface(argType, expectedType) {
						// Allowed interface satisfaction
					} else if expectedType == NUMBER_TYPE && argType == INT_TYPE {
						// Promotion allowed
					} else if strings.HasPrefix(string(expectedType), "ҚҰРЫЛЫМ_") && argType == INT_TYPE {
						// Null pointer initializer allowed
					} else {
						tc.errorf("'%s' функциясының %d-аргументі %s типін күтеді, бірақ %s берілді", node.Function, i+1, expectedType, argType)
					}
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

	case *parser.ThreadStatement:
		tc.Check(node.Body, env)
		return NUMBER_TYPE

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

	case *parser.ErrorLiteral:
		tc.Check(node.Message, env)
		return RESULT_TYPE

	case *parser.TryErrorExpression:
		leftType := tc.Check(node.Left, env)
		if !IsResultType(leftType) {
			tc.errorf("'қатемен' операторы тек нәтиже (Result) типі үшін қолданыла алады, бірақ %s берілді", leftType)
			return UNKNOWN
		}
		underlying := GetResultUnderlyingType(leftType)
		blockEnv := NewEnclosedTypeEnv(env)
		blockEnv.Set(node.VarName, STRING_TYPE)
		tc.Check(node.Block, blockEnv)
		return underlying

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
		if _, ok := tc.interfaces[ft]; ok {
			return Type("ИНТЕРФЕЙС_" + ft)
		}
		return Type("ҚҰРЫЛЫМ_" + ft)
	}
}

func (tc *TypeChecker) satisfiesInterface(structType Type, interfaceType Type) bool {
	if !strings.HasPrefix(string(structType), "ҚҰРЫЛЫМ_") || !strings.HasPrefix(string(interfaceType), "ИНТЕРФЕЙС_") {
		return false
	}
	structName := string(structType)[len("ҚҰРЫЛЫМ_"):]
	interfaceName := string(interfaceType)[len("ИНТЕРФЕЙС_"):]

	interf, ok := tc.interfaces[interfaceName]
	if !ok {
		return false
	}

	for _, method := range interf.Methods {
		funcName := structName + "." + method.Name
		sig, ok := tc.funcs[funcName]
		if !ok {
			return false
		}
		
		// Сравниваем параметры. В структуре первый параметр — өзі (тип structType)
		expectedParamsCount := len(method.Parameters) + 1
		if len(sig.ParamTypes) != expectedParamsCount {
			return false
		}
		
		// Первый параметр должен быть типом структуры
		if sig.ParamTypes[0] != structType {
			return false
		}
		
		// Остальные параметры должны совпадать
		for i, pTypeStr := range method.Parameters {
			expectedType := tc.fieldTypeToType(pTypeStr)
			if sig.ParamTypes[i+1] != expectedType {
				return false
			}
		}

		// Возвращаемый тип должен совпадать
		expectedRet := tc.fieldTypeToType(method.ReturnType)
		if method.ReturnType == "" {
			expectedRet = VOID_TYPE
		}
		if sig.ReturnType != expectedRet {
			return false
		}
	}

	return true
}

func (tc *TypeChecker) SatisfiesInterface(structType Type, interfaceType Type) bool {
	return tc.satisfiesInterface(structType, interfaceType)
}

func (tc *TypeChecker) FieldTypeToType(ft string) Type {
	return tc.fieldTypeToType(ft)
}


// ---------------------------------------------------------------------------
// Function registration (first pass)
// ---------------------------------------------------------------------------

func (tc *TypeChecker) registerFunction(fs *parser.FunctionStatement, env *TypeEnv) {
	env.Set(fs.Name, VOID_TYPE)
	if _, exists := tc.funcs[fs.Name]; exists {
		return
	}
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
}
