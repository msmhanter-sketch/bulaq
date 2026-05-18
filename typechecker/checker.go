package typechecker

import (
	"butaq/parser"
	"fmt"
)

// The TypeChecker runs statically over the AST before the VM executes it.

type Type string

const (
	NUMBER_TYPE Type = "САН"
	STRING_TYPE Type = "МӘТІН"
	BOOL_TYPE   Type = "АҚИҚАТ"
	UNKNOWN     Type = "БЕЛГІСІЗ"
)

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

type TypeChecker struct {
	env    *TypeEnv
	Errors []string
}

func New() *TypeChecker {
	return &TypeChecker{env: NewTypeEnv(), Errors: []string{}}
}

func (tc *TypeChecker) Check(node parser.Node, env *TypeEnv) Type {
	switch node := node.(type) {
	case *parser.Program:
		for _, stmt := range node.Statements {
			tc.Check(stmt, env)
		}
		return UNKNOWN

	case *parser.NumberLiteral:
		return NUMBER_TYPE

	case *parser.StringLiteral:
		return STRING_TYPE

	case *parser.Identifier:
		t, ok := env.Get(node.Value)
		if !ok {
			tc.Errors = append(tc.Errors, fmt.Sprintf("Тип қатесі (Type Error): айнымалы '%s' жарияланбаған (undeclared variable)", node.Value))
			return UNKNOWN
		}
		return t

	case *parser.PostfixExpression:
		leftType := tc.Check(node.Left, env)
		rightType := tc.Check(node.Right, env)

		if leftType == UNKNOWN || rightType == UNKNOWN {
			return UNKNOWN
		}

		if leftType != rightType {
			tc.Errors = append(tc.Errors, fmt.Sprintf("Тип қатесі (Type Error): типтер сәйкес емес (%s және %s %s үшін)", leftType, rightType, node.Operator))
			return UNKNOWN
		}

		// Arithmetic and comparison both require same types
		switch node.Operator {
		case "қосу", "алу", "көбейту", "бөлу":
			if leftType != NUMBER_TYPE && !(leftType == STRING_TYPE && node.Operator == "қосу") {
				tc.Errors = append(tc.Errors, fmt.Sprintf("Тип қатесі: '%s' операторы %s типіне қолданылмайды", node.Operator, leftType))
				return UNKNOWN
			}
			return leftType
		case "үлкен", "кіші", "тең":
			return BOOL_TYPE
		}

	case *parser.VarAssignStatement:
		valType := tc.Check(node.Value, env)
		existingType, ok := env.Get(node.Name.Value)

		// Static Typing Logic: Type inference on first assignment.
		if !ok {
			env.Set(node.Name.Value, valType)
		} else {
			// If already exists, statically enforce it retains the same type.
			if existingType != valType && valType != UNKNOWN {
				tc.Errors = append(tc.Errors, fmt.Sprintf("Тип қатесі: '%s' айнымалысының типін өзгертуге болмайды (%s -> %s)", node.Name.Value, existingType, valType))
			}
		}
		return valType

	case *parser.PrintStatement:
		tc.Check(node.Value, env)
		return UNKNOWN

	case *parser.BlockStatement:
		blockEnv := NewEnclosedTypeEnv(env)
		for _, stmt := range node.Statements {
			tc.Check(stmt, blockEnv)
		}
		return UNKNOWN

	case *parser.IfStatement:
		condType := tc.Check(node.Condition, env)
		if condType != BOOL_TYPE && condType != UNKNOWN {
			tc.Errors = append(tc.Errors, fmt.Sprintf("Тип қатесі: 'егер' шарты АҚИҚАТ (boolean) болуы керек, бірақ ол %s", condType))
		}
		tc.Check(node.Consequence, env)
		if node.Alternative != nil {
			tc.Check(node.Alternative, env)
		}
		return UNKNOWN

	case *parser.WhileStatement:
		condType := tc.Check(node.Condition, env)
		if condType != BOOL_TYPE && condType != UNKNOWN {
			tc.Errors = append(tc.Errors, fmt.Sprintf("Тип қатесі: 'әзірше' шарты АҚИҚАТ (boolean) болуы керек, бірақ ол %s", condType))
		}
		tc.Check(node.Body, env)
		return UNKNOWN

	case *parser.ExpressionStatement:
		return tc.Check(node.Expression, env)
	}

	return UNKNOWN
}
