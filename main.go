package main

import (
	"butaq/codegen"
	"butaq/lexer"
	"butaq/parser"
	"butaq/typechecker"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "bap":
			handleBap(os.Args[2:])
			return
		case "fmt":
			handleFmt(os.Args[2:])
			return
		case "lsp":
			handleLsp(os.Args[2:])
			return
		}
	}

	// CLI Flags
	var (
		helpFlag     bool
		runFlag      bool
		outputFlag   string
		platformFlag string
		asmFileFlag  string
		verboseFlag  bool
		llvmFlag     bool
	)

	flag.BoolVar(&helpFlag, "h", false, "Көмек көрсету")
	flag.BoolVar(&helpFlag, "help", false, "Көмек көрсету")
	flag.BoolVar(&runFlag, "r", false, "Бағдарламаны компиляциядан кейін бірден іске қосу")
	flag.BoolVar(&runFlag, "run", false, "Бағдарламаны компиляциядан кейін бірден іске қосу")
	flag.StringVar(&outputFlag, "o", "", "Шығыс екілік (binary) файлдың атауы")
	flag.StringVar(&platformFlag, "platform", "", "Мақсатты платформа (windows немесе linux)")
	flag.StringVar(&asmFileFlag, "asm", "", "Генерацияланатын ассемблер немесе LLVM файлының атауы")
	flag.BoolVar(&verboseFlag, "v", false, "Толық компиляция журналдарын көрсету")
	flag.BoolVar(&verboseFlag, "verbose", false, "Толық компиляция журналдарын көрсету")
	flag.BoolVar(&llvmFlag, "llvm", false, "LLVM IR генерациясын және clang компиляциясын қолдану")

	flag.Usage = func() {
		fmt.Println("═══════════════════════════════════════════════════════════")
		fmt.Println("             Butaq тілі — SOV Компиляторы (CLI)")
		fmt.Println("═══════════════════════════════════════════════════════════")
		fmt.Println("Қолдану: butaq [жалаушалар] <файл.btq>")
		fmt.Println()
		fmt.Println("Жалаушалар:")
		fmt.Println("  -h, --help        Осы анықтаманы көрсету")
		fmt.Println("  -r, --run         Компиляциядан кейін бағдарламаны іске қосу")
		fmt.Println("  -o <файл>         Шығыс файл атауы (әдепкі: кіріс файл аты)")
		fmt.Println("  --platform <тип>  Платформаны таңдау (windows немесе linux)")
		fmt.Println("  --asm <файл>      Кодты сақтайтын файл (әдепкі: out.asm немесе out.ll)")
		fmt.Println("  --llvm            LLVM IR backend-ін пайдалану (әдепкі: NASM)")
		fmt.Println("  -v, --verbose     Толық компиляция барысын шығару")
		fmt.Println()
		fmt.Println("Мысалдар:")
		fmt.Println("  butaq examples/test_complex.btq")
		fmt.Println("  butaq -r examples/test_loop.btq")
		fmt.Println("  butaq --llvm -r examples/test_loop.btq")
	}

	flag.Parse()

	if helpFlag || flag.NArg() < 1 {
		flag.Usage()
		os.Exit(0)
	}

	inputFile := flag.Arg(0)
	sourceCode, err := os.ReadFile(inputFile)
	if err != nil {
		fmt.Printf("❌ Қате: файлды оқу мүмкін болмады: %v\n", err)
		os.Exit(1)
	}

	if verboseFlag {
		fmt.Printf("🔤 Оқылды: %s (%d байт)\n", inputFile, len(sourceCode))
	}

	// ── 1. Лексер + Парсер ──────────────────────────────────────────────────
	l := lexer.New(string(sourceCode))
	p := parser.New(l)
	program := p.ParseProgram()

	if len(p.ErrorsStructured()) != 0 {
		fmt.Println("❌ Синтаксистік қателер:")
		for _, e := range p.ErrorsStructured() {
			renderError(inputFile, string(sourceCode), e.Line, e.Col, e.Message, "Синтаксис")
		}
		os.Exit(1)
	}
	if verboseFlag {
		fmt.Printf("✅ Лексер + Парсер: %d оператор талданды\n", len(program.Statements))
	}

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
			renderError(inputFile, string(sourceCode), e.Line, e.Col, e.Message, "Тип")
		}
		os.Exit(1)
	}
	if verboseFlag {
		fmt.Println("✅ Тип тексеру: сәтті аяқталды (қателер жоқ)")
	}

	// ── Linter ─────────────────────────────────────────────────────────────
	linter := typechecker.NewLinter()
	warnings := linter.Lint(program)
	if len(warnings) > 0 {
		fmt.Println("⚠️  Ескертулер:")
		for _, w := range warnings {
			renderWarning(inputFile, string(sourceCode), w.Line, w.Col, w.Message)
		}
	}

	// ── 3. Кодогенерация ──────────────────────────────
	platform := codegen.PlatformLinux
	if platformFlag != "" {
		switch strings.ToLower(platformFlag) {
		case "windows", "win":
			platform = codegen.PlatformWindows
		case "linux":
			platform = codegen.PlatformLinux
		default:
			fmt.Printf("⚠️ Белгісіз платформа '%s'. Ағымдағы ОЖ пайдаланылады.\n", platformFlag)
			if runtime.GOOS == "windows" {
				platform = codegen.PlatformWindows
			}
		}
	} else {
		if runtime.GOOS == "windows" {
			platform = codegen.PlatformWindows
		}
	}

	asmFile := asmFileFlag
	if asmFile == "" {
		if llvmFlag {
			asmFile = "out.ll"
		} else {
			asmFile = "out.asm"
		}
	}

	var generatedCode string
	if llvmFlag {
		lg := codegen.NewLlvm(tcEnv, tc, platform)
		generatedCode = lg.Generate(program)
		if verboseFlag {
			fmt.Println("✅ LLVM IR коды сәтті генерацияланды")
		}
	} else {
		cg := codegen.NewWithTC(tcEnv, tc, platform)
		generatedCode = cg.Generate(program)
		if verboseFlag {
			fmt.Println("✅ NASM ассемблер коды сәтті генерацияланды")
		}
	}

	// ── 4. Сақтау ─────────────────────────────────────────────
	if err := os.WriteFile(asmFile, []byte(generatedCode), 0644); err != nil {
		fmt.Printf("❌ Қате: шығыс файлын жазу мүмкін болмады: %v\n", err)
		os.Exit(1)
	}

	baseName := strings.TrimSuffix(filepath.Base(inputFile), ".btq")
	outputBinary := outputFlag
	if outputBinary == "" {
		outputBinary = baseName
		if platform == codegen.PlatformWindows {
			outputBinary += ".exe"
		}
	}

	// Find runtime.c relative to the compiler executable first, fallback to relative path
	runtimeC := "runtime/runtime.c"
	if exePath, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exePath), "runtime", "runtime.c")
		if _, err := os.Stat(candidate); err == nil {
			runtimeC = candidate
		}
	}

	if llvmFlag {
		// ── 5. LLVM IR компиляциясы және линковкасы (Clang арқылы) ──────────────
		if verboseFlag {
			fmt.Printf("🔧 Clang компиляциясы: %s + %s → %s\n", asmFile, runtimeC, outputBinary)
		}
		// Try clang
		clangArgs := []string{"-O3", "-o", outputBinary, asmFile, runtimeC}
		if platform == codegen.PlatformLinux {
			clangArgs = append(clangArgs, "-lpthread", "-lm")
		}
		clangCmd := exec.Command("clang", clangArgs...)
		clangOut, err := clangCmd.CombinedOutput()
		if err != nil {
			// If clang fails or not found, try to compile or notice user
			fmt.Println("⚠️ Clang компиляциясы сәтсіз аяқталды немесе 'clang' табылмады.")
			fmt.Println("LLVM IR коды келесі файлға сақталды:", asmFile)
			if verboseFlag {
				fmt.Println(string(clangOut))
			}
			if runFlag {
				os.Exit(1)
			}
		} else {
			if verboseFlag {
				fmt.Println("✅ Clang: сәтті орындалды")
			}
			os.Chmod(outputBinary, 0755)
			if asmFileFlag == "" {
				os.Remove("out.ll") // Delete default temporary LLVM IR file
			}
		}
	} else {
		// ── 5. NASM арқылы объектілік файл жасау ────────────────────────────────
		objFile := baseName + ".o"
		if verboseFlag {
			fmt.Printf("🔧 NASM компиляциясы: %s → %s\n", filepath.Base(asmFile), filepath.Base(objFile))
		}

		var nasmArgs []string
		nasmExe := "nasm"
		if runtime.GOOS == "windows" {
			nasmArgs = []string{"-f", "win64", "-Ox", "-o", objFile, asmFile}
			if _, err := exec.LookPath("nasm"); err != nil {
				if _, err := os.Stat("C:\\Program Files\\NASM\\nasm.exe"); err == nil {
					nasmExe = "C:\\Program Files\\NASM\\nasm.exe"
				}
			}
		} else {
			nasmArgs = []string{"-f", "elf64", "-Ox", "-o", objFile, asmFile}
		}

		nasmOut, err := exec.Command(nasmExe, nasmArgs...).CombinedOutput()
		if err != nil {
			fmt.Println("❌ NASM қатесі:")
			fmt.Println(string(nasmOut))
			fmt.Println("\n--- Генерацияланған ASM коды ---")
			printNumbered(generatedCode)
			os.Exit(1)
		}
		if verboseFlag {
			fmt.Println("✅ NASM: объект файл жасалды")
		}

		// ── 6. Линковка ─────────────────────────────────────────────────────────
		if verboseFlag {
			fmt.Printf("🔗 Линковка: %s → %s\n", filepath.Base(objFile), outputBinary)
		}

		var linkCmd *exec.Cmd
		if platform == codegen.PlatformWindows {
			linkCmd = exec.Command("gcc", "-O3", "-o", outputBinary, objFile, runtimeC, "-lkernel32", "-lmsvcrt")
		} else {
			linkCmd = exec.Command("gcc", "-O3", "-o", outputBinary, objFile, runtimeC, "-no-pie", "-lpthread", "-lc")
		}

		linkOut, err := linkCmd.CombinedOutput()
		if err != nil {
			fmt.Println("❌ Линковка қатесі:")
			fmt.Println(string(linkOut))
			os.Exit(1)
		}

		os.Chmod(outputBinary, 0755)

		// Clean up intermediate object file
		os.Remove(objFile)
		if asmFileFlag == "" {
			os.Remove("out.asm") // Delete default asm file
		}

		if verboseFlag {
			fmt.Println("🧹 Аралық объектілік файлдар тазартылды")
		}
	}

	if !runFlag {
		fmt.Println()
		fmt.Println("═══════════════════════════════════════════════════════════")
		fmt.Printf("  ✅ Сәтті! Дербес бағдарлама жасалды: ./%s\n", outputBinary)
		if llvmFlag {
			fmt.Println("  (Таза машина коды — LLVM компиляциясы арқылы!)")
		} else {
			fmt.Println("  (Таза x86-64 машина коды — C++ жоқ, Go жоқ!)")
		}
		fmt.Println("═══════════════════════════════════════════════════════════")
	} else {
		// Run the program immediately
		var cmd *exec.Cmd
		args := flag.Args()[1:]
		if filepath.IsAbs(outputBinary) || strings.Contains(outputBinary, string(filepath.Separator)) {
			cmd = exec.Command(outputBinary, args...)
		} else {
			cmd = exec.Command("." + string(filepath.Separator) + outputBinary, args...)
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		if verboseFlag {
			fmt.Printf("🚀 Бағдарлама іске қосылуда: %s\n\n", outputBinary)
		}
		runErr := cmd.Run()

		// Clean up binary if it was a temporary run
		if outputFlag == "" {
			os.Remove(outputBinary)
		}

		if runErr != nil {
			os.Exit(1)
		}
	}
}

