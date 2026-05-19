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

func (p *Parser) Errors() []string { return p.errors }

func (p *Parser) nextToken() {
	p.curToken = p.peekToken
	p.peekToken = p.l.NextToken()
}

func (p *Parser) errorf(format string, args ...interface{}) {
	msg := fmt.Sprintf("[жол %d, баған %d] %s", p.curToken.Line, p.curToken.Col, fmt.Sprintf(format, args...))
	p.errors = append(p.errors, msg)
}

// ---------------------------------------------------------------------------
// Top-level
// ---------------------------------------------------------------------------

func (p *Parser) ParseProgram() *Program {
	program := &Program{Statements: []Statement{}}
	for p.curToken.Type != lexer.EOF {
		stmt := p.parseTopLevelStatement()
		if stmt != nil {
			program.Statements = append(program.Statements, stmt)
		}
		p.nextToken()
	}
	return program
}

func (p *Parser) parseTopLevelStatement() Statement {
	if p.curToken.Type == lexer.FUNC {
		return p.parseFunctionStatement()
	}
	if p.curToken.Type == lexer.STRUCT {
		return p.parseStructStatement()
	}
	return p.parseStatement()
}

// құрылым Адам { аты жасы }
func (p *Parser) parseStructStatement() *StructStatement {
	p.nextToken() // cur = name
	if p.curToken.Type != lexer.IDENTIFIER {
		p.errorf("құрылым атауы күтілді")
		return nil
	}
	name := p.curToken.Literal
	p.nextToken() // cur = {

	if p.curToken.Type != lexer.LBRACE {
		p.errorf("'{' күтілді")
		return nil
	}
	p.nextToken() // cur = first field

	var fields []string
	var types []string
	for p.curToken.Type != lexer.RBRACE && p.curToken.Type != lexer.EOF {
		if p.curToken.Type == lexer.IDENTIFIER {
			fieldName := p.curToken.Literal
			fieldType := "САН" // default type is float/number

			// If next token is a type keyword, consume it as type
			if p.peekToken.Type == lexer.TYPE_INT ||
				p.peekToken.Type == lexer.TYPE_FLOAT ||
				p.peekToken.Type == lexer.TYPE_STRING ||
				p.peekToken.Type == lexer.TYPE_BOOL ||
				p.peekToken.Type == lexer.TYPE_BYTE ||
				p.peekToken.Type == lexer.IDENTIFIER {
				p.nextToken()
				fieldType = p.curToken.Literal
			}

			fields = append(fields, fieldName)
			types = append(types, fieldType)
		}
		p.nextToken()
	}
	// cur = }

	return &StructStatement{Name: name, Fields: fields, Types: types}
}

// ---------------------------------------------------------------------------
// Statement dispatch
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Function definition: функция атауы(x, y) { ... }
// ---------------------------------------------------------------------------

func (p *Parser) parseFunctionStatement() *FunctionStatement {
	// cur = функция
	p.nextToken() // cur = name

	if p.curToken.Type != lexer.IDENTIFIER {
		p.errorf("функция атауы күтілді, бірақ '%s' табылды", p.curToken.Literal)
		return nil
	}
	name := p.curToken.Literal
	p.nextToken() // cur = (

	if p.curToken.Type != lexer.LPAREN {
		p.errorf("'(' күтілді функция параметрлері алдында, бірақ '%s' табылды", p.curToken.Literal)
		return nil
	}
	p.nextToken() // cur = first param or )

	var params []string
	for p.curToken.Type != lexer.RPAREN && p.curToken.Type != lexer.EOF {
		if p.curToken.Type == lexer.IDENTIFIER {
			params = append(params, p.curToken.Literal)
		}
		p.nextToken()
		if p.curToken.Type == lexer.COMMA {
			p.nextToken()
		}
	}
	// cur = )
	p.nextToken() // cur = {

	if p.curToken.Type != lexer.LBRACE {
		p.errorf("'{' күтілді функция денесінде, бірақ '%s' табылды", p.curToken.Literal)
		return nil
	}
	body := p.parseBlockStatement()

	return &FunctionStatement{Name: name, Parameters: params, Body: body}
}

