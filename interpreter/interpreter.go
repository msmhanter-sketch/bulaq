package interpreter

import (
	"butaq/parser"
	"fmt"
	"strings"
)

// Environment holds variables and functions
type Environment struct {
	vars   map[string]interface{}
	parent *Environment
}

func NewEnvironment(parent *Environment) *Environment {
	return &Environment{
		vars:   make(map[string]interface{}),
		parent: parent,
	}
}

func (e *Environment) Get(name string) (interface{}, bool) {
	val, ok := e.vars[name]
	if !ok && e.parent != nil {
		return e.parent.Get(name)
	}
	return val, ok
}

func (e *Environment) Set(name string, val interface{}) {
	if e.parent != nil {
		if _, ok := e.parent.Get(name); ok {
			e.parent.Set(name, val)
			return
		}
	}
	e.vars[name] = val
}

func (e *Environment) SetLocal(name string, val interface{}) {
	e.vars[name] = val
}

// User-defined function representation
type UserFunction struct {
	Params []string
	Body   *parser.BlockStatement
}

// Struct instance representation
type StructInstance struct {
	Name   string
	Fields map[string]interface{}
}

// Interpreter executes the program AST
type Interpreter struct {
	Output strings.Builder
	env    *Environment
	funcs  map[string]*UserFunction
	structDefs map[string]*parser.StructStatement
}

func New() *Interpreter {
	return &Interpreter{
		env:        NewEnvironment(nil),
		funcs:      make(map[string]*UserFunction),
		structDefs: make(map[string]*parser.StructStatement),
	}
}

// Return/Break/Continue signals
type ReturnValue struct{ Value interface{} }
type BreakSignal struct{}
type ContinueSignal struct{}

func (ip *Interpreter) Run(program *parser.Program) (string, error) {
	ip.Output.Reset()
	for _, stmt := range program.Statements {
		res, err := ip.evalStatement(stmt, ip.env)
		if err != nil {
			return "", err
		}
		if _, ok := res.(ReturnValue); ok {
			break
		}
	}
	return ip.Output.String(), nil
}

func (ip *Interpreter) evalStatement(stmt parser.Statement, env *Environment) (interface{}, error) {
	switch s := stmt.(type) {
	case *parser.ExpressionStatement:
		return ip.evalExpression(s.Expression, env)

	case *parser.VarAssignStatement:
		val, err := ip.evalExpression(s.Value, env)
		if err != nil {
			return nil, err
		}
		env.Set(s.Name.Value, val)
		return nil, nil

	case *parser.PrintStatement:
		val, err := ip.evalExpression(s.Value, env)
		if err != nil {
			return nil, err
		}
		ip.Output.WriteString(fmt.Sprintf("%v\n", val))
		return nil, nil

	case *parser.BlockStatement:
		blockEnv := NewEnvironment(env)
		for _, subStmt := range s.Statements {
			res, err := ip.evalStatement(subStmt, blockEnv)
			if err != nil {
				return nil, err
			}
			if res != nil {
				return res, nil // propagates ReturnValue/Break/Continue
			}
		}
		return nil, nil

	case *parser.IfStatement:
		cond, err := ip.evalExpression(s.Condition, env)
		if err != nil {
			return nil, err
		}
		if isTruthy(cond) {
			return ip.evalStatement(s.Consequence, env)
		} else if s.Alternative != nil {
			return ip.evalStatement(s.Alternative, env)
		}
		return nil, nil

	case *parser.WhileStatement:
		for {
			cond, err := ip.evalExpression(s.Condition, env)
			if err != nil {
				return nil, err
			}
			if !isTruthy(cond) {
				break
			}
			res, err := ip.evalStatement(s.Body, env)
			if err != nil {
				return nil, err
			}
			if res != nil {
				if _, ok := res.(BreakSignal); ok {
					break
				}
				if _, ok := res.(ContinueSignal); ok {
					continue
				}
				return res, nil // propagates ReturnValue
			}
		}
		return nil, nil

	case *parser.BreakStatement:
		return BreakSignal{}, nil

	case *parser.ContinueStatement:
		return ContinueSignal{}, nil

	case *parser.ReturnStatement:
		val, err := ip.evalExpression(s.Value, env)
		if err != nil {
			return nil, err
		}
		return ReturnValue{Value: val}, nil

	case *parser.FunctionStatement:
		ip.funcs[s.Name] = &UserFunction{
			Params: s.Parameters,
			Body:   s.Body,
		}
		return nil, nil

	case *parser.StructStatement:
		ip.structDefs[s.Name] = s
		return nil, nil

	case *parser.StructFieldAssignStatement:
		targetObj, err := ip.evalExpression(s.Target, env)
		if err != nil {
			return nil, err
		}
		instance, ok := targetObj.(*StructInstance)
		if !ok {
			return nil, fmt.Errorf("field assignment on non-struct instance")
		}
		val, err := ip.evalExpression(s.Value, env)
		if err != nil {
			return nil, err
		}
		instance.Fields[s.Field] = val
		return nil, nil

	case *parser.IndexAssignStatement:
		arrObj, ok := env.Get(s.Array.Value)
		if !ok {
			return nil, fmt.Errorf("array not found: %s", s.Array.Value)
		}
		arr, ok := arrObj.([]interface{})
		if !ok {
			return nil, fmt.Errorf("object is not an array")
		}
		idxVal, err := ip.evalExpression(s.Index, env)
		if err != nil {
			return nil, err
		}
		idx, ok := toInt64(idxVal)
		if !ok || idx < 0 || idx >= int64(len(arr)) {
			return nil, fmt.Errorf("invalid index: %v", idxVal)
		}
		val, err := ip.evalExpression(s.Value, env)
		if err != nil {
			return nil, err
		}
		arr[idx] = val
		return nil, nil
	}

	return nil, nil
}

