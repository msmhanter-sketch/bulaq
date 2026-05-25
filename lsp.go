package main

import (
	"bufio"
	"butaq/lexer"
	"butaq/parser"
	"butaq/typechecker"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type LspRequest struct {
	JsonRpc string          `json:"jsonrpc"`
	Id      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type LspResponse struct {
	JsonRpc string      `json:"jsonrpc"`
	Id      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

type LspNotification struct {
	JsonRpc string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

func handleLsp(args []string) {
	// Simple stdin/stdout LSP server loop
	reader := bufio.NewReader(os.Stdin)
	for {
		req, err := readLspRequest(reader)
		if err != nil {
			if err == io.EOF {
				break
			}
			continue
		}

		processLspRequest(req)
	}
}

func readLspRequest(reader *bufio.Reader) (*LspRequest, error) {
	var contentLength int

	// Read headers
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			break // End of headers
		}

		if strings.HasPrefix(line, "Content-Length:") {
			valStr := strings.TrimSpace(line[len("Content-Length:"):])
			length, err := strconv.Atoi(valStr)
			if err == nil {
				contentLength = length
			}
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("missing Content-Length header")
	}

	// Read body
	body := make([]byte, contentLength)
	_, err := io.ReadFull(reader, body)
	if err != nil {
		return nil, err
	}

	var req LspRequest
	err = json.Unmarshal(body, &req)
	if err != nil {
		return nil, err
	}

	return &req, nil
}

func sendLspResponse(id interface{}, result interface{}) {
	resp := LspResponse{
		JsonRpc: "2.0",
		Id:      id,
		Result:  result,
	}
	sendLspMessage(resp)
}

func sendLspNotification(method string, params interface{}) {
	notif := LspNotification{
		JsonRpc: "2.0",
		Method:  method,
		Params:  params,
	}
	sendLspMessage(notif)
}

func sendLspMessage(msg interface{}) {
	body, err := json.Marshal(msg)
	if err != nil {
		return
	}
	fmt.Printf("Content-Length: %d\r\n\r\n%s", len(body), string(body))
}

// In-memory document storage
var documents = make(map[string]string)

func processLspRequest(req *LspRequest) {
	var rawId interface{}
	if len(req.Id) > 0 {
		json.Unmarshal(req.Id, &rawId)
	}

	switch req.Method {
	case "initialize":
		result := map[string]interface{}{
			"capabilities": map[string]interface{}{
				"textDocumentSync": 1, // Full sync
				"completionProvider": map[string]interface{}{
					"resolveProvider": false,
					"triggerCharacters": []string{".", " "},
				},
				"hoverProvider": true,
			},
		}
		sendLspResponse(rawId, result)

	case "textDocument/didOpen":
		var params map[string]interface{}
		json.Unmarshal(req.Params, &params)
		if textDocument, ok := params["textDocument"].(map[string]interface{}); ok {
			if uri, ok := textDocument["uri"].(string); ok {
				if text, ok := textDocument["text"].(string); ok {
					documents[uri] = text
					triggerDiagnostics(uri, text)
				}
			}
		}

	case "textDocument/didChange":
		var params map[string]interface{}
		json.Unmarshal(req.Params, &params)
		if textDocument, ok := params["textDocument"].(map[string]interface{}); ok {
			if uri, ok := textDocument["uri"].(string); ok {
				if contentChanges, ok := params["contentChanges"].([]interface{}); ok && len(contentChanges) > 0 {
					if firstChange, ok := contentChanges[0].(map[string]interface{}); ok {
						if text, ok := firstChange["text"].(string); ok {
							documents[uri] = text
							triggerDiagnostics(uri, text)
						}
					}
				}
			}
		}

	case "textDocument/didSave":
		// Do nothing or re-trigger diagnostics
		var params map[string]interface{}
		json.Unmarshal(req.Params, &params)
		if textDocument, ok := params["textDocument"].(map[string]interface{}); ok {
			if uri, ok := textDocument["uri"].(string); ok {
				if text, ok := documents[uri]; ok {
					triggerDiagnostics(uri, text)
				}
			}
		}

	case "textDocument/hover":
		var params map[string]interface{}
		json.Unmarshal(req.Params, &params)
		var textDocumentUri string
		var line, character float64

		if textDocument, ok := params["textDocument"].(map[string]interface{}); ok {
			textDocumentUri, _ = textDocument["uri"].(string)
		}
		if position, ok := params["position"].(map[string]interface{}); ok {
			line, _ = position["line"].(float64)
			character, _ = position["character"].(float64)
		}

		hoverText := getHoverInfo(textDocumentUri, int(line), int(character))
		if hoverText != "" {
			sendLspResponse(rawId, map[string]interface{}{
				"contents": map[string]interface{}{
					"kind":  "markdown",
					"value": hoverText,
				},
			})
		} else {
			sendLspResponse(rawId, nil)
		}

	case "textDocument/completion":
		// Provide default keyword and built-in function completions
		var completions []map[string]interface{}
		lspKeywords := []string{
			"болсын", "егер", "әйтпесе", "әзірше", "үзу", "жалғастыру", "жазу",
			"қосу", "алу", "көбейту", "бөлу", "үлкен", "кіші", "тең", "тең_емес",
			"үлкен_тең", "кіші_тең", "және", "немесе", "емес", "жылжыту_сол", "жылжыту_оң",
			"функция", "қайтару", "шақыру", "ақиқат", "жалған", "тізім", "ұзындық",
			"тізім_алу", "тізім_қой", "бос", "БҮТІН", "САН", "МӘТІН", "АҚИҚАТ", "БАЙТ",
			"құрылым", "жасау", "әлсіз", "ағын", "символ", "біріктіру", "ұзындық_жол",
			"мәтін_тең", "санды_мәтін", "таңба_коды", "файл_оқу", "файл_жазу", "кіру", "енгізу",
		}
		for _, kw := range lspKeywords {
			completions = append(completions, map[string]interface{}{
				"label": kw,
				"kind":  14, // Keyword
			})
		}
		// Add stdlib builtins
		stdlibBuiltins := []string{
			"жсон_оқу", "жсон_жазу", "жсон_сан_алу", "жсон_мәтін_алу", "жсон_ақиқат_алу",
			"жсон_объект_алу", "жсон_тізім_алу", "жсон_тізім_өлшемі", "жсон_тізім_индекс_алу",
			"жсон_жасау", "жсон_сан_қосу", "жсон_мәтін_қосу", "жсон_ақиқат_қосу",
			"жсон_объект_қосу", "жсон_тізім_қосу", "мд5", "ша256", "бейс64_кодтау",
			"бейс64_декодтау", "ағын_күту",
		}
		for _, blt := range stdlibBuiltins {
			completions = append(completions, map[string]interface{}{
				"label": blt,
				"kind":  3, // Function
			})
		}
		sendLspResponse(rawId, completions)

	default:
		// Unknown request, return empty response to not block client
		if rawId != nil {
			sendLspResponse(rawId, nil)
		}
	}
}

func parseErrorPosition(msg string) (int, int, bool) {
	if !strings.HasPrefix(msg, "[жол ") {
		return 0, 0, false
	}
	idx := strings.Index(msg, "]")
	if idx == -1 {
		return 0, 0, false
	}
	posPart := msg[len("[жол "):idx] // e.g. "5, баған 12"
	parts := strings.Split(posPart, ", баған ")
	if len(parts) != 2 {
		return 0, 0, false
	}
	line, err1 := strconv.Atoi(parts[0])
	col, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return line - 1, col - 1, true
}

func triggerDiagnostics(uri string, content string) {
	var diagnostics []map[string]interface{}

	// Compile in memory
	l := lexer.New(content)
	p := parser.New(l)
	program := p.ParseProgram()

	// Parse errors
	for _, parseErr := range p.ErrorsStructured() {
		line := parseErr.Line - 1
		col := parseErr.Col - 1
		if line < 0 {
			line = 0
		}
		if col < 0 {
			col = 0
		}
		diagnostics = append(diagnostics, map[string]interface{}{
			"severity": 1, // Error
			"range": map[string]interface{}{
				"start": map[string]interface{}{"line": line, "character": col},
				"end":   map[string]interface{}{"line": line, "character": col + 1},
			},
			"message": parseErr.Message,
			"source":  "Butaq Parser",
		})
	}

	// Typechecker errors (if parser succeeded)
	if len(p.ErrorsStructured()) == 0 {
		tcEnv := typechecker.NewTypeEnv()
		tc := typechecker.New()
		tc.Check(program, tcEnv)

		for _, tcErr := range tc.Errors {
			line := tcErr.Line - 1
			col := tcErr.Col - 1
			if line < 0 {
				line = 0
			}
			if col < 0 {
				col = 0
			}
			diagnostics = append(diagnostics, map[string]interface{}{
				"severity": 1, // Error
				"range": map[string]interface{}{
					"start": map[string]interface{}{"line": line, "character": col},
					"end":   map[string]interface{}{"line": line, "character": col + 1},
				},
				"message": tcErr.Message,
				"source":  "Butaq Typechecker",
			})
		}

		// Run linter
		linter := typechecker.NewLinter()
		warnings := linter.Lint(program)
		for _, w := range warnings {
			line := w.Line - 1
			col := w.Col - 1
			if line < 0 {
				line = 0
			}
			if col < 0 {
				col = 0
			}
			diagnostics = append(diagnostics, map[string]interface{}{
				"severity": 2, // Warning
				"range": map[string]interface{}{
					"start": map[string]interface{}{"line": line, "character": col},
					"end":   map[string]interface{}{"line": line, "character": col + 1},
				},
				"message": w.Message,
				"source":  "Butaq Linter",
			})
		}
	}

	// Send diagnostics notification
	sendLspNotification("textDocument/publishDiagnostics", map[string]interface{}{
		"uri":         uri,
		"diagnostics": diagnostics,
	})
}

func getHoverInfo(uri string, line int, char int) string {
	content, ok := documents[uri]
	if !ok {
		return ""
	}

	// Basic hover token lookup
	lines := strings.Split(content, "\n")
	if line < 0 || line >= len(lines) {
		return ""
	}
	lText := lines[line]
	
	// Find word under cursor
	word := getWordAt(lText, char)
	if word == "" {
		return ""
	}

	// Return custom markdown documentation depending on the keyword/builtin hovered
	switch word {
	case "болсын":
		return "**болсын** (айнымалы жариялау/меншіктеу)\n\nЖаңа айнымалы құру немесе оған мән беру."
	case "егер":
		return "**егер** (шартты оператор)\n\nШарт орындалған жағдайда код блогын орындайды."
	case "әйтпесе":
		return "**әйтпесе** (балама шартты блок)\n\nЕгер шарт орындалмаса, осы блокты орындайды."
	case "әзірше":
		return "**әзірше** (цикл)\n\nШарт ақиқат болғанша код блогын қайталайды."
	case "жасу":
		return "**жазу** (мәліметтерді басып шығару)\n\nМән немесе мәтінді консольге шығарады."
	case "құрылым":
		return "**құрылым** (құрылым жариялау)\n\nЖаңа тип құру үшін пайдаланылады."
	case "әлсіз":
		return "**әлсіз** (weak reference)\n\nСілтемелер циклын болдырмау үшін әлсіз сілтеме жариялайды."
	case "ағын":
		return "**ағын** (thread spawn)\n\nЖаңа параллельді ағынды іске қосады."
	}

	return ""
}

func getWordAt(s string, idx int) string {
	if idx < 0 || idx >= len(s) {
		return ""
	}
	
	// Scan left
	start := idx
	for start > 0 && isWordChar(rune(s[start-1])) {
		start--
	}
	// Scan right
	end := idx
	for end < len(s) && isWordChar(rune(s[end])) {
		end++
	}
	
	return s[start:end]
}

func isWordChar(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' || (ch >= 'а' && ch <= 'я') || (ch >= 'А' && ch <= 'Я') || ch == 'ә' || ch == 'і' || ch == 'ң' || ch == 'ғ' || ch == 'ү' || ch == 'ұ' || ch == 'қ' || ch == 'ө' || ch == 'һ'
}
