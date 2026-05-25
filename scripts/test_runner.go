package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("             Butaq Автоматты Тестілеу Жүйесі")
	fmt.Println("═══════════════════════════════════════════════════════════")

	// 1. Build the compiler first
	fmt.Println("🔧 Компиляторды жинақтау...")
	compilerBinary := "./butaq"
	if runtime.GOOS == "windows" {
		compilerBinary = ".\\butaq.exe"
	}

	buildCmd := exec.Command("go", "build", "-o", compilerBinary, ".")
	buildOut, err := buildCmd.CombinedOutput()
	if err != nil {
		fmt.Printf("❌ Компиляторды жинақтау қатесі:\n%s\n", string(buildOut))
		os.Exit(1)
	}
	fmt.Println("✅ Компилятор сәтті жинақталды!")

	// 2. Scan examples
	files, err := os.ReadDir("examples")
	if err != nil {
		fmt.Printf("❌ 'examples' каталогын оқу мүмкін болмады: %v\n", err)
		os.Exit(1)
	}

	passedCount := 0
	failedCount := 0

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".btq") {
			continue
		}

		// Skip helper/include files
		if strings.HasSuffix(file.Name(), "_helper.btq") {
			continue
		}

		filePath := filepath.Join("examples", file.Name())
		fmt.Printf("🏃 Тест басталуда: %s... ", file.Name())

		// Compile
		binaryName := strings.TrimSuffix(file.Name(), ".btq")
		if runtime.GOOS == "windows" {
			binaryName += ".exe"
		}

		compileCmd := exec.Command(compilerBinary, filePath)
		compileOut, err := compileCmd.CombinedOutput()
		if err != nil {
			fmt.Printf("❌ [КОМПИЛЯЦИЯ ҚАТЕСІ]\n%s\n", string(compileOut))
			failedCount++
			continue
		}

		// Run the generated binary
		runCmd := exec.Command("./" + binaryName)
		if runtime.GOOS == "windows" {
			runCmd = exec.Command(".\\" + binaryName)
		}
		runCmd.Stdin = strings.NewReader("Butaq\n")

		runOut, err := runCmd.CombinedOutput()
		if err != nil {
			if file.Name() == "test_safety.btq" {
				fmt.Println("🟢 [ӨТТІ] (Күтілген қауіпсіздік қатесі орындалды)")
				passedCount++
				os.Remove(binaryName)
				continue
			}
			fmt.Printf("❌ [Орындалу қатесі]\n%s\n", string(runOut))
			failedCount++
			// Clean up residue
			os.Remove(binaryName)
			continue
		}

		// Clean up generated binary
		os.Remove(binaryName)

		fmt.Println("🟢 [ӨТТІ]")
		passedCount++
	}

	// Clean up compiler binary
	os.Remove(compilerBinary)

	fmt.Println("\n═══════════════════════════════════════════════════════════")
	fmt.Printf("  Қорытынды: Барлығы: %d | Өтті: %d | Қате: %d\n", passedCount+failedCount, passedCount, failedCount)
	fmt.Println("═══════════════════════════════════════════════════════════")

	if failedCount > 0 {
		os.Exit(1)
	}
}
