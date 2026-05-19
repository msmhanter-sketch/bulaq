package lexer

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

type TokenType int

const (
	EOF TokenType = iota
	ILLEGAL

	// Data Types
	NUMBER      // float
	INT_LITERAL // int64
	STRING
	IDENTIFIER

	// Keywords — Butaq SOV syntax
	VAR      // болсын   (assign/declare)
	IF       // егер     (if)
	ELSE     // әйтпесе  (else)
	WHILE    // әзірше   (while)
	BREAK    // үзу
	CONTINUE // жалғастыру
	PRINT    // жазу     (print)

	// Math Operations
	PLUS  // қосу
	MINUS // алу
	MUL   // көбейту
	DIV   // бөлу

	// Comparisons
	GT  // үлкен    (>)
	LT  // кіші     (<)
	EQ  // тең      (==)
	NEQ // тең_емес (!=)
	GTE // үлкен_тең (>=)
	LTE // кіші_тең  (<=)

	// Logic
	AND // және
	OR  // немесе
	NOT // емес

	// Blocks
	LBRACE // {
	RBRACE // }
	LPAREN // (
	RPAREN // )
	COMMA  // ,

	// Functions
	FUNC   // функция
	RETURN // қайтару
	CALL   // шақыру
	IMPORT // енгізу

	// Booleans
	TRUE  // ақиқат
	FALSE // жалған

	// Arrays
	ARRAY     // тізім
	LBRACKET  // [
	RBRACKET  // ]
	INDEX_GET // тізім_алу (array index get)
	INDEX_SET // тізім_қой (array index set)
	ARRAY_LEN // ұзындық
	FREE      // бос (free heap memory)

	// Types
	TYPE_INT    // БҮТІН
	TYPE_FLOAT  // САН
	TYPE_STRING // МӘТІН
	TYPE_BOOL   // АҚИҚАТ
	TYPE_BYTE   // БАЙТ

	// Structs
	STRUCT // құрылым
	NEW    // жасау

	// String / char ops
	CHAR_AT    // символ
	STR_CONCAT // біріктіру (string concat)
	STR_LEN    // ұзындық_жол (string length)
	STR_EQ     // мәтін_тең (string equals compare)
	TO_STR     // санды_мәтін (number to string)
	CHAR_CODE  // таңба_коды (byte value of first char)

	// File I/O
	FILE_READ  // файл_оқу
	FILE_WRITE // файл_жазу

	// User Input
	INPUT // кіру
)

type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Col     int
}

var keywords = map[string]TokenType{
	// Core
	"болсын":     VAR,
	"егер":       IF,
	"әйтпесе":    ELSE,
	"әзірше":     WHILE,
	"үзу":        BREAK,
	"жалғастыру": CONTINUE,
	"жазу":       PRINT,

	// Math
	"қосу":    PLUS,
	"алу":     MINUS,
	"көбейту": MUL,
	"бөлу":    DIV,

	// Comparisons
	"үлкен":     GT,
	"кіші":      LT,
	"тең":       EQ,
	"тең_емес":  NEQ,
	"үлкен_тең": GTE,
	"кіші_тең":  LTE,

	// Logic
	"және":   AND,
	"немесе": OR,
	"емес":   NOT,

	// Functions
	"функция": FUNC,
	"қайтару": RETURN,
	"шақыру":  CALL,

	// Booleans
	"ақиқат": TRUE,
	"жалған": FALSE,

	// Arrays
	"тізім":     ARRAY,
	"ұзындық":   ARRAY_LEN,
	"тізім_алу": INDEX_GET,
	"тізім_қой": INDEX_SET,
	"бос":       FREE,

	// Types
	"БҮТІН":  TYPE_INT,
	"САН":    TYPE_FLOAT,
	"МӘТІН":  TYPE_STRING,
	"АҚИҚАТ": TYPE_BOOL,
	"БАЙТ":   TYPE_BYTE,

	// Structs
	"құрылым": STRUCT,
	"жасау":   NEW,

	// String / char ops
	"символ":      CHAR_AT,
	"біріктіру":   STR_CONCAT,
	"ұзындық_жол": STR_LEN,
	"мәтін_тең":   STR_EQ,
	"санды_мәтін": TO_STR,
	"таңба_коды":  CHAR_CODE,

	// File I/O
	"файл_оқу":  FILE_READ,
	"файл_жазу": FILE_WRITE,

	// User Input
	"кіру": INPUT,

	// Imports
	"енгізу": IMPORT,
}

