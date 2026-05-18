package codegen

import (
	"butaq/parser"
	"fmt"
	"strings"
)

type AsmGenerator struct {
	labelCount int
	strings    map[string]string // String value to label map
	variables  map[string]int    // Variable to BSS index
}

func NewAsm() *AsmGenerator {
	return &AsmGenerator{
		strings:   make(map[string]string),
		variables: make(map[string]int),
	}
}

func (cg *AsmGenerator) getLabel(prefix string) string {
	cg.labelCount++
	return fmt.Sprintf("%s_%d", prefix, cg.labelCount)
}

func (cg *AsmGenerator) Generate(program *parser.Program) string {
	var code strings.Builder
	var bss strings.Builder
	var data strings.Builder

	// Text section code
	code.WriteString("global _start\n")
	code.WriteString("section .text\n\n")

	// Print integer function
	code.WriteString("print_int:\n")
	code.WriteString("    push rbp\n")
	code.WriteString("    mov rbp, rsp\n")
	code.WriteString("    mov rax, rdi\n")
	code.WriteString("    mov rcx, 10\n")
	code.WriteString("    push 10\n") // newline
	code.WriteString("    mov rbx, 1\n") // count chars
	code.WriteString(".loop:\n")
	code.WriteString("    xor rdx, rdx\n")
	code.WriteString("    div rcx\n")
	code.WriteString("    add rdx, 48\n")
	code.WriteString("    push rdx\n")
	code.WriteString("    inc rbx\n")
	code.WriteString("    test rax, rax\n")
	code.WriteString("    jnz .loop\n")
	code.WriteString(".print_loop:\n")
	code.WriteString("    mov rax, 1\n") // sys_write
	code.WriteString("    mov rdi, 1\n") // stdout
	code.WriteString("    mov rsi, rsp\n")
	code.WriteString("    mov rdx, 1\n")
	code.WriteString("    syscall\n")
	code.WriteString("    add rsp, 8\n")
	code.WriteString("    dec rbx\n")
	code.WriteString("    jnz .print_loop\n")
	code.WriteString("    mov rsp, rbp\n")
	code.WriteString("    pop rbp\n")
	code.WriteString("    ret\n\n")

	// Main entry point
	code.WriteString("_start:\n")

	for _, stmt := range program.Statements {
		code.WriteString(cg.genStatement(stmt))
	}

	// Exit syscall
	code.WriteString("    mov rax, 60\n")
	code.WriteString("    xor rdi, rdi\n")
	code.WriteString("    syscall\n\n")

	// Setup BSS
	bss.WriteString("section .bss\n")
	if len(cg.variables) > 0 {
		bss.WriteString(fmt.Sprintf("    vars resq %d\n", len(cg.variables)))
	}

	// Setup Data
	data.WriteString("section .data\n")
	for val, label := range cg.strings {
		// Calculate string length including newline
		data.WriteString(fmt.Sprintf("    %s db `%s`, 10\n", label, val))
		data.WriteString(fmt.Sprintf("    %s_len equ $ - %s\n", label, label))
	}

	return data.String() + "\n" + bss.String() + "\n" + code.String()
}

func (cg *AsmGenerator) genStatement(node parser.Statement) string {
	switch n := node.(type) {
	case *parser.VarAssignStatement:
		valCode := cg.genExpression(n.Value)
		varIdx, exists := cg.variables[n.Name.Value]
		if !exists {
			varIdx = len(cg.variables)
			cg.variables[n.Name.Value] = varIdx
		}

		return valCode +
			"    pop rax\n" +
			fmt.Sprintf("    mov [vars + %d*8], rax\n", varIdx)

	case *parser.PrintStatement:
		valCode := cg.genExpression(n.Value)

		// If string, we know because it's a string literal currently, but with variables it gets tricky.
		// For our simple pure static language right now:
		if strLit, isStr := n.Value.(*parser.StringLiteral); isStr {
			label := cg.strings[strLit.Value]
			return fmt.Sprintf("    mov rax, 1\n    mov rdi, 1\n    mov rsi, %s\n    mov rdx, %s_len\n    syscall\n", label, label)
		}

		// Otherwise print as int
		return valCode +
			"    pop rdi\n" +
			"    call print_int\n"

	case *parser.IfStatement:
		condCode := cg.genExpression(n.Condition)

		endLabel := cg.getLabel(".if_end")
		altLabel := endLabel

		if n.Alternative != nil {
			altLabel = cg.getLabel(".if_alt")
		}

		res := condCode +
			"    pop rax\n" +
			"    test rax, rax\n" +
			"    jz " + altLabel + "\n"

		// Consequence
		for _, stmt := range n.Consequence.Statements {
			res += cg.genStatement(stmt)
		}

		if n.Alternative != nil {
			res += "    jmp " + endLabel + "\n"
			res += altLabel + ":\n"
			for _, stmt := range n.Alternative.Statements {
				res += cg.genStatement(stmt)
			}
		}

		res += endLabel + ":\n"
		return res

	case *parser.WhileStatement:
		startLabel := cg.getLabel(".while_start")
		endLabel := cg.getLabel(".while_end")

		res := startLabel + ":\n"
		res += cg.genExpression(n.Condition)
		res += "    pop rax\n" +
			"    test rax, rax\n" +
			"    jz " + endLabel + "\n"

		for _, stmt := range n.Body.Statements {
			res += cg.genStatement(stmt)
		}
		res += "    jmp " + startLabel + "\n"
		res += endLabel + ":\n"
		return res

	case *parser.ExpressionStatement:
		return cg.genExpression(n.Expression) + "    pop rax\n" // Discard result
	}

	return ""
}

func (cg *AsmGenerator) genExpression(node parser.Expression) string {
	switch n := node.(type) {
	case *parser.NumberLiteral:
		return fmt.Sprintf("    push %d\n", n.Value)
	case *parser.StringLiteral:
		label, exists := cg.strings[n.Value]
		if !exists {
			label = cg.getLabel("str")
			cg.strings[n.Value] = label
		}
		// Strings are mainly pushed for reference but for now we only support printing them directly.
		return "" // Not strictly supported as arbitrary values on stack yet.

	case *parser.Identifier:
		varIdx := cg.variables[n.Value]
		return fmt.Sprintf("    push qword [vars + %d*8]\n", varIdx)

	case *parser.PostfixExpression:
		left := cg.genExpression(n.Left)
		right := cg.genExpression(n.Right)

		res := left + right
		res += "    pop rbx\n"
		res += "    pop rax\n"

		switch n.Operator {
		case "қосу": // +
			res += "    add rax, rbx\n"
		case "алу": // -
			res += "    sub rax, rbx\n"
		case "көбейту": // *
			res += "    imul rax, rbx\n"
		case "бөлу": // /
			res += "    cqo\n"
			res += "    idiv rbx\n"
		case "үлкен": // >
			res += "    cmp rax, rbx\n"
			res += "    setg al\n"
			res += "    movzx rax, al\n"
		case "кіші": // <
			res += "    cmp rax, rbx\n"
			res += "    setl al\n"
			res += "    movzx rax, al\n"
		case "тең": // ==
			res += "    cmp rax, rbx\n"
			res += "    sete al\n"
			res += "    movzx rax, al\n"
		}

		res += "    push rax\n"
		return res
	}
	return ""
}