func (ip *Interpreter) evalExpression(expr parser.Expression, env *Environment) (interface{}, error) {
	if expr == nil {
		return nil, nil
	}

	switch e := expr.(type) {
	case *parser.IntLiteral:
		return e.Value, nil
	case *parser.NumberLiteral:
		return e.Value, nil
	case *parser.StringLiteral:
		return e.Value, nil
	case *parser.BoolLiteral:
		return e.Value, nil

	case *parser.Identifier:
		val, ok := env.Get(e.Value)
		if !ok {
			return nil, fmt.Errorf("identifier not found: %s", e.Value)
		}
		return val, nil

	case *parser.PostfixExpression:
		left, err := ip.evalExpression(e.Left, env)
		if err != nil {
			return nil, err
		}
		right, err := ip.evalExpression(e.Right, env)
		if err != nil {
			return nil, err
		}
		return evalBinaryOp(left, right, e.Operator)

	case *parser.UnaryExpression:
		right, err := ip.evalExpression(e.Right, env)
		if err != nil {
			return nil, err
		}
		if e.Operator == "емес" {
			return !isTruthy(right), nil
		}
		return nil, fmt.Errorf("unknown unary operator: %s", e.Operator)

	case *parser.ArrayLiteral:
		var elems []interface{}
		for _, el := range e.Elements {
			val, err := ip.evalExpression(el, env)
			if err != nil {
				return nil, err
			}
			elems = append(elems, val)
		}
		return elems, nil

	case *parser.IndexExpression:
		left, err := ip.evalExpression(e.Left, env)
		if err != nil {
			return nil, err
		}
		idxVal, err := ip.evalExpression(e.Index, env)
		if err != nil {
			return nil, err
		}
		arr, ok := left.([]interface{})
		if !ok {
			return nil, fmt.Errorf("index access on non-array")
		}
		idx, ok := toInt64(idxVal)
		if !ok || idx < 0 || idx >= int64(len(arr)) {
			return nil, fmt.Errorf("invalid index: %v", idxVal)
		}
		return arr[idx], nil

	case *parser.LengthExpression:
		val, err := ip.evalExpression(e.Value, env)
		if err != nil {
			return nil, err
		}
		if arr, ok := val.([]interface{}); ok {
			return int64(len(arr)), nil
		}
		return nil, fmt.Errorf("length of non-array")
	
	case *parser.StrLenExpression:
		val, err := ip.evalExpression(e.Value, env)
		if err != nil {
			return nil, err
		}
		if str, ok := val.(string); ok {
			return int64(len(str)), nil
		}
		return nil, fmt.Errorf("strlen of non-string")

	case *parser.StrConcatExpression:
		left, err := ip.evalExpression(e.Left, env)
		if err != nil {
			return nil, err
		}
		right, err := ip.evalExpression(e.Right, env)
		if err != nil {
			return nil, err
		}
		return fmt.Sprintf("%v%v", left, right), nil

	case *parser.StructCreateExpression:
		def, ok := ip.structDefs[e.StructName]
		if !ok {
			return nil, fmt.Errorf("struct definition not found: %s", e.StructName)
		}
		fields := make(map[string]interface{})
		for _, fName := range def.Fields {
			fields[fName] = nil
		}
		return &StructInstance{
			Name:   e.StructName,
			Fields: fields,
		}, nil

	case *parser.StructFieldAccessExpression:
		targetObj, err := ip.evalExpression(e.Target, env)
		if err != nil {
			return nil, err
		}
		instance, ok := targetObj.(*StructInstance)
		if !ok {
			return nil, fmt.Errorf("field access on non-struct")
		}
		val, ok := instance.Fields[e.Field]
		if !ok {
			return nil, fmt.Errorf("field %s not found on struct %s", e.Field, instance.Name)
		}
		return val, nil

	case *parser.CallExpression:
		// Execute user function or built-in
		fn, ok := ip.funcs[e.Function]
		if !ok {
			// Check builtins
			return ip.callBuiltin(e.Function, e.Arguments, env)
		}

		// Evaluate args
		var args []interface{}
		for _, arg := range e.Arguments {
			val, err := ip.evalExpression(arg, env)
			if err != nil {
				return nil, err
			}
			args = append(args, val)
		}

		// Bind parameters
		fnEnv := NewEnvironment(ip.env)
		for idx, param := range fn.Params {
			if idx < len(args) {
				fnEnv.SetLocal(param, args[idx])
			} else {
				fnEnv.SetLocal(param, nil)
			}
		}

		// Execute function body
		res, err := ip.evalStatement(fn.Body, fnEnv)
		if err != nil {
			return nil, err
		}

		if ret, ok := res.(ReturnValue); ok {
			return ret.Value, nil
		}
		return nil, nil
	}

	return nil, nil
}