func printNumbered(code string) {
	lines := strings.Split(code, "\n")
	for i, line := range lines {
		fmt.Printf("%4d | %s\n", i+1, line)
	}
}

func renderError(filename string, sourceCode string, line int, col int, message string, errorType string) {
	fmt.Printf("\n\033[1;31m%s қатесі (жол %d, баған %d):\033[0m %s\n", errorType, line, col, message)

	lines := strings.Split(sourceCode, "\n")
	if line > 0 && line <= len(lines) {
		if line > 1 {
			fmt.Printf(" \033[34m%4d |\033[0m %s\n", line-1, lines[line-2])
		}

		errorLine := lines[line-1]
		fmt.Printf(" \033[34m%4d |\033[0m %s\n", line, errorLine)

		caretLine := ""
		for i, char := range errorLine {
			if i >= col-1 {
				break
			}
			if char == '\t' {
				caretLine += "\t"
			} else {
				caretLine += " "
			}
		}
		fmt.Printf("      \033[34m|\033[0m %s\033[1;31m^\033[0m\n", caretLine)
	}

	hint := getHint(message)
	if hint != "" {
		fmt.Printf(" \033[1;36mКеңес/Подсказка:\033[0m %s\n", hint)
	}
	fmt.Println()
}

func getHint(msg string) string {
	if strings.Contains(msg, "айнымалысы жарияланбаған") || strings.Contains(msg, "undeclared var") || strings.Contains(msg, "undeclared variable") {
		return "Айнымалыны бірінші рет қолданбас бұрын оған мән меншіктеңіз (мысалы: айнымалы болсын мән) / Перед использованием переменной объявите её с помощью 'болсын' (например: x болсын 5)."
	}
	if strings.Contains(msg, "өзгертуге болмайды") {
		return "Айнымалының типін өзгертуге рұқсат етілмейді. Басқа жаңа айнымалыны қолданыңыз / Изменение типа переменной не допускается. Используйте новую переменную."
	}
	if strings.Contains(msg, "функциясы табылмады") {
		return "Функцияның атауын тексеріңіз немесе оны жариялаңыз / Проверьте имя функции или объявите её."
	}
	if strings.Contains(msg, "салыстыру операторы үшін типтер сәйкес болуы керек") {
		return "Әр түрлі типтегі мәндерді салыстыруға болмайды. Санды мәтінге немесе керісінше түрлендіріңіз / Нельзя сравнивать значения разных типов. Приведите их к одному типу."
	}
	if strings.Contains(msg, "егер шарты АҚИҚАТ болуы керек") {
		return "'егер' шарты АҚИҚАТ (bool) типіндегі мән болуы тиіс / Условие 'егер' должно возвращать логическое значение (АҚИҚАТ/ЖАЛҒАН)."
	}
	if strings.Contains(msg, "әзірше шарты АҚИҚАТ болуы керек") {
		return "'әзірше' шарты АҚИҚАТ (bool) типіндегі мән болуы тиіс / Условие 'әзірше' должно возвращать логическое значение."
	}
	if strings.Contains(msg, "күтілді") || strings.Contains(msg, "expected") {
		return "Синтаксисті тексеріңіз. Күтілген таңбаның дұрыс қойылғанына көз жеткізіңіз / Проверьте синтаксис. Убедитесь, что все скобки и ключевые слова расставлены верно."
	}
	if strings.Contains(msg, "шарты жоқ") {
		return "Шартты өрнекті көрсетіңіз / Укажите условное выражение."
	}
	return ""
}

func renderWarning(filename string, sourceCode string, line int, col int, message string) {
	fmt.Printf("\n\033[1;33mЕскерту (жол %d, баған %d):\033[0m %s\n", line, col, message)

	lines := strings.Split(sourceCode, "\n")
	if line > 0 && line <= len(lines) {
		if line > 1 {
			fmt.Printf(" \033[34m%4d |\033[0m %s\n", line-1, lines[line-2])
		}

		errorLine := lines[line-1]
		fmt.Printf(" \033[34m%4d |\033[0m %s\n", line, errorLine)

		caretLine := ""
		for i, char := range errorLine {
			if i >= col-1 {
				break
			}
			if char == '\t' {
				caretLine += "\t"
			} else {
				caretLine += " "
			}
		}
		fmt.Printf("      \033[34m|\033[0m %s\033[1;33m^\033[0m\n", caretLine)
	}
	fmt.Println()
}