// ---------------------------------------------------------------------------
// Main expression parser (stack-based SOV)
// ---------------------------------------------------------------------------

func (p *Parser) parseExpression() Node {
	var stack []Node

	for p.curToken.Type != lexer.EOF &&
		p.curToken.Type != lexer.LBRACE &&
		p.curToken.Type != lexer.RBRACE {

		switch p.curToken.Type {

		// --- Literals ---
		case lexer.NUMBER:
			val, err := strconv.ParseFloat(p.curToken.Literal, 64)
			if err != nil {
				p.errorf("жарамсыз сан: '%s'", p.curToken.Literal)
				return nil
			}
			stack = append(stack, &NumberLiteral{Value: val})

		case lexer.INT_LITERAL:
			val, err := strconv.ParseInt(p.curToken.Literal, 10, 64)
			if err != nil {
				p.errorf("жарамсыз бүтін сан: '%s'", p.curToken.Literal)
				return nil
			}
			stack = append(stack, &IntLiteral{Value: val})

		case lexer.STRING:
			stack = append(stack, &StringLiteral{Value: p.curToken.Literal})

		case lexer.IDENTIFIER:
			// Peek ahead — if next is '(' this is a call expression
			if p.peekToken.Type == lexer.LPAREN {
				callExpr := p.parseCallExpression(p.curToken.Literal)
				if callExpr == nil {
					return nil
				}
				stack = append(stack, callExpr)
			} else {
				id := p.curToken.Literal
				if len(id) > 0 && id[0] == '.' {
					if len(stack) < 1 {
						p.errorf("өріске кіру үшін объект қажет")
						return nil
					}
					target := stack[len(stack)-1].(Expression)
					stack = stack[:len(stack)-1]
					stack = append(stack, &StructFieldAccessExpression{
						StructName: "",
						Field:      id[1:],
						Target:     target,
					})
				} else {
					isStructField := false
					for i := 0; i < len(id); i++ {
						if id[i] == '.' {
							stack = append(stack, &StructFieldAccessExpression{
								StructName: id[:i],
								Field:      id[i+1:],
							})
							isStructField = true
							break
						}
					}
					if !isStructField {
						stack = append(stack, &Identifier{Value: id})
					}
				}
			}

		case lexer.TRUE:
			stack = append(stack, &BoolLiteral{Value: true})

		case lexer.FALSE:
			stack = append(stack, &BoolLiteral{Value: false})

		// --- Array literal: [ ... ] ---
		case lexer.LBRACKET:
			arr := p.parseArrayLiteral()
			if arr == nil {
				return nil
			}
			stack = append(stack, arr)

		// --- Binary operators: need 2 operands ---
		case lexer.PLUS, lexer.MINUS, lexer.MUL, lexer.DIV,
			lexer.GT, lexer.LT, lexer.EQ, lexer.NEQ, lexer.GTE, lexer.LTE,
			lexer.AND, lexer.OR:
			if len(stack) < 2 {
				p.errorf("'%s' операторы 2 операнд талап етеді", p.curToken.Literal)
				return nil
			}
			right := stack[len(stack)-1].(Expression)
			left := stack[len(stack)-2].(Expression)
			stack = stack[:len(stack)-2]
			stack = append(stack, &PostfixExpression{Left: left, Right: right, Operator: p.curToken.Literal})

		// --- Unary NOT ---
		case lexer.NOT:
			if len(stack) < 1 {
				p.errorf("'емес' операторы 1 операнд талап етеді")
				return nil
			}
			val := stack[len(stack)-1].(Expression)
			stack = stack[:len(stack)-1]
			stack = append(stack, &UnaryExpression{Operator: "емес", Right: val})

		// --- Array length: arr ұзындық ---
		case lexer.ARRAY_LEN:
			if len(stack) < 1 {
				p.errorf("'ұзындық' 1 операнд талап етеді")
				return nil
			}
			val := stack[len(stack)-1].(Expression)
			stack = stack[:len(stack)-1]
			stack = append(stack, &LengthExpression{Value: val})

		// --- Char at: str idx символ ---
		case lexer.CHAR_AT:
			if len(stack) < 2 {
				p.errorf("'символ' 2 операнд талап етеді (мәтін, индекс)")
				return nil
			}
			idx := stack[len(stack)-1].(Expression)
			str := stack[len(stack)-2].(Expression)
			stack = stack[:len(stack)-2]
			stack = append(stack, &CharAtExpression{Str: str, Index: idx})

		// --- String concat: str1 str2 біріктіру ---
		case lexer.STR_CONCAT:
			if len(stack) < 2 {
				p.errorf("'біріктіру' 2 операнд талап етеді (мәтін1, мәтін2)")
				return nil
			}
			right := stack[len(stack)-1].(Expression)
			left := stack[len(stack)-2].(Expression)
			stack = stack[:len(stack)-2]
			stack = append(stack, &StrConcatExpression{Left: left, Right: right})

		// --- String length: str ұзындық_жол ---
		case lexer.STR_LEN:
			if len(stack) < 1 {
				p.errorf("'ұзындық_жол' 1 операнд талап етеді")
				return nil
			}
			val := stack[len(stack)-1].(Expression)
			stack = stack[:len(stack)-1]
			stack = append(stack, &StrLenExpression{Value: val})

		// --- String equals: str1 str2 мәтін_тең ---
		case lexer.STR_EQ:
			if len(stack) < 2 {
				p.errorf("'мәтін_тең' 2 операнд талап етеді")
				return nil
			}
			right := stack[len(stack)-1].(Expression)
			left := stack[len(stack)-2].(Expression)
			stack = stack[:len(stack)-2]
			stack = append(stack, &StrEqExpression{Left: left, Right: right})

		// --- Number to string: num санды_мәтін ---
		case lexer.TO_STR:
			if len(stack) < 1 {
				p.errorf("'санды_мәтін' 1 операнд талап етеді")
				return nil
			}
			val := stack[len(stack)-1].(Expression)
			stack = stack[:len(stack)-1]
			stack = append(stack, &ToStrExpression{Value: val})

		// --- Char code: str таңба_коды → byte value ---
		case lexer.CHAR_CODE:
			if len(stack) < 1 {
				p.errorf("'таңба_коды' 1 операнд талап етеді")
				return nil
			}
			val := stack[len(stack)-1].(Expression)
			stack = stack[:len(stack)-1]
			stack = append(stack, &CharCodeExpression{Value: val})

		// --- PRINT: <value> жазу ---
		case lexer.PRINT:
			if len(stack) < 1 {
				p.errorf("'жазу' 1 операнд талап етеді")
				return nil
			}
			val := stack[len(stack)-1].(Expression)
			stack = stack[:len(stack)-1]
			stmt := &PrintStatement{Value: val}
			stack = append(stack, stmt)
			return stack[0]

		// --- RETURN: қайтару <value> ---
		case lexer.RETURN:
			if len(stack) < 1 {
				p.errorf("'қайтару' 1 операнд талап етеді")
				return nil
			}
			val := stack[len(stack)-1].(Expression)
			stack = stack[:len(stack)-1]
			stmt := &ReturnStatement{Value: val}
			stack = append(stack, stmt)
			return stack[0]

		// --- CALL (шақыру) as statement: funcName(args) шақыру ---
		case lexer.CALL:
			if len(stack) < 1 {
				p.errorf("'шақыру' алдында функция шақыруы болуы керек")
				return nil
			}
			top := stack[len(stack)-1]
			callExpr, ok := top.(*CallExpression)
			if !ok {
				p.errorf("'шақыру' функция шақыруымен ғана жұмыс істейді")
				return nil
			}
			stack = stack[:len(stack)-1]
			stmt := &CallStatement{Call: callExpr}
			stack = append(stack, stmt)
			return stack[0]

		// --- FILE READ: "path" файл_оқу → FileReadExpression ---
		case lexer.FILE_READ:
			if len(stack) < 1 {
				p.errorf("'файл_оқу' 1 операнд талап етеді (жол атауы)")
				return nil
			}
			path := stack[len(stack)-1].(Expression)
			stack = stack[:len(stack)-1]
			stack = append(stack, &FileReadExpression{Path: path})

		// --- INPUT: кіру → InputExpression (reads a line from stdin) ---
		case lexer.INPUT:
			stack = append(stack, &InputExpression{})

		// --- FILE WRITE: path content файл_жазу → FileWriteStatement ---
		// SOV: "salam.txt" "Сәлем!" файл_жазу
		// stack[-2] = path, stack[-1] = content
		case lexer.FILE_WRITE:
			if len(stack) < 2 {
				p.errorf("'файл_жазу' 2 операнд талап етеді (жол, мазмұн)")
				return nil
			}
			content := stack[len(stack)-1].(Expression)
			path := stack[len(stack)-2].(Expression)
			stack = stack[:len(stack)-2]
			stmt := &FileWriteStatement{Path: path, Content: content}
			stack = append(stack, stmt)
			return stack[0]

		// --- IF: <cond> егер { ... } ---
		case lexer.IF:
			if len(stack) < 1 {
				p.errorf("'егер' шарты жоқ")
				return nil
			}
			cond := stack[len(stack)-1].(Expression)
			stack = stack[:len(stack)-1]

			stmt := &IfStatement{Condition: cond}
			p.nextToken()
			if p.curToken.Type != lexer.LBRACE {
				p.errorf("'егер' кейін '{' күтілді, бірақ '%s' табылды", p.curToken.Literal)
				return nil
			}
			stmt.Consequence = p.parseBlockStatement()

			if p.peekToken.Type == lexer.ELSE {
				p.nextToken()
				p.nextToken()
				if p.curToken.Type != lexer.LBRACE {
					p.errorf("'әйтпесе' кейін '{' күтілді, бірақ '%s' табылды", p.curToken.Literal)
					return nil
				}
				stmt.Alternative = p.parseBlockStatement()
			}

			stack = append(stack, stmt)
			return stack[0]

		// --- WHILE: <cond> әзірше { ... } ---
		case lexer.WHILE:
			if len(stack) < 1 {
				p.errorf("'әзірше' шарты жоқ")
				return nil
			}
			cond := stack[len(stack)-1].(Expression)
			stack = stack[:len(stack)-1]

			stmt := &WhileStatement{Condition: cond}
			p.nextToken()
			if p.curToken.Type != lexer.LBRACE {
				p.errorf("'әзірше' кейін '{' күтілді, бірақ '%s' табылды", p.curToken.Literal)
				return nil
			}
			stmt.Body = p.parseBlockStatement()

			stack = append(stack, stmt)
			return stack[0]

		// --- ILLEGAL token ---
		case lexer.ILLEGAL:
			p.errorf("жарамсыз таңба: '%s'", p.curToken.Literal)
			return nil

		// --- INDEX GET: arr idx тізім_алу ---
		case lexer.INDEX_GET:
			if len(stack) < 2 {
				p.errorf("'тізім_алу' 2 операнд талап етеді (тізім, индекс)")
				return nil
			}
			idx := stack[len(stack)-1].(Expression)
			arr := stack[len(stack)-2].(Expression)
			stack = stack[:len(stack)-2]
			stack = append(stack, &IndexExpression{Left: arr, Index: idx})

		// --- INDEX SET: arr idx val тізім_қой ---
		case lexer.INDEX_SET:
			if len(stack) < 3 {
				p.errorf("'тізім_қой' 3 операнд талап етеді (тізім, индекс, мән)")
				return nil
			}
			val := stack[len(stack)-1].(Expression)
			idxSet := stack[len(stack)-2].(Expression)
			arrNode, ok := stack[len(stack)-3].(*Identifier)
			if !ok {
				p.errorf("'тізім_қой' бірінші операнды идентификатор болуы керек")
				return nil
			}
			stack = stack[:len(stack)-3]
			stmt := &IndexAssignStatement{Array: arrNode, Index: idxSet, Value: val}
			stack = append(stack, stmt)
			return stack[0]

		// --- Struct Create: Адам жасау ---
		case lexer.NEW:
			if len(stack) < 1 {
				p.errorf("'жасау' 1 операнд талап етеді (құрылым атауы)")
				return nil
			}
			val := stack[len(stack)-1].(Expression)
			stack = stack[:len(stack)-1]

			id, ok := val.(*Identifier)
			if !ok {
				p.errorf("'жасау' алдында құрылым атауы болуы керек")
				return nil
			}
			stack = append(stack, &StructCreateExpression{StructName: id.Value})

		// --- Variable Assign: x 10 болсын, OR adam.name "Ali" болсын ---
		case lexer.VAR:
			if len(stack) < 2 {
				p.errorf("айнымалыға меншіктеу ('болсын') 2 операнд талап етеді: мән және айнымалы")
				return nil
			}
			val := stack[len(stack)-1].(Expression)    // 10
			idExpr := stack[len(stack)-2].(Expression) // x
			stack = stack[:len(stack)-2]

			idNode, ok := idExpr.(*Identifier)
			if !ok {
				// We also allow StructFieldAccessExpression to be assigned to, but since we parse
				// object.field as an Identifier initially, we need to check if it has a dot.
				// Wait, if it was parsed as StructFieldAccessExpression in parseSingleExpression,
				// we check that.
				if sfa, isSfa := idExpr.(*StructFieldAccessExpression); isSfa {
					stmt := &StructFieldAssignStatement{
						StructName: sfa.StructName,
						Field:      sfa.Field,
						Value:      val,
						Target:     sfa.Target,
					}
					stack = append(stack, stmt)
					return stack[0]
				}

				p.errorf("айнымалы атауы жарамсыз: %v", idExpr)
				return nil
			}

			stmt := &VarAssignStatement{Name: idNode, Value: val}
			stack = append(stack, stmt)
			return stack[0]

		// --- BREAK: үзу ---
		case lexer.BREAK:
			stmt := &BreakStatement{}
			stack = append(stack, stmt)
			return stack[0]

		// --- CONTINUE: жалғастыру ---
		case lexer.CONTINUE:
			stmt := &ContinueStatement{}
			stack = append(stack, stmt)
			return stack[0]

		// --- FREE: arr бос ---
		case lexer.FREE:
			if len(stack) < 1 {
				p.errorf("'бос' 1 операнд талап етеді")
				return nil
			}
			val := stack[len(stack)-1].(Expression)
			stack = stack[:len(stack)-1]
			stmt := &FreeStatement{Value: val}
			stack = append(stack, stmt)
			return stack[0]

		// --- IMPORT: "path" енгізу ---
		case lexer.IMPORT:
			if len(stack) < 1 {
				p.errorf("'енгізу' 1 операнд талап етеді (жол атауы)")
				return nil
			}
			pathNode, ok := stack[len(stack)-1].(*StringLiteral)
			if !ok {
				p.errorf("'енгізу' жол атауын талап етеді")
				return nil
			}
			stack = stack[:len(stack)-1]
			stmt := &ImportStatement{Path: pathNode.Value}
			stack = append(stack, stmt)
			return stack[0]
		}

		// болсын: assignment — triggered when VAR token literal is "болсын"
		if p.curToken.Type == lexer.VAR && p.curToken.Literal == "болсын" {
			if len(stack) < 2 {
				p.errorf("'болсын' идентификатор мен мән талап етеді")
				return nil
			}
			value := stack[len(stack)-1].(Expression)
			ident, ok := stack[len(stack)-2].(*Identifier)
			if !ok {
				p.errorf("'болсын' сол жағы идентификатор болуы керек")
				return nil
			}
			stack = stack[:len(stack)-2]
			stmt := &VarAssignStatement{Name: ident, Value: value}
			stack = append(stack, stmt)
			return stack[0]
		}

		// Check for early termination hints
		if p.peekToken.Type == lexer.LBRACE || p.peekToken.Type == lexer.EOF {
			break
		}

		p.nextToken()
	}

	if len(stack) > 0 {
		return stack[len(stack)-1]
	}
	return nil
}