type Lexer struct {
	input        string
	position     int
	readPosition int
	ch           rune
	line         int
	col          int
}

func New(input string) *Lexer {
	l := &Lexer{input: input, line: 1, col: 0}
	l.readChar()
	return l
}

func (l *Lexer) readChar() {
	if l.readPosition >= len(l.input) {
		l.ch = 0
	} else {
		r, size := utf8.DecodeRuneInString(l.input[l.readPosition:])
		l.ch = r
		l.position = l.readPosition
		l.readPosition += size
		l.col++
	}
}

func (l *Lexer) NextToken() Token {
	var tok Token

	l.skipWhitespace()

	switch l.ch {
	case '{':
		tok = l.newToken(LBRACE, string(l.ch))
	case '}':
		tok = l.newToken(RBRACE, string(l.ch))
	case '(':
		tok = l.newToken(LPAREN, string(l.ch))
	case ')':
		tok = l.newToken(RPAREN, string(l.ch))
	case '[':
		tok = l.newToken(LBRACKET, string(l.ch))
	case ']':
		tok = l.newToken(RBRACKET, string(l.ch))
	case ',':
		tok = l.newToken(COMMA, string(l.ch))
	case '"':
		tok.Type = STRING
		tok.Line = l.line
		tok.Col = l.col
		str, ok := l.readString()
		if !ok {
			tok.Type = ILLEGAL
			tok.Literal = "незакрытые кавычки"
			return tok
		}
		tok.Literal = str
	case '#':
		l.skipComment()
		return l.NextToken()
	case 0:
		tok.Literal = ""
		tok.Type = EOF
		tok.Line = l.line
		tok.Col = l.col
	default:
		if isDigit(l.ch) {
			tok.Line = l.line
			tok.Col = l.col
			num, ok, isFloat := l.readNumber()
			if !ok {
				tok.Type = ILLEGAL
				tok.Literal = num
				return tok
			}
			if isFloat {
				tok.Type = NUMBER
			} else {
				tok.Type = INT_LITERAL
			}
			tok.Literal = num
			return tok
		} else if isLetter(l.ch) || l.ch == '.' {
			tok.Line = l.line
			tok.Col = l.col
			tok.Literal = l.readIdentifier()
			if kw, ok := keywords[tok.Literal]; ok {
				tok.Type = kw
			} else {
				tok.Type = IDENTIFIER
			}
			return tok
		} else {
			tok = l.newToken(ILLEGAL, string(l.ch))
		}
	}

	l.readChar()
	return tok
}

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		if l.ch == '\n' {
			l.line++
			l.col = 0
		}
		l.readChar()
	}
}

func (l *Lexer) skipComment() {
	for l.ch != '\n' && l.ch != 0 {
		l.readChar()
	}
}

func (l *Lexer) readIdentifier() string {
	startPos := l.position
	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' || l.ch == '.' {
		l.readChar()
	}
	return l.input[startPos:l.position]
}

// readNumber returns the number literal, a boolean indicating validity, and a boolean indicating if it's a float.
func (l *Lexer) readNumber() (string, bool, bool) {
	startPos := l.position
	dotCount := 0
	for isDigit(l.ch) || l.ch == '.' {
		if l.ch == '.' {
			dotCount++
			if dotCount > 1 {
				// consume rest and return invalid
				for isDigit(l.ch) || l.ch == '.' {
					l.readChar()
				}
				return l.input[startPos:l.position], false, false
			}
		}
		l.readChar()
	}
	return l.input[startPos:l.position], true, dotCount == 1
}

// readString returns the string contents (without quotes) and a bool indicating if it was properly closed.
func (l *Lexer) readString() (string, bool) {
	startPos := l.position + 1
	for {
		l.readChar()
		if l.ch == '"' {
			break
		}
		if l.ch == 0 {
			// EOF without closing quote
			return "", false
		}
	}
	return l.input[startPos:l.position], true
}

func isLetter(ch rune) bool {
	return unicode.IsLetter(ch) || ch == '_'
}

func isDigit(ch rune) bool {
	return '0' <= ch && ch <= '9'
}

func (l *Lexer) newToken(tokenType TokenType, ch string) Token {
	return Token{Type: tokenType, Literal: ch, Line: l.line, Col: l.col}
}

func (t Token) String() string {
	return fmt.Sprintf("Token(%d, %q, line %d, col %d)", t.Type, t.Literal, t.Line, t.Col)
}
