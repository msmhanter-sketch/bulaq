package codegen

import (
	"butaq/parser"
	"butaq/typechecker"
	"bytes"
	"fmt"
)

type Scope struct {
	declared map[string]bool
	outer    *Scope
}

func NewScope(outer *Scope) *Scope {
	return &Scope{
		declared: make(map[string]bool),
		outer:    outer,
	}
}

func (s *Scope) Declare(name string) {
	s.declared[name] = true
}

func (s *Scope) IsDeclared(name string) bool {
	if s.declared[name] {
		return true
	}
	if s.outer != nil {
		return s.outer.IsDeclared(name)
	}
	return false
}

type CppGenerator struct {
	env          *typechecker.TypeEnv
	currentScope *Scope
}

func New(env *typechecker.TypeEnv) *CppGenerator {
	cg := &CppGenerator{env: env}
	cg.currentScope = NewScope(nil)
	return cg
}

func (cg *CppGenerator) Generate(program *parser.Program) string {
	var out bytes.Buffer

	out.WriteString("#include <iostream>\n")
	out.WriteString("#include <string>\n\n")

	out.WriteString("int main() {\n")

	for _, stmt := range program.Statements {
		out.WriteString("\t" + cg.genStatement(stmt, 1) + "\n")
	}

	out.WriteString("\treturn 0;\n")
	out.WriteString("}\n")

	return out.String()
}

func indent(level int) string {
	return string(bytes.Repeat([]byte("\t"), level))
}

func (cg *CppGenerator) genStatement(node parser.Statement, level int) string {
	switch n := node.(type) {
	case *parser.VarAssignStatement:
		varType, exists := cg.env.Get(n.Name.Value)
		valExpr := cg.genExpression(n.Value)

		if !exists {
			return fmt.Sprintf("auto %s = %s;", n.Name.Value, valExpr)
		}

		if !cg.currentScope.IsDeclared(n.Name.Value) {
			cg.currentScope.Declare(n.Name.Value)

			cppType := "auto"
			if varType == typechecker.NUMBER_TYPE { cppType = "double" }
			if varType == typechecker.STRING_TYPE { cppType = "std::string" }
			if varType == typechecker.BOOL_TYPE { cppType = "bool" }

			return fmt.Sprintf("%s %s = %s;", cppType, n.Name.Value, valExpr)
		}

		return fmt.Sprintf("%s = %s;", n.Name.Value, valExpr)

	case *parser.PrintStatement:
		valExpr := cg.genExpression(n.Value)
		return fmt.Sprintf("std::cout << %s << std::endl;", valExpr)

	case *parser.IfStatement:
		condExpr := cg.genExpression(n.Condition)
		out := fmt.Sprintf("if (%s) {\n", condExpr)

		if n.Consequence != nil {
			previousScope := cg.currentScope
			cg.currentScope = NewScope(previousScope)

			for _, stmt := range n.Consequence.Statements {
				out += indent(level+1) + cg.genStatement(stmt, level+1) + "\n"
			}

			cg.currentScope = previousScope
		}
		out += indent(level) + "}"

		if n.Alternative != nil {
			out += " else {\n"

			previousScope := cg.currentScope
			cg.currentScope = NewScope(previousScope)

			for _, stmt := range n.Alternative.Statements {
				out += indent(level+1) + cg.genStatement(stmt, level+1) + "\n"
			}

			cg.currentScope = previousScope
			out += indent(level) + "}"
		}

		return out

	case *parser.WhileStatement:
		condExpr := cg.genExpression(n.Condition)
		out := fmt.Sprintf("while (%s) {\n", condExpr)

		if n.Body != nil {
			previousScope := cg.currentScope
			cg.currentScope = NewScope(previousScope)

			for _, stmt := range n.Body.Statements {
				out += indent(level+1) + cg.genStatement(stmt, level+1) + "\n"
			}

			cg.currentScope = previousScope
		}
		out += indent(level) + "}"

		return out

	case *parser.ExpressionStatement:
		return cg.genExpression(n.Expression) + ";"
	}

	return ""
}

func (cg *CppGenerator) genExpression(node parser.Expression) string {
	switch n := node.(type) {
	case *parser.NumberLiteral:
		return fmt.Sprintf("%v", n.Value)
	case *parser.StringLiteral:
		return fmt.Sprintf(`"%s"`, n.Value)
	case *parser.Identifier:
		return n.Value
	case *parser.PostfixExpression:
		left := cg.genExpression(n.Left)
		right := cg.genExpression(n.Right)

		op := ""
		switch n.Operator {
		case "қосу": op = "+"
		case "алу": op = "-"
		case "көбейту": op = "*"
		case "бөлу": op = "/"
		case "үлкен": op = ">"
		case "кіші": op = "<"
		case "тең": op = "=="
		}

		return fmt.Sprintf("(%s %s %s)", left, op, right)
	}
	return ""
}