// ---------------------------------------------------------------------------
// Call expression: funcName(arg1, arg2)
// ---------------------------------------------------------------------------

func (p *Parser) parseCallExpression(name string) *CallExpression {
	// cur = funcName, peek = (
	p.nextToken() // cur = (
	p.nextToken() // cur = first arg or )

	var args []Expression
	for p.curToken.Type != lexer.RPAREN && p.curToken.Type != lexer.EOF {
		expr := p.parseArgExpression()
		if expr != nil {
			args = append(args, expr)
		}
		if p.curToken.Type == lexer.COMMA {
			p.nextToken()
		}
	}
	// cur = )
	return &CallExpression{Function: name, Arguments: args}
}

// parseArgExpression parses one argument expression which may be a postfix
// (SOV) binary expression, e.g. "x y қосу" or just a literal/identifier.
func (p *Parser) parseArgExpression() Expression {
	var stack []Expression
	for p.curToken.Type != lexer.RPAREN &&
		p.curToken.Type != lexer.COMMA &&
		p.curToken.Type != lexer.RBRACKET &&
		p.curToken.Type != lexer.EOF {

		switch p.curToken.Type {
		case lexer.NUMBER:
			val, err := strconv.ParseFloat(p.curToken.Literal, 64)
			if err == nil {
				stack = append(stack, &NumberLiteral{Value: val})
			}
		case lexer.INT_LITERAL:
			val, err := strconv.ParseInt(p.curToken.Literal, 10, 64)
			if err == nil {
				stack = append(stack, &IntLiteral{Value: val})
			}
		case lexer.STRING:
			stack = append(stack, &StringLiteral{Value: p.curToken.Literal})
		case lexer.TRUE:
			stack = append(stack, &BoolLiteral{Value: true})
		case lexer.FALSE:
			stack = append(stack, &BoolLiteral{Value: false})
		case lexer.LBRACKET:
			arr := p.parseArrayLiteral()
			if arr != nil {
				stack = append(stack, arr)
			}
		case lexer.NEW:
			if len(stack) >= 1 {
				val := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				id, ok := val.(*Identifier)
				if ok {
					stack = append(stack, &StructCreateExpression{StructName: id.Value})
				}
			}
		case lexer.FILE_READ:
			if len(stack) >= 1 {
				path := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				stack = append(stack, &FileReadExpression{Path: path})
			}
		case lexer.INPUT:
			stack = append(stack, &InputExpression{})
		case lexer.IDENTIFIER:
			if p.peekToken.Type == lexer.LPAREN {
				callExpr := p.parseCallExpression(p.curToken.Literal)
				if callExpr != nil {
					stack = append(stack, callExpr)
					continue
				}
			} else {
				id := p.curToken.Literal
				if len(id) > 0 && id[0] == '.' {
					if len(stack) < 1 {
						p.errorf("өріске кіру үшін объект қажет")
						return nil
					}
					target := stack[len(stack)-1].(Expression)
					stack = stack[:len(stack)-1]
					stack = append(stack, &StructFieldAccessExpression{
						StructName: "",
						Field:      id[1:],
						Target:     target,
					})
				} else {
					isStructField := false
					for i := 0; i < len(id); i++ {
						if id[i] == '.' {
							stack = append(stack, &StructFieldAccessExpression{
								StructName: id[:i],
								Field:      id[i+1:],
							})
							isStructField = true
							break
						}
					}
					if !isStructField {
						stack = append(stack, &Identifier{Value: id})
					}
				}
			}
		case lexer.PLUS, lexer.MINUS, lexer.MUL, lexer.DIV,
			lexer.GT, lexer.LT, lexer.EQ, lexer.NEQ, lexer.GTE, lexer.LTE,
			lexer.AND, lexer.OR:
			if len(stack) >= 2 {
				right := stack[len(stack)-1]
				left := stack[len(stack)-2]
				stack = stack[:len(stack)-2]
				stack = append(stack, &PostfixExpression{Left: left, Right: right, Operator: p.curToken.Literal})
			}
		case lexer.NOT:
			if len(stack) >= 1 {
				val := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				stack = append(stack, &UnaryExpression{Operator: "емес", Right: val})
			}
		case lexer.STR_CONCAT:
			if len(stack) >= 2 {
				right := stack[len(stack)-1]
				left := stack[len(stack)-2]
				stack = stack[:len(stack)-2]
				stack = append(stack, &StrConcatExpression{Left: left, Right: right})
			}
		case lexer.STR_LEN:
			if len(stack) >= 1 {
				val := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				stack = append(stack, &StrLenExpression{Value: val})
			}
		case lexer.STR_EQ:
			if len(stack) >= 2 {
				right := stack[len(stack)-1]
				left := stack[len(stack)-2]
				stack = stack[:len(stack)-2]
				stack = append(stack, &StrEqExpression{Left: left, Right: right})
			}
		case lexer.TO_STR:
			if len(stack) >= 1 {
				val := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				stack = append(stack, &ToStrExpression{Value: val})
			}
		case lexer.CHAR_CODE:
			if len(stack) >= 1 {
				val := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				stack = append(stack, &CharCodeExpression{Value: val})
			}
		case lexer.CHAR_AT:
			if len(stack) >= 2 {
				idx := stack[len(stack)-1]
				str := stack[len(stack)-2]
				stack = stack[:len(stack)-2]
				stack = append(stack, &CharAtExpression{Str: str, Index: idx})
			}
		case lexer.ARRAY_LEN:
			if len(stack) >= 1 {
				val := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				stack = append(stack, &LengthExpression{Value: val})
			}
		case lexer.INDEX_GET:
			if len(stack) >= 2 {
				idx := stack[len(stack)-1]
				arr := stack[len(stack)-2]
				stack = stack[:len(stack)-2]
				stack = append(stack, &IndexExpression{Left: arr, Index: idx})
			}
		}
		p.nextToken()
	}

	if len(stack) > 0 {
		return stack[len(stack)-1]
	}
	return nil
}

