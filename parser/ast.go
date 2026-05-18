package parser

import (
	"bytes"
)

type Node interface {
	TokenLiteral() string
	String() string
}

type Statement interface {
	Node
	statementNode()
}

type Expression interface {
	Node
	expressionNode()
}

type Program struct {
	Statements []Statement
}

func (p *Program) TokenLiteral() string {
	if len(p.Statements) > 0 {
		return p.Statements[0].TokenLiteral()
	}
	return ""
}

func (p *Program) String() string {
	var out bytes.Buffer
	for _, s := range p.Statements {
		out.WriteString(s.String())
		out.WriteString("\n")
	}
	return out.String()
}

type BlockStatement struct {
	Statements []Statement
}

func (b *BlockStatement) statementNode()       {}
func (b *BlockStatement) TokenLiteral() string { return "{" }
func (b *BlockStatement) String() string {
	var out bytes.Buffer
	for _, s := range b.Statements {
		out.WriteString(s.String())
	}
	return out.String()
}

// x 5 болсын (x = 5)
type VarAssignStatement struct {
	Name  *Identifier
	Value Expression
}

func (vs *VarAssignStatement) statementNode()       {}
func (vs *VarAssignStatement) TokenLiteral() string { return "айнымалы/болсын" }
func (vs *VarAssignStatement) String() string {
	return vs.Name.String() + " " + vs.Value.String() + " болсын"
}

// "Сәлем" жазу (print "Сәлем")
type PrintStatement struct {
	Value Expression
}

func (ps *PrintStatement) statementNode()       {}
func (ps *PrintStatement) TokenLiteral() string { return "жазу" }
func (ps *PrintStatement) String() string {
	return ps.Value.String() + " жазу"
}

// 5 10 қосу
type PostfixExpression struct {
	Left     Expression
	Right    Expression
	Operator string
}

func (pe *PostfixExpression) expressionNode()      {}
func (pe *PostfixExpression) TokenLiteral() string { return pe.Operator }
func (pe *PostfixExpression) String() string {
	return "(" + pe.Left.String() + " " + pe.Right.String() + " " + pe.Operator + ")"
}

type Identifier struct {
	Value string
}

func (i *Identifier) expressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.Value }
func (i *Identifier) String() string       { return i.Value }

type NumberLiteral struct {
	Value float64
}

func (nl *NumberLiteral) expressionNode()      {}
func (nl *NumberLiteral) TokenLiteral() string { return "number" }
func (nl *NumberLiteral) String() string       { return "number" } // simplified

type StringLiteral struct {
	Value string
}

func (sl *StringLiteral) expressionNode()      {}
func (sl *StringLiteral) TokenLiteral() string { return sl.Value }
func (sl *StringLiteral) String() string       { return `"` + sl.Value + `"` }

// x 5 үлкен егер { ... } әйтпесе { ... }
type IfStatement struct {
	Condition   Expression
	Consequence *BlockStatement
	Alternative *BlockStatement
}

func (is *IfStatement) statementNode()       {}
func (is *IfStatement) TokenLiteral() string { return "егер" }
func (is *IfStatement) String() string {
	out := is.Condition.String() + " егер { " + is.Consequence.String() + " }"
	if is.Alternative != nil {
		out += " әйтпесе { " + is.Alternative.String() + " }"
	}
	return out
}

// x 10 кіші әзірше { ... }
type WhileStatement struct {
	Condition Expression
	Body      *BlockStatement
}

func (ws *WhileStatement) statementNode()       {}
func (ws *WhileStatement) TokenLiteral() string { return "әзірше" }
func (ws *WhileStatement) String() string {
	return ws.Condition.String() + " әзірше { " + ws.Body.String() + " }"
}

// General Expression Statement for standalone expressions
type ExpressionStatement struct {
	Expression Expression
}

func (es *ExpressionStatement) statementNode()       {}
func (es *ExpressionStatement) TokenLiteral() string { return es.Expression.TokenLiteral() }
func (es *ExpressionStatement) String() string {
	if es.Expression != nil {
		return es.Expression.String()
	}
	return ""
}
