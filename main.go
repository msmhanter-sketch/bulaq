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
	// CLI Flags
	var (
		helpFlag     bool
		runFlag      bool
		outputFlag   string
		platformFlag string
		asmFileFlag  string
		verboseFlag  bool
	)

	flag.BoolVar(&helpFlag, "h", false, "Көмек көрсету")
	flag.BoolVar(&helpFlag, "help", false, "Көмек көрсету")
	flag.BoolVar(&runFlag, "r", false, "Бағдарламаны компиляциядан кейін бірден іске қосу")
	flag.BoolVar(&runFlag, "run", false, "Бағдарламаны компиляциядан кейін бірден іске қосу")
	flag.StringVar(&outputFlag, "o", "", "Шығыс екілік (binary) файлдың атауы")
	flag.StringVar(&platformFlag, "platform", "", "Мақсатты платформа (windows немесе linux)")
	flag.StringVar(&asmFileFlag, "asm", "out.asm", "Генерацияланатын ассемблер файлының атауы")
	flag.BoolVar(&verboseFlag, "v", false, "Толық компиляция журналдарын көрсету")
	flag.BoolVar(&verboseFlag, "verbose", false, "Толық компиляция журналдарын көрсету")

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
		fmt.Println("  --asm <файл>      Ассемблер кодын сақтайтын файл (әдепкі: out.asm)")
		fmt.Println("  -v, --verbose     Толық компиляция барысын шығару")
		fmt.Println()
		fmt.Println("Мысалдар:")
		fmt.Println("  butaq examples/test_complex.btq")
		fmt.Println("  butaq -r examples/test_loop.btq")
		fmt.Println("  butaq -o myprog.exe examples/test_complex.btq")
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

	if len(p.Errors()) != 0 {
		fmt.Println("❌ Синтаксистік қателер:")
		for _, e := range p.Errors() {
			fmt.Printf("   → %s\n", e)
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
			fmt.Printf("   → %s\n", e)
		}
		os.Exit(1)
	}
	if verboseFlag {
		fmt.Println("✅ Тип тексеру: сәтті аяқталды (қателер жоқ)")
	}

	// ── 3. NASM x86-64 Assembly кодогенерация ──────────────────────────────
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

	cg := codegen.NewWithTC(tcEnv, tc, platform)
	asmCode := cg.Generate(program)
	if verboseFlag {
		fmt.Println("✅ Ассемблер коды сәтті генерацияланды")
	}

	// ── 4. Сақтау ─────────────────────────────────────────────
	asmFile := asmFileFlag
	if err := os.WriteFile(asmFile, []byte(asmCode), 0644); err != nil {
		fmt.Printf("❌ Қате: ассемблер файлын жазу мүмкін болмады: %v\n", err)
		os.Exit(1)
	}

	baseName := strings.TrimSuffix(filepath.Base(inputFile), ".btq")
	objFile := baseName + ".o"

	// ── 5. NASM арқылы объектілік файл жасау ────────────────────────────────
	if verboseFlag {
		fmt.Printf("🔧 NASM компиляциясы: %s → %s\n", filepath.Base(asmFile), filepath.Base(objFile))
	}

	var nasmArgs []string
	nasmExe := "nasm"
	if platform == codegen.PlatformWindows {
		nasmArgs = []string{"-f", "win64", "-o", objFile, asmFile}
		if _, err := exec.LookPath("nasm"); err != nil {
			if _, err := os.Stat("C:\\Program Files\\NASM\\nasm.exe"); err == nil {
				nasmExe = "C:\\Program Files\\NASM\\nasm.exe"
			}
		}
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
	if verboseFlag {
		fmt.Println("✅ NASM: объект файл жасалды")
	}

	// ── 6. Линковка ─────────────────────────────────────────────────────────
	outputBinary := outputFlag
	if outputBinary == "" {
		outputBinary = baseName
		if platform == codegen.PlatformWindows {
			outputBinary += ".exe"
		}
	}

	if verboseFlag {
		fmt.Printf("🔗 Линковка: %s → %s\n", filepath.Base(objFile), outputBinary)
	}

	var linkCmd *exec.Cmd
	if platform == codegen.PlatformWindows {
		linkCmd = exec.Command("gcc", "-o", outputBinary, objFile, "-lkernel32", "-lmsvcrt")
	} else {
		linkCmd = exec.Command("gcc", "-o", outputBinary, objFile, "-no-pie", "-lc")
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
	if asmFileFlag == "out.asm" {
		os.Remove("out.asm") // Delete default asm file
	}

	if verboseFlag {
		fmt.Println("🧹 Аралық объектілік файлдар тазартылды")
	}

	if !runFlag {
		fmt.Println()
		fmt.Println("═══════════════════════════════════════════════════════════")
		fmt.Printf("  ✅ Сәтті! Дербес бағдарлама жасалды: ./%s\n", outputBinary)
		fmt.Println("  (Таза x86-64 машина коды — C++ жоқ, Go жоқ!)")
		fmt.Println("═══════════════════════════════════════════════════════════")
	} else {
		// Run the program immediately
		var cmd *exec.Cmd
		if filepath.IsAbs(outputBinary) || strings.Contains(outputBinary, string(filepath.Separator)) {
			cmd = exec.Command(outputBinary)
		} else {
			cmd = exec.Command("." + string(filepath.Separator) + outputBinary)
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