// parseSingleExpression parses one atomic expression (literal or identifier).
func (p *Parser) parseSingleExpression() Expression {
	switch p.curToken.Type {
	case lexer.NUMBER:
		val, err := strconv.ParseFloat(p.curToken.Literal, 64)
		if err != nil {
			p.errorf("жарамсыз сан: '%s'", p.curToken.Literal)
			return nil
		}
		return &NumberLiteral{Value: val}
	case lexer.INT_LITERAL:
		val, err := strconv.ParseInt(p.curToken.Literal, 10, 64)
		if err != nil {
			p.errorf("жарамсыз бүтін сан: '%s'", p.curToken.Literal)
			return nil
		}
		return &IntLiteral{Value: val}
	case lexer.STRING:
		return &StringLiteral{Value: p.curToken.Literal}
	case lexer.IDENTIFIER:
		id := p.curToken.Literal
		// Check if it is a struct field access (e.g. adam.аты)
		importStrings := true // we'll just check manually to avoid adding imports if not present
		_ = importStrings
		for i := 0; i < len(id); i++ {
			if id[i] == '.' {
				return &StructFieldAccessExpression{
					StructName: id[:i],
					Field:      id[i+1:],
				}
			}
		}
		return &Identifier{Value: id}
	case lexer.TRUE:
		return &BoolLiteral{Value: true}
	case lexer.FALSE:
		return &BoolLiteral{Value: false}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Array literal: [ expr expr expr ]
// ---------------------------------------------------------------------------

func (p *Parser) parseArrayLiteral() *ArrayLiteral {
	// cur = [
	p.nextToken() // cur = first element or ]
	var elements []Expression
	for p.curToken.Type != lexer.RBRACKET && p.curToken.Type != lexer.EOF {
		expr := p.parseArgExpression()
		if expr != nil {
			elements = append(elements, expr)
		}
		if p.curToken.Type == lexer.COMMA {
			p.nextToken()
		}
	}
	// cur = ]
	return &ArrayLiteral{Elements: elements}
}

// ---------------------------------------------------------------------------
// Block: { stmt stmt ... }
// ---------------------------------------------------------------------------

func (p *Parser) parseBlockStatement() *BlockStatement {
	block := &BlockStatement{Statements: []Statement{}}
	p.nextToken() // skip {

	for p.curToken.Type != lexer.RBRACE && p.curToken.Type != lexer.EOF {
		// Handle nested function definitions inside blocks
		var stmt Statement
		if p.curToken.Type == lexer.FUNC {
			stmt = p.parseFunctionStatement()
		} else {
			stmt = p.parseStatement()
		}
		if stmt != nil {
			block.Statements = append(block.Statements, stmt)
		}
		p.nextToken()
	}
	return block
}
