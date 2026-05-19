package main

import (
	"butaq/codegen"
	"butaq/lexer"
	"butaq/parser"
	"butaq/typechecker"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("═══════════════════════════════════")
		fmt.Println("  Butaq тілі — SOV Компиляторы")
		fmt.Println("═══════════════════════════════════")
		fmt.Println("Қолдану: butaq <файл.btq>")
		fmt.Println()
		fmt.Println("Мысал: butaq math.btq")
		os.Exit(1)
	}

	inputFile := os.Args[1]
	sourceCode, err := os.ReadFile(inputFile)
	if err != nil {
		fmt.Printf("Қате: файлды оқу мүмкін болмады: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("🔤 Оқылды: %s (%d байт)\n", inputFile, len(sourceCode))

	// ── 1. Лексер ──────────────────────────────────────────────────────────
	l := lexer.New(string(sourceCode))
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.Errors()) != 0 {
		fmt.Println("❌ Синтаксистік қателер:")
		for _, e := range p.Errors() {
			fmt.Printf("   → %s\n", e)
		}
		os.Exit(1)
	}
	fmt.Printf("✅ Лексер + Парсер: %d оператор\n", len(program.Statements))

	// Resolve imports
	currentDir := filepath.Dir(inputFile)
	if err := parser.ResolveImports(program, currentDir); err != nil {
		fmt.Printf("❌ Енгізу қатесі: %v\n", err)
		os.Exit(1)
	}

	// ── 2. Статикалық типтер тексеру ───────────────────────────────────────
	tcEnv := typechecker.NewTypeEnv()
	tc := typechecker.New()
	tc.Check(program, tcEnv)

	if len(tc.Errors) != 0 {
		fmt.Println("❌ Тип қателері:")
		for _, e := range tc.Errors {
			fmt.Printf("   → %s\n", e)
		}
		os.Exit(1)
	}
	fmt.Println("✅ Тип тексеру: қате жоқ")

	// ── 3. NASM x86-64 Assembly кодогенерация ──────────────────────────────
	platform := codegen.PlatformLinux
	if runtime.GOOS == "windows" {
		platform = codegen.PlatformWindows
	}

	cg := codegen.NewWithTC(tcEnv, tc, platform)
	asmCode := cg.Generate(program)
	fmt.Println("✅ Ассемблер коды генерацияланды")

	// ── 4. Сақтау ─────────────────────────────────────────────
	asmFile := "out.asm"
	if err := os.WriteFile(asmFile, []byte(asmCode), 0644); err != nil {
		fmt.Println("Қате: asm файлын жазу мүмкін болмады:", err)
		os.Exit(1)
	}

	baseName := strings.TrimSuffix(filepath.Base(inputFile), ".btq")
	objFile := baseName + ".o"

	// ── 5. NASM арқылы объектілік файл жасау ────────────────────────────────
	fmt.Printf("🔧 NASM компиляциясы: %s → %s\n", filepath.Base(asmFile), filepath.Base(objFile))

	var nasmArgs []string
	nasmExe := "nasm"
	if runtime.GOOS == "windows" {
		nasmArgs = []string{"-f", "win64", "-o", objFile, asmFile}
		if _, err := exec.LookPath("nasm"); err != nil {
			if _, err := os.Stat("C:\\Program Files\\NASM\\nasm.exe"); err == nil {
				nasmExe = "C:\\Program Files\\NASM\\nasm.exe"
			}
		}
	} else if runtime.GOOS == "darwin" {
		nasmArgs = []string{"-f", "macho64", "-o", objFile, asmFile}
	} else {
		nasmArgs = []string{"-f", "elf64", "-o", objFile, asmFile}
	}

	nasmOut, err := exec.Command(nasmExe, nasmArgs...).CombinedOutput()
	if err != nil {
		fmt.Println("❌ NASM қатесі:")
		fmt.Println(string(nasmOut))
		fmt.Println("\n--- Генерацияланған ASM коды ---")
		printNumbered(asmCode)
		os.Exit(1)
	}
	fmt.Println("✅ NASM: объект файл жасалды")

	// ── 6. Линковка ─────────────────────────────────────────────────────────
	outputBinary := baseName
	if runtime.GOOS == "windows" {
		outputBinary += ".exe"
	}

	fmt.Printf("🔗 Линковка: %s → %s\n", filepath.Base(objFile), outputBinary)

	var linkCmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Windows: link with gcc (MinGW) or cl
		linkCmd = exec.Command("gcc", "-o", outputBinary, objFile, "-lkernel32", "-lmsvcrt")
	} else if runtime.GOOS == "darwin" {
		linkCmd = exec.Command("ld", "-o", outputBinary, objFile, "-lSystem", "-L/usr/lib")
	} else {
		// Linux: link with gcc to get libc (for printf)
		linkCmd = exec.Command("gcc", "-o", outputBinary, objFile, "-no-pie", "-lc")
	}

	linkOut, err := linkCmd.CombinedOutput()
	if err != nil {
		fmt.Println("❌ Линковка қатесі:")
		fmt.Println(string(linkOut))
		os.Exit(1)
	}

	os.Chmod(outputBinary, 0755)

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("  ✅ Сәтті! Дербес бағдарлама жасалды: ./%s\n", outputBinary)
	fmt.Println("  (Таза x86-64 машина коды — C++ жоқ, Go жоқ!)")
	fmt.Println("═══════════════════════════════════════════════════════════")
}

func printNumbered(code string) {
	lines := strings.Split(code, "\n")
	for i, line := range lines {
		fmt.Printf("%4d | %s\n", i+1, line)
	}
}
