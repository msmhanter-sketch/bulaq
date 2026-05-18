package vm

import (
	"butaq/parser"
	"fmt"
)

type Type string

const (
	NUMBER_TYPE Type = "САН"
	STRING_TYPE Type = "МӘТІН"
	BOOL_TYPE   Type = "АҚИҚАТ"
	NULL_TYPE   Type = "БОС"
)

type Object interface {
	Type() Type
	Inspect() string
}

type Number struct {
	Value float64
}

func (n *Number) Type() Type      { return NUMBER_TYPE }
func (n *Number) Inspect() string { return fmt.Sprintf("%v", n.Value) }

type String struct {
	Value string
}

func (s *String) Type() Type      { return STRING_TYPE }
func (s *String) Inspect() string { return s.Value }

type Boolean struct {
	Value bool
}

func (b *Boolean) Type() Type      { return BOOL_TYPE }
func (b *Boolean) Inspect() string { return fmt.Sprintf("%v", b.Value) }

type Null struct{}

func (n *Null) Type() Type      { return NULL_TYPE }
func (n *Null) Inspect() string { return "бос" }

var NULL = &Null{}

type Environment struct {
	store map[string]Object
	outer *Environment
}

func NewEnvironment() *Environment {
	s := make(map[string]Object)
	return &Environment{store: s, outer: nil}
}

func NewEnclosedEnvironment(outer *Environment) *Environment {
	env := NewEnvironment()
	env.outer = outer
	return env
}

func (e *Environment) Get(name string) (Object, bool) {
	obj, ok := e.store[name]
	if !ok && e.outer != nil {
		obj, ok = e.outer.Get(name)
	}
	return obj, ok
}

func (e *Environment) Set(name string, val Object) Object {
	e.store[name] = val
	return val
}

type VM struct {
	env *Environment
}

func New() *VM {
	return &VM{env: NewEnvironment()}
}

func (v *VM) Eval(node parser.Node, env *Environment) Object {
	switch node := node.(type) {

	case *parser.Program:
		var result Object
		for _, stmt := range node.Statements {
			result = v.Eval(stmt, env)
		}
		return result

	case *parser.ExpressionStatement:
		return v.Eval(node.Expression, env)

	case *parser.NumberLiteral:
		return &Number{Value: node.Value}

	case *parser.StringLiteral:
		return &String{Value: node.Value}

	case *parser.Identifier:
		val, ok := env.Get(node.Value)
		if !ok {
			fmt.Printf("Қате: '%s' табылмады\n", node.Value)
			return NULL
		}
		return val

	case *parser.PostfixExpression:
		left := v.Eval(node.Left, env)
		right := v.Eval(node.Right, env)
		return v.evalPostfixExpression(node.Operator, left, right)

	case *parser.VarAssignStatement:
		val := v.Eval(node.Value, env)
		env.Set(node.Name.Value, val)
		return val

	case *parser.PrintStatement:
		val := v.Eval(node.Value, env)
		fmt.Println(val.Inspect())
		return NULL

	case *parser.BlockStatement:
		var result Object
		for _, stmt := range node.Statements {
			result = v.Eval(stmt, env)
		}
		return result

	case *parser.IfStatement:
		cond := v.Eval(node.Condition, env)
		if isTruthy(cond) {
			return v.Eval(node.Consequence, env)
		} else if node.Alternative != nil {
			return v.Eval(node.Alternative, env)
		} else {
			return NULL
		}

	case *parser.WhileStatement:
		for {
			cond := v.Eval(node.Condition, env)
			if !isTruthy(cond) {
				break
			}
			v.Eval(node.Body, env)
		}
		return NULL
	}

	return nil
}

func (v *VM) evalPostfixExpression(operator string, left, right Object) Object {
	if left.Type() == NUMBER_TYPE && right.Type() == NUMBER_TYPE {
		l := left.(*Number).Value
		r := right.(*Number).Value

		switch operator {
		case "қосу":
			return &Number{Value: l + r}
		case "алу":
			return &Number{Value: l - r}
		case "көбейту":
			return &Number{Value: l * r}
		case "бөлу":
			return &Number{Value: l / r}
		case "үлкен":
			return &Boolean{Value: l > r}
		case "кіші":
			return &Boolean{Value: l < r}
		case "тең":
			return &Boolean{Value: l == r}
		}
	}

	if left.Type() == STRING_TYPE && right.Type() == STRING_TYPE && operator == "қосу" {
		l := left.(*String).Value
		r := right.(*String).Value
		return &String{Value: l + r}
	}

	fmt.Printf("Қате: оператор %s үшін типтер сәйкес емес (%s, %s)\n", operator, left.Type(), right.Type())
	return NULL
}

func isTruthy(obj Object) bool {
	if obj == NULL {
		return false
	}
	if b, ok := obj.(*Boolean); ok {
		return b.Value
	}
	return true
}
