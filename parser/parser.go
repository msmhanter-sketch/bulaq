package parser

import (
	"butaq/lexer"
	"fmt"
	"strconv"
)

type Parser struct {
	l *lexer.Lexer

	curToken  lexer.Token
	peekToken lexer.Token
	errors    []string
}

func New(l *lexer.Lexer) *Parser {
	p := &Parser{l: l, errors: []string{}}
	p.nextToken()
	p.nextToken()
	return p
}

func (p *Parser) Errors() []string {
	return p.errors
}

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) ParseProgram() *Program {
	program := &Program{}
	program.Statements = []Statement{}

	for p.curToken.Type != lexer.EOF {
		stmt := p.parseStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		p.nextToken()
	}

	return program
}

func (p *Parser) parseStatement() Statement {
	node := p.parseExpression()

	if node == nil {
		return nil
	}

	if stmt, ok := node.(Statement); ok {
		return stmt
	}

	if expr, ok := node.(Expression); ok {
		return &ExpressionStatement{Expression: expr}
	}

	return nil
}

func (p *Parser) parseExpression() Node {
	var stack []Node

	for p.curToken.Type != lexer.EOF && p.curToken.Type != lexer.LBRACE && p.curToken.Type != lexer.RBRACE {

		switch p.curToken.Type {
		case lexer.NUMBER:
			val, _ := strconv.ParseFloat(p.curToken.Literal, 64)
			stack = append(stack, &NumberLiteral{Value: val})
		case lexer.STRING:
			stack = append(stack, &StringLiteral{Value: p.curToken.Literal})
		case lexer.IDENTIFIER:
			stack = append(stack, &Identifier{Value: p.curToken.Literal})
		case lexer.VAR:
			// ignore "айнымалы"
		case lexer.PLUS, lexer.MINUS, lexer.MUL, lexer.DIV, lexer.GT, lexer.LT, lexer.EQ:
			if len(stack) < 2 {
				p.errors = append(p.errors, fmt.Sprintf("Operator %s requires 2 operands", p.curToken.Literal))
				return nil
			}
			right := stack[len(stack)-1].(Expression)
			left := stack[len(stack)-2].(Expression)
			stack = stack[:len(stack)-2]

			stack = append(stack, &PostfixExpression{
				Left:     left,
				Right:    right,
				Operator: p.curToken.Literal,
			})

		case lexer.ASSIGN:
			if len(stack) < 2 {
				p.errors = append(p.errors, "болсын requires an identifier and a value")
				return nil
			}
			value := stack[len(stack)-1].(Expression)
			ident, ok := stack[len(stack)-2].(*Identifier)
			if !ok {
				p.errors = append(p.errors, "Left of болсын must be identifier")
				return nil
			}
			stack = stack[:len(stack)-2]

			stmt := &VarAssignStatement{Name: ident, Value: value}
			stack = append(stack, stmt)
			return stack[0]

		case lexer.PRINT:
			if len(stack) < 1 {
				p.errors = append(p.errors, "жазу requires 1 operand")
				return nil
			}
			val := stack[len(stack)-1].(Expression)
			stack = stack[:len(stack)-1]

			stmt := &PrintStatement{Value: val}
			stack = append(stack, stmt)
			return stack[0]

		case lexer.IF:
			if len(stack) < 1 {
				p.errors = append(p.errors, "егер requires a condition")
				return nil
			}
			cond := stack[len(stack)-1].(Expression)
			stack = stack[:len(stack)-1]

			stmt := &IfStatement{Condition: cond}

			p.nextToken()
			if p.curToken.Type != lexer.LBRACE {
				p.errors = append(p.errors, "Expected { after егер")
				return nil
			}
			stmt.Consequence = p.parseBlockStatement()

			if p.peekToken.Type == lexer.ELSE {
				p.nextToken()
				p.nextToken()
				if p.curToken.Type != lexer.LBRACE {
					p.errors = append(p.errors, "Expected { after әйтпесе")
					return nil
				}
				stmt.Alternative = p.parseBlockStatement()
			}

			stack = append(stack, stmt)
			return stack[0]

		case lexer.WHILE:
			if len(stack) < 1 {
				p.errors = append(p.errors, "әзірше requires a condition")
				return nil
			}
			cond := stack[len(stack)-1].(Expression)
			stack = stack[:len(stack)-1]

			stmt := &WhileStatement{Condition: cond}

			p.nextToken()
			if p.curToken.Type != lexer.LBRACE {
				p.errors = append(p.errors, "Expected { after әзірше")
				return nil
			}
			stmt.Body = p.parseBlockStatement()

			stack = append(stack, stmt)
			return stack[0]
		}

		if p.peekToken.Type == lexer.LBRACE || p.peekToken.Type == lexer.EOF || p.curToken.Type == lexer.RBRACE {
			break
		}

		p.nextToken()
	}

	if len(stack) > 0 {
		return stack[len(stack)-1]
	}

	return nil
}

func (p *Parser) parseBlockStatement() *BlockStatement {
	block := &BlockStatement{Statements: []Statement{}}
	p.nextToken()

	for p.curToken.Type != lexer.RBRACE && p.curToken.Type != lexer.EOF {
		stmt := p.parseStatement()
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}

	return block
}
