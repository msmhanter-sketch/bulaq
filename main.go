package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// compileBUtoGo translates Bulaq .bu code into native Go code
func compileBUtoGo(source string) string {
	out := "package main\nimport \"fmt\"\nfunc main() {\n"

	lines := strings.Split(source, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// жазу "Сәлем" немесе жазу x -> fmt.Println(...)
		if strings.HasPrefix(line, "жазу ") {
			content := strings.TrimPrefix(line, "жазу ")
			out += fmt.Sprintf("\tfmt.Println(%s)\n", content)
			continue
		}

		// айнымалы x = 5 -> var x = 5
		if strings.HasPrefix(line, "айнымалы ") {
			line = strings.Replace(line, "айнымалы ", "var ", 1)
		}

		// егер x > 5 { -> if x > 5 {
		line = strings.Replace(line, "егер ", "if ", -1)

		// әйтпесе -> else
		line = strings.Replace(line, "әйтпесе", "else", -1)

		// әзірше x < 5 { -> for x < 5 {
		line = strings.Replace(line, "әзірше ", "for ", -1)

		// цикл (while loop replacement for empty condition)
		line = strings.Replace(line, "цикл {", "for {", -1)

		// тоқтату -> break
		line = strings.Replace(line, "тоқтату", "break", -1)

		// жалғастыру -> continue
		line = strings.Replace(line, "жалғастыру", "continue", -1)

		out += "\t" + line + "\n"
	}

	out += "}\n"
	return out
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Қолдану: bulaq build <файл.bu>")
		os.Exit(1)
	}

	command := os.Args[1]
	inputFile := os.Args[2]

	if command != "build" {
		fmt.Println("Белгісіз бұйрық:", command)
		os.Exit(1)
	}

	if !strings.HasSuffix(inputFile, ".bu") {
		fmt.Println("Қате: Файл .bu кеңейтімімен аяқталуы керек")
		os.Exit(1)
	}

	sourceCode, err := os.ReadFile(inputFile)
	if err != nil {
		fmt.Println("Файлды оқу қатесі:", err)
		os.Exit(1)
	}

	goCode := compileBUtoGo(string(sourceCode))

	// Create a temporary directory for the Go compilation
	tmpDir, err := os.MkdirTemp("", "bulaq_build_*")
	if err != nil {
		fmt.Println("Уақытша папка құру қатесі:", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tmpDir) // Clean up temp files

	mainGoPath := filepath.Join(tmpDir, "main.go")
	err = os.WriteFile(mainGoPath, []byte(goCode), 0644)
	if err != nil {
		fmt.Println("Go файлын жазу қатесі:", err)
		os.Exit(1)
	}

	// Initialize go mod in the temp directory
	cmdInit := exec.Command("go", "mod", "init", "bulapp")
	cmdInit.Dir = tmpDir
	cmdInit.Run()

	// Compile it using the Go toolchain to create a native binary
	baseName := strings.TrimSuffix(filepath.Base(inputFile), ".bu")
	outputBinary := baseName

	cmdBuild := exec.Command("go", "build", "-o", outputBinary, "main.go")
	cmdBuild.Dir = tmpDir

	output, err := cmdBuild.CombinedOutput()
	if err != nil {
		fmt.Println("Компиляция қатесі:\n", string(output))
		fmt.Println("--- Бұл ішкі қате (Internal compiler error) ---")
		fmt.Println(goCode)
		os.Exit(1)
	}

	// Copy the compiled binary back to the current directory
	input, err := os.ReadFile(filepath.Join(tmpDir, outputBinary))
	if err == nil {
	    os.WriteFile(outputBinary, input, 0755)
	} else {
	    fmt.Println("Нәтижелік файлды көшіру қатесі:", err)
	    os.Exit(1)
	}

	fmt.Printf("Сәтті! Компиляцияланды: %s\n", outputBinary)
}
