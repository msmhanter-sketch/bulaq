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
	NUMBER
	STRING
	IDENTIFIER

	// Keywords
	VAR     // айнымалы
	IF      // егер
	ELSE    // әйтпесе
	WHILE   // әзірше
	PRINT   // жазу
	ASSIGN  // болсын (to assign, used at the end)

	// Math Operations (Words in Butaq)
	PLUS    // қосу
	MINUS   // алу
	MUL     // көбейту
	DIV     // бөлу

	// Logic
	GT      // үлкен
	LT      // кіші
	EQ      // тең

	// Blocks
	LBRACE  // {
	RBRACE  // }
)

type Token struct {
	Type    TokenType
	Literal string
	Line    int
	Col     int
}

var keywords = map[string]TokenType{
	"айнымалы": VAR,
	"егер":     IF,
	"әйтпесе":  ELSE,
	"әзірше":   WHILE,
	"жазу":     PRINT,
	"болсын":   ASSIGN,
	"қосу":     PLUS,
	"алу":      MINUS,
	"көбейту":  MUL,
	"бөлу":     DIV,
	"үлкен":    GT,
	"кіші":     LT,
	"тең":      EQ,
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
	case '"':
		tok.Type = STRING
		tok.Line = l.line
		tok.Col = l.col
		tok.Literal = l.readString()
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
			tok.Type = NUMBER
			tok.Line = l.line
			tok.Col = l.col
			tok.Literal = l.readNumber()
			return tok
		} else if isLetter(l.ch) {
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
	l.skipWhitespace()
}

func (l *Lexer) readIdentifier() string {
	startPos := l.position
	for isLetter(l.ch) || isDigit(l.ch) {
		l.readChar()
	}
	return l.input[startPos:l.position]
}

func (l *Lexer) readNumber() string {
	startPos := l.position
	for isDigit(l.ch) || l.ch == '.' {
		l.readChar()
	}
	return l.input[startPos:l.position]
}

func (l *Lexer) readString() string {
	startPos := l.position + 1
	for {
		l.readChar()
		if l.ch == '"' || l.ch == 0 {
			break
		}
	}
	return l.input[startPos:l.position]
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
	return fmt.Sprintf("Token(%d, %s)", t.Type, t.Literal)
}
