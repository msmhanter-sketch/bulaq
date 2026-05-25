package main

import (
	"fmt"
	"os"
	"strings"
)

func handleFmt(args []string) {
	if len(args) < 1 {
		fmt.Println("═══════════════════════════════════════════════════════════")
		fmt.Println("             Butaq Code Formatter (butaq-fmt)")
		fmt.Println("═══════════════════════════════════════════════════════════")
		fmt.Println("Қолдану: butaq fmt <файл.btq>")
		fmt.Println()
		return
	}

	fileName := args[0]
	content, err := os.ReadFile(fileName)
	if err != nil {
		fmt.Printf("❌ Қате: файлды оқу мүмкін болмады: %v\n", err)
		return
	}

	formatted := FormatButaqCode(string(content))
	err = os.WriteFile(fileName, []byte(formatted), 0644)
	if err != nil {
		fmt.Printf("❌ Қате: файлды жазу сәтсіз аяқталды: %v\n", err)
		return
	}

	fmt.Printf("✨ Файл сәтті форматталды: %s\n", fileName)
}

type FmtToken struct {
	Type    string // "WS", "NEWLINE", "COMMENT", "STRING", "PUNCT", "WORD"
	Literal string
}

func FormatButaqCode(input string) string {
	tokens := tokenizeForFmt(input)
	return formatTokens(tokens)
}

func tokenizeForFmt(input string) []FmtToken {
	var tokens []FmtToken
	runes := []rune(input)
	n := len(runes)
	i := 0

	for i < n {
		ch := runes[i]
		if ch == '\n' || ch == '\r' {
			literal := string(ch)
			if ch == '\r' && i+1 < n && runes[i+1] == '\n' {
				literal = "\r\n"
				i++
			}
			tokens = append(tokens, FmtToken{Type: "NEWLINE", Literal: literal})
			i++
			continue
		}
		if ch == ' ' || ch == '\t' {
			start := i
			for i < n && (runes[i] == ' ' || runes[i] == '\t') {
				i++
			}
			tokens = append(tokens, FmtToken{Type: "WS", Literal: string(runes[start:i])})
			continue
		}
		if ch == '#' {
			start := i
			for i < n && runes[i] != '\n' && runes[i] != '\r' {
				i++
			}
			tokens = append(tokens, FmtToken{Type: "COMMENT", Literal: string(runes[start:i])})
			continue
		}
		if ch == '"' {
			start := i
			i++ // skip first quote
			for i < n && runes[i] != '"' {
				if runes[i] == '\\' && i+1 < n {
					i += 2
				} else {
					i++
				}
			}
			if i < n {
				i++ // skip closing quote
			}
			tokens = append(tokens, FmtToken{Type: "STRING", Literal: string(runes[start:i])})
			continue
		}
		if isPunctOrOp(ch) {
			tokens = append(tokens, FmtToken{Type: "PUNCT", Literal: string(ch)})
			i++
			continue
		}
		
		start := i
		for i < n && runes[i] != ' ' && runes[i] != '\t' && runes[i] != '\n' && runes[i] != '\r' && runes[i] != '#' && runes[i] != '"' && !isPunctOrOp(runes[i]) {
			i++
		}
		tokens = append(tokens, FmtToken{Type: "WORD", Literal: string(runes[start:i])})
	}
	return tokens
}

func isPunctOrOp(ch rune) bool {
	return ch == '{' || ch == '}' || ch == '(' || ch == ')' || ch == '[' || ch == ']' || ch == ',' || ch == '.' || ch == '+' || ch == '-' || ch == '*' || ch == '/' || ch == '=' || ch == '<' || ch == '>' || ch == '!' || ch == ':'
}

func formatTokens(tokens []FmtToken) string {
	var sb strings.Builder
	indent := 0
	
	var lines [][]FmtToken
	var currentLine []FmtToken
	for _, tok := range tokens {
		if tok.Type == "NEWLINE" {
			lines = append(lines, currentLine)
			currentLine = nil
		} else {
			currentLine = append(currentLine, tok)
		}
	}
	if len(currentLine) > 0 {
		lines = append(lines, currentLine)
	}

	for _, line := range lines {
		var sigTokens []FmtToken
		for _, tok := range line {
			if tok.Type != "WS" {
				sigTokens = append(sigTokens, tok)
			}
		}

		if len(sigTokens) == 0 {
			sb.WriteString("\n")
			continue
		}

		startsWithClosingBrace := false
		for _, tok := range sigTokens {
			if tok.Type == "PUNCT" && tok.Literal == "}" {
				startsWithClosingBrace = true
				break
			}
			if tok.Type != "COMMENT" {
				break
			}
		}

		if startsWithClosingBrace {
			indent--
			if indent < 0 {
				indent = 0
			}
		}

		if sigTokens[0].Type != "COMMENT" || indent > 0 {
			sb.WriteString(strings.Repeat("    ", indent))
		}

		for idx, tok := range sigTokens {
			if idx > 0 {
				prev := sigTokens[idx-1]
				needSpace := true

				if prev.Literal == "." || tok.Literal == "." {
					needSpace = false
				}
				if prev.Literal == "(" || tok.Literal == ")" {
					needSpace = false
				}
				if prev.Literal == "[" || tok.Literal == "]" {
					needSpace = false
				}
				if tok.Literal == "," {
					needSpace = false
				}
				if tok.Type == "COMMENT" {
					needSpace = true
				}

				if needSpace {
					sb.WriteString(" ")
				}
			}
			sb.WriteString(tok.Literal)
		}
		sb.WriteString("\n")

		for _, tok := range sigTokens {
			if tok.Type == "PUNCT" && tok.Literal == "{" {
				indent++
			}
		}
	}

	result := sb.String()
	linesOut := strings.Split(result, "\n")
	var finalLines []string
	consecutiveEmpty := 0
	for _, l := range linesOut {
		trimmed := strings.TrimRight(l, " \t")
		if trimmed == "" {
			consecutiveEmpty++
			if consecutiveEmpty <= 1 {
				finalLines = append(finalLines, "")
			}
		} else {
			consecutiveEmpty = 0
			finalLines = append(finalLines, trimmed)
		}
	}
	
	if len(finalLines) > 0 && finalLines[len(finalLines)-1] == "" {
		finalLines = finalLines[:len(finalLines)-1]
	}

	return strings.Join(finalLines, "\n") + "\n"
}