func (ip *Interpreter) callBuiltin(name string, args []parser.Expression, env *Environment) (interface{}, error) {
	// Simple implementations of common built-ins for Playground demonstration
	switch name {
	case "мд5":
		val, _ := ip.evalExpression(args[0], env)
		return fmt.Sprintf("[MD5 placeholder for %v]", val), nil
	case "ша256":
		val, _ := ip.evalExpression(args[0], env)
		return fmt.Sprintf("[SHA256 placeholder for %v]", val), nil
	}
	return nil, fmt.Errorf("unknown function: %s", name)
}

func evalBinaryOp(left, right interface{}, op string) (interface{}, error) {
	switch op {
	case "қосу":
		if l, ok := toFloat64(left); ok {
			if r, ok := toFloat64(right); ok {
				return l + r, nil
			}
		}
	case "алу":
		if l, ok := toFloat64(left); ok {
			if r, ok := toFloat64(right); ok {
				return l - r, nil
			}
		}
	case "көбейту":
		if l, ok := toFloat64(left); ok {
			if r, ok := toFloat64(right); ok {
				return l * r, nil
			}
		}
	case "бөлу":
		if l, ok := toFloat64(left); ok {
			if r, ok := toFloat64(right); ok {
				if r == 0 {
					return nil, fmt.Errorf("division by zero")
				}
				return l / r, nil
			}
		}
	case "тең":
		return left == right, nil
	case "тең_емес":
		return left != right, nil
	case "үлкен":
		if l, ok := toFloat64(left); ok {
			if r, ok := toFloat64(right); ok {
				return l > r, nil
			}
		}
	case "кіші":
		if l, ok := toFloat64(left); ok {
			if r, ok := toFloat64(right); ok {
				return l < r, nil
			}
		}
	case "үлкен_тең":
		if l, ok := toFloat64(left); ok {
			if r, ok := toFloat64(right); ok {
				return l >= r, nil
			}
		}
	case "кіші_тең":
		if l, ok := toFloat64(left); ok {
			if r, ok := toFloat64(right); ok {
				return l <= r, nil
			}
		}
	}
	return nil, fmt.Errorf("invalid operation: %v %s %v", left, op, right)
}

func isTruthy(val interface{}) bool {
	if val == nil {
		return false
	}
	if b, ok := val.(bool); ok {
		return b
	}
	return true
}

func toFloat64(val interface{}) (float64, bool) {
	switch v := val.(type) {
	case int64:
		return float64(v), true
	case float64:
		return v, true
	}
	return 0, false
}

func toInt64(val interface{}) (int64, bool) {
	switch v := val.(type) {
	case int64:
		return v, true
	case float64:
		return int64(v), true
	}
	return 0, false
}
