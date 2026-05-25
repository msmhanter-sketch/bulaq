package parser

import (
	"butaq/lexer"
	"bytes"
	"fmt"
)

// ---------------------------------------------------------------------------
// Position tracking
// ---------------------------------------------------------------------------

type Pos struct {
	Line int
	Col  int
}

func (p *Pos) Position() (int, int) {
	return p.Line, p.Col
}

func (p *Pos) SetPosition(line, col int) {
	p.Line = line
	p.Col = col
}

// ---------------------------------------------------------------------------
// Node interfaces
// ---------------------------------------------------------------------------

type Node interface {
	TokenLiteral() string
	String() string
	Position() (int, int)
	SetPosition(line, col int)
}

type Statement interface {
	Node
	statementNode()
}

type Expression interface {
	Node
	expressionNode()
}

// ---------------------------------------------------------------------------
// Program / Block
// ---------------------------------------------------------------------------

type Program struct {
	Pos
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
	Pos
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

// ---------------------------------------------------------------------------
// Literals
// ---------------------------------------------------------------------------

type Identifier struct {
	Pos
	Value string
}

func (i *Identifier) expressionNode()      {}
func (i *Identifier) TokenLiteral() string { return i.Value }
func (i *Identifier) String() string       { return i.Value }

type NumberLiteral struct {
	Pos
	Value float64
}

func (nl *NumberLiteral) expressionNode()      {}
func (nl *NumberLiteral) TokenLiteral() string { return "number" }
func (nl *NumberLiteral) String() string       { return fmt.Sprintf("%v", nl.Value) }

type IntLiteral struct {
	Pos
	Value int64
}

func (il *IntLiteral) expressionNode()      {}
func (il *IntLiteral) TokenLiteral() string { return "int" }
func (il *IntLiteral) String() string       { return fmt.Sprintf("%d", il.Value) }

type StringLiteral struct {
	Pos
	Value string
}

func (sl *StringLiteral) expressionNode()      {}
func (sl *StringLiteral) TokenLiteral() string { return sl.Value }
func (sl *StringLiteral) String() string       { return `"` + sl.Value + `"` }

type BoolLiteral struct {
	Pos
	Value bool
}

func (bl *BoolLiteral) expressionNode()      {}
func (bl *BoolLiteral) TokenLiteral() string { return fmt.Sprintf("%v", bl.Value) }
func (bl *BoolLiteral) String() string       { return fmt.Sprintf("%v", bl.Value) }

// ---------------------------------------------------------------------------
// Expressions
// ---------------------------------------------------------------------------

// Postfix / infix binary operation: <left> <right> <operator>
type PostfixExpression struct {
	Pos
	Left     Expression
	Right    Expression
	Operator string
}

func (pe *PostfixExpression) expressionNode()      {}
func (pe *PostfixExpression) TokenLiteral() string { return pe.Operator }
func (pe *PostfixExpression) String() string {
	return "(" + pe.Left.String() + " " + pe.Right.String() + " " + pe.Operator + ")"
}

// Unary NOT: <expr> емес
type UnaryExpression struct {
	Pos
	Operator string
	Right    Expression
}

func (ue *UnaryExpression) expressionNode()      {}
func (ue *UnaryExpression) TokenLiteral() string { return ue.Operator }
func (ue *UnaryExpression) String() string       { return "(" + ue.Operator + " " + ue.Right.String() + ")" }

// Function call: funcName(arg1, arg2) шақыру
type CallExpression struct {
	Pos
	Function  string
	Arguments []Expression
}

func (ce *CallExpression) expressionNode()      {}
func (ce *CallExpression) TokenLiteral() string { return ce.Function }
func (ce *CallExpression) String() string {
	var out bytes.Buffer
	out.WriteString(ce.Function + "(")
	for i, a := range ce.Arguments {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString(a.String())
	}
	out.WriteString(")")
	return out.String()
}

// Struct definition: құрылым Адам { аты жасы }
type StructStatement struct {
	Pos
	Name   string
	Fields []string
	Types  []string
	IsWeak []bool
}

func (ss *StructStatement) statementNode()       {}
func (ss *StructStatement) TokenLiteral() string { return "құрылым" }
func (ss *StructStatement) String() string       { return "struct " + ss.Name }

// Thread Statement: ағын { ... } немесе ағын функция()
type ThreadStatement struct {
	Pos
	Token lexer.Token // THREAD
	Body  Statement   // BlockStatement or ExpressionStatement containing CallExpression
}

func (ts *ThreadStatement) statementNode()       {}
func (ts *ThreadStatement) expressionNode()      {}
func (ts *ThreadStatement) TokenLiteral() string { return ts.Token.Literal }
func (ts *ThreadStatement) String() string {
	return "thread " + ts.Body.String()
}

// Struct instantiation: Адам жасау
type StructCreateExpression struct {
	Pos
	StructName string
}

func (sc *StructCreateExpression) expressionNode()      {}
func (sc *StructCreateExpression) TokenLiteral() string { return "жасау" }
func (sc *StructCreateExpression) String() string       { return "new " + sc.StructName }

// Struct field access: adam.аты
type StructFieldAccessExpression struct {
	Pos
	StructName string
	Field      string
	Target     Expression
}

func (sa *StructFieldAccessExpression) expressionNode()      {}
func (sa *StructFieldAccessExpression) TokenLiteral() string { return "өріс_алу" }
func (sa *StructFieldAccessExpression) String() string       { return sa.StructName + "." + sa.Field }

// Array literal: тізім [1 2 3] болсын
type ArrayLiteral struct {
	Pos
	Elements []Expression
}

func (al *ArrayLiteral) expressionNode()      {}
func (al *ArrayLiteral) TokenLiteral() string { return "тізім" }
func (al *ArrayLiteral) String() string {
	var out bytes.Buffer
	out.WriteString("[")
	for i, e := range al.Elements {
		if i > 0 {
			out.WriteString(", ")
		}
		out.WriteString(e.String())
	}
	out.WriteString("]")
	return out.String()
}

// Index get: arr 0 алу
type IndexExpression struct {
	Pos
	Left  Expression
	Index Expression
}

func (ie *IndexExpression) expressionNode()      {}
func (ie *IndexExpression) TokenLiteral() string { return "алу" }
func (ie *IndexExpression) String() string {
	return ie.Left.String() + "[" + ie.Index.String() + "]"
}

// Array length: arr ұзындық
type LengthExpression struct {
	Pos
	Value Expression
}

func (le *LengthExpression) expressionNode()      {}
func (le *LengthExpression) TokenLiteral() string { return "ұзындық" }
func (le *LengthExpression) String() string       { return "len(" + le.Value.String() + ")" }

// Char at: str idx символ
type CharAtExpression struct {
	Pos
	Str   Expression
	Index Expression
}

func (ca *CharAtExpression) expressionNode()      {}
func (ca *CharAtExpression) TokenLiteral() string { return "символ" }
func (ca *CharAtExpression) String() string {
	return ca.Str.String() + "[" + ca.Index.String() + "]"
}

// String concat: str1 str2 біріктіру
type StrConcatExpression struct {
	Pos
	Left  Expression
	Right Expression
}

func (sc *StrConcatExpression) expressionNode()      {}
func (sc *StrConcatExpression) TokenLiteral() string { return "біріктіру" }
func (sc *StrConcatExpression) String() string       { return sc.Left.String() + " + " + sc.Right.String() }

// String length: str ұзындық_жол
type StrLenExpression struct {
	Pos
	Value Expression
}

func (sl *StrLenExpression) expressionNode()      {}
func (sl *StrLenExpression) TokenLiteral() string { return "ұзындық_жол" }
func (sl *StrLenExpression) String() string       { return "strlen(" + sl.Value.String() + ")" }

// String equals: str1 str2 мәтін_тең
type StrEqExpression struct {
	Pos
	Left  Expression
	Right Expression
}

func (se *StrEqExpression) expressionNode()      {}
func (se *StrEqExpression) TokenLiteral() string { return "мәтін_тең" }
func (se *StrEqExpression) String() string       { return se.Left.String() + " == " + se.Right.String() }

// Number to string: num санды_мәтін
type ToStrExpression struct {
	Pos
	Value Expression
}

func (ts *ToStrExpression) expressionNode()      {}
func (ts *ToStrExpression) TokenLiteral() string { return "санды_мәтін" }
func (ts *ToStrExpression) String() string       { return "tostr(" + ts.Value.String() + ")" }

// Char code: str таңба_коды → byte value of first byte as float
type CharCodeExpression struct {
	Pos
	Value Expression
}

func (cc *CharCodeExpression) expressionNode()      {}
func (cc *CharCodeExpression) TokenLiteral() string { return "таңба_коды" }
func (cc *CharCodeExpression) String() string       { return "charcode(" + cc.Value.String() + ")" }

// File read: "path" файл_оқу  → returns string with file contents
type FileReadExpression struct {
	Pos
	Path Expression
}

func (fr *FileReadExpression) expressionNode()      {}
func (fr *FileReadExpression) TokenLiteral() string { return "файл_оқу" }
func (fr *FileReadExpression) String() string       { return "file_read(" + fr.Path.String() + ")" }

// Input: кіру  → reads one line from stdin, returns string pointer
type InputExpression struct {
	Pos
}

func (ie *InputExpression) expressionNode()      {}
func (ie *InputExpression) TokenLiteral() string { return "кіру" }
func (ie *InputExpression) String() string       { return "кіру" }

// File write: "path" content файл_жазу  → statement (writes content to file)
type FileWriteStatement struct {
	Pos
	Path    Expression
	Content Expression
}

func (fw *FileWriteStatement) statementNode()       {}
func (fw *FileWriteStatement) TokenLiteral() string { return "файл_жазу" }
func (fw *FileWriteStatement) String() string {
	return fw.Path.String() + " " + fw.Content.String() + " файл_жазу"
}

// ---------------------------------------------------------------------------
// Statements
// ---------------------------------------------------------------------------

// x 10 болсын
type VarAssignStatement struct {
	Pos
	Name  *Identifier
	Value Expression
}

func (vs *VarAssignStatement) statementNode()       {}
func (vs *VarAssignStatement) TokenLiteral() string { return "болсын" }
func (vs *VarAssignStatement) String() string {
	return vs.Name.String() + " " + vs.Value.String() + " болсын"
}

// adam.аты "Али" болсын
type StructFieldAssignStatement struct {
	Pos
	StructName string
	Field      string
	Value      Expression
	Target     Expression
}

func (sf *StructFieldAssignStatement) statementNode()       {}
func (sf *StructFieldAssignStatement) TokenLiteral() string { return "болсын" }
func (sf *StructFieldAssignStatement) String() string {
	return sf.StructName + "." + sf.Field + " = " + sf.Value.String()
}

// arr 0 10 тізім_қой  (array[index] = value)
type IndexAssignStatement struct {
	Pos
	Array *Identifier
	Index Expression
	Value Expression
}

func (ia *IndexAssignStatement) statementNode()       {}
func (ia *IndexAssignStatement) TokenLiteral() string { return "тізім_қой" }
func (ia *IndexAssignStatement) String() string {
	return ia.Array.String() + "[" + ia.Index.String() + "] = " + ia.Value.String()
}

// arr бос  (free(arr))
type FreeStatement struct {
	Pos
	Value Expression
}

func (fs *FreeStatement) statementNode()       {}
func (fs *FreeStatement) TokenLiteral() string { return "бос" }
func (fs *FreeStatement) String() string       { return "free(" + fs.Value.String() + ")" }

// "Сәлем" жазу
type PrintStatement struct {
	Pos
	Value Expression
}

func (ps *PrintStatement) statementNode()       {}
func (ps *PrintStatement) TokenLiteral() string { return "жазу" }
func (ps *PrintStatement) String() string       { return ps.Value.String() + " жазу" }

// x 5 үлкен егер { ... } әйтпесе { ... }
type IfStatement struct {
	Pos
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

// z 5 кіші әзірше { ... }
type WhileStatement struct {
	Pos
	Condition Expression
	Body      *BlockStatement
}

func (ws *WhileStatement) statementNode()       {}
func (ws *WhileStatement) TokenLiteral() string { return "әзірше" }
func (ws *WhileStatement) String() string {
	return ws.Condition.String() + " әзірше { " + ws.Body.String() + " }"
}

// функция атауы(x, y) { ... }
type FunctionStatement struct {
	Pos
	Name       string
	Parameters []string
	Body       *BlockStatement
}

func (fs *FunctionStatement) statementNode()       {}
func (fs *FunctionStatement) TokenLiteral() string { return "функция" }
func (fs *FunctionStatement) String() string {
	return "функция " + fs.Name + "(...) { ... }"
}

// қайтару <expr>
type ReturnStatement struct {
	Pos
	Value Expression
}

func (rs *ReturnStatement) statementNode()       {}
func (rs *ReturnStatement) TokenLiteral() string { return "қайтару" }
func (rs *ReturnStatement) String() string       { return "қайтару " + rs.Value.String() }

// Standalone call statement: funcName(args) шақыру
type CallStatement struct {
	Pos
	Call *CallExpression
}

func (cs *CallStatement) statementNode()       {}
func (cs *CallStatement) TokenLiteral() string { return "шақыру" }
func (cs *CallStatement) String() string       { return cs.Call.String() + " шақыру" }

// General expression statement
type ExpressionStatement struct {
	Pos
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

// Break statement: үзу
type BreakStatement struct {
	Pos
}

func (bs *BreakStatement) statementNode()       {}
func (bs *BreakStatement) TokenLiteral() string { return "үзу" }
func (bs *BreakStatement) String() string       { return "үзу" }

// Continue statement: жалғастыру
type ContinueStatement struct {
	Pos
}

func (cs *ContinueStatement) statementNode()       {}
func (cs *ContinueStatement) TokenLiteral() string { return "жалғастыру" }
func (cs *ContinueStatement) String() string       { return "жалғастыру" }

// Import statement: "path" енгізу
type ImportStatement struct {
	Pos
	Path string
}

func (is *ImportStatement) statementNode()       {}
func (is *ImportStatement) TokenLiteral() string { return "енгізу" }
func (is *ImportStatement) String() string       { return "\"" + is.Path + "\" енгізу" }

// қате "хабарлама"
type ErrorLiteral struct {
	Pos
	Token lexer.Token
	Message Expression // сообщение об ошибке
}

func (el *ErrorLiteral) expressionNode()      {}
func (el *ErrorLiteral) TokenLiteral() string { return el.Token.Literal }
func (el *ErrorLiteral) String() string       { return "қате " + el.Message.String() }

// <өрнек> қатемен <айнымалы> { <блок> }
type TryErrorExpression struct {
	Pos
	Token   lexer.Token // токен "қатемен"
	Left    Expression  // выражение, возвращающее Result
	VarName string      // имя переменной для сообщения об ошибке
	Block   *BlockStatement
}

func (te *TryErrorExpression) expressionNode()      {}
func (te *TryErrorExpression) TokenLiteral() string { return te.Token.Literal }
func (te *TryErrorExpression) String() string {
	return te.Left.String() + " қатемен " + te.VarName + " " + te.Block.String()
}

type MethodSignature struct {
	Name       string
	Parameters []string
	ReturnType string
}

type InterfaceStatement struct {
	Pos
	Name    string
	Methods []MethodSignature
}

func (is *InterfaceStatement) statementNode()       {}
func (is *InterfaceStatement) TokenLiteral() string { return "интерфейс" }
func (is *InterfaceStatement) String() string       { return "interface " + is.Name }
