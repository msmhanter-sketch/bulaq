package typechecker

import (
	"butaq/parser"
	"fmt"
)

type LinterWarning struct {
	Message string
	Line    int
	Col     int
}

func (w LinterWarning) String() string {
	return fmt.Sprintf("Ескерту (%d:%d): %s", w.Line, w.Col, w.Message)
}

type LinterScope struct {
	vars      map[string]bool
	positions map[string]parser.Pos
	outer     *LinterScope
}

func NewLinterScope(outer *LinterScope) *LinterScope {
	return &LinterScope{
		vars:      make(map[string]bool),
		positions: make(map[string]parser.Pos),
		outer:     outer,
	}
}

func (s *LinterScope) Declare(name string, pos parser.Pos) {
	s.vars[name] = false
	s.positions[name] = pos
}

func (s *LinterScope) Read(name string) {
	curr := s
	for curr != nil {
		if _, exists := curr.vars[name]; exists {
			curr.vars[name] = true
			return
		}
		curr = curr.outer
	}
}

type Linter struct {
	Warnings []LinterWarning
}

func NewLinter() *Linter {
	return &Linter{
		Warnings: []LinterWarning{},
	}
}

func (l *Linter) warn(line, col int, msg string) {
	l.Warnings = append(l.Warnings, LinterWarning{
		Message: msg,
		Line:    line,
		Col:     col,
	})
}

func (l *Linter) checkUnused(scope *LinterScope) {
	for name, read := range scope.vars {
		if !read && name != "_" && name != "өзі" {
			pos := scope.positions[name]
			line, col := pos.Line, pos.Col
			l.warn(line, col, fmt.Sprintf("Айнымалы '%s' жарияланған, бірақ қолданылмаған / Переменная '%s' объявлена, но не используется", name, name))
		}
	}
}

func (l *Linter) Lint(node parser.Node) []LinterWarning {
	globalScope := NewLinterScope(nil)
	l.checkNode(node, globalScope)
	return l.Warnings
}

// isMagicNumber returns true for numeric literals that are not 0, 1 or -1
func isMagicNumber(v float64) bool {
	return v != 0 && v != 1 && v != -1 && v != 2 && v != 10 && v != 100
}


func (l *Linter) checkNode(node parser.Node, scope *LinterScope) {
	if node == nil {
		return
	}

	switch n := node.(type) {
	case *parser.Program:
		for _, stmt := range n.Statements {
			l.checkNode(stmt, scope)
		}
		l.checkUnused(scope)

	case *parser.FunctionStatement:
		funcScope := NewLinterScope(scope)
		for _, param := range n.Parameters {
			funcScope.Declare(param, n.Pos)
		}
		l.checkNode(n.Body, funcScope)
		l.checkUnused(funcScope)

	case *parser.BlockStatement:
		blockScope := NewLinterScope(scope)
		hasTerminated := false
		for _, stmt := range n.Statements {
			if hasTerminated {
				line, col := stmt.Position()
				l.warn(line, col, "Орындалмайтын код (қайтару/үзу/жалғастыру командасынан кейін орналасқан) / Недостижимый код (расположен после return/break/continue)")
				hasTerminated = false
			}
			l.checkNode(stmt, blockScope)
			switch stmt.(type) {
			case *parser.ReturnStatement, *parser.BreakStatement, *parser.ContinueStatement:
				hasTerminated = true
			}
		}
		l.checkUnused(blockScope)

	case *parser.VarAssignStatement:
		l.checkNode(n.Value, scope)
		// Check if it exists in any outer scope
		exists := false
		curr := scope
		for curr != nil {
			if _, ok := curr.vars[n.Name.Value]; ok {
				exists = true
				break
			}
			curr = curr.outer
		}
		if !exists {
			scope.Declare(n.Name.Value, n.Name.Pos)
		}

	case *parser.Identifier:
		scope.Read(n.Value)

	case *parser.PrintStatement:
		l.checkNode(n.Value, scope)

	case *parser.IfStatement:
		l.checkNode(n.Condition, scope)
		l.checkNode(n.Consequence, scope)
		if n.Alternative != nil {
			l.checkNode(n.Alternative, scope)
		}

	case *parser.WhileStatement:
		l.checkNode(n.Condition, scope)
		l.checkNode(n.Body, scope)

	case *parser.ThreadStatement:
		l.checkNode(n.Body, scope)

	case *parser.ReturnStatement:
		l.checkNode(n.Value, scope)

	case *parser.PostfixExpression:
		l.checkNode(n.Left, scope)
		l.checkNode(n.Right, scope)

	case *parser.UnaryExpression:
		l.checkNode(n.Right, scope)

	case *parser.CallExpression:
		for _, arg := range n.Arguments {
			l.checkNode(arg, scope)
		}

	case *parser.CallStatement:
		l.checkNode(n.Call, scope)

	case *parser.IndexExpression:
		l.checkNode(n.Left, scope)
		l.checkNode(n.Index, scope)

	case *parser.IndexAssignStatement:
		scope.Read(n.Array.Value)
		l.checkNode(n.Index, scope)
		l.checkNode(n.Value, scope)

	case *parser.StructCreateExpression:
		// No arguments to check

	case *parser.StructFieldAccessExpression:
		if n.Target != nil {
			l.checkNode(n.Target, scope)
		} else {
			scope.Read(n.StructName)
		}

	case *parser.StructFieldAssignStatement:
		if n.Target != nil {
			l.checkNode(n.Target, scope)
		} else {
			scope.Read(n.StructName)
		}
		l.checkNode(n.Value, scope)

	case *parser.ArrayLiteral:
		for _, el := range n.Elements {
			l.checkNode(el, scope)
		}

	case *parser.LengthExpression:
		l.checkNode(n.Value, scope)

	case *parser.CharAtExpression:
		l.checkNode(n.Str, scope)
		l.checkNode(n.Index, scope)

	case *parser.CharCodeExpression:
		l.checkNode(n.Value, scope)

	case *parser.StrConcatExpression:
		l.checkNode(n.Left, scope)
		l.checkNode(n.Right, scope)

	case *parser.StrEqExpression:
		l.checkNode(n.Left, scope)
		l.checkNode(n.Right, scope)

	case *parser.StrLenExpression:
		l.checkNode(n.Value, scope)

	case *parser.ToStrExpression:
		l.checkNode(n.Value, scope)

	case *parser.FileReadExpression:
		l.checkNode(n.Path, scope)

	case *parser.FileWriteStatement:
		l.checkNode(n.Path, scope)
		l.checkNode(n.Content, scope)

	case *parser.FreeStatement:
		l.checkNode(n.Value, scope)

	case *parser.ErrorLiteral:
		l.checkNode(n.Message, scope)

	case *parser.TryErrorExpression:
		l.checkNode(n.Left, scope)
		blockScope := NewLinterScope(scope)
		blockScope.Declare(n.VarName, n.Pos)
		l.checkNode(n.Block, blockScope)
		l.checkUnused(blockScope)

	case *parser.ExpressionStatement:
		l.checkNode(n.Expression, scope)
	}
}
