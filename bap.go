package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func handleBap(args []string) {
	if len(args) < 1 {
		printBapUsage()
		return
	}

	command := args[0]
	switch command {
	case "init":
		bapInit()
	case "get":
		bapGet()
	case "build":
		bapBuild()
	case "run":
		bapRun()
	default:
		fmt.Printf("❌ Белгісіз bap командасы: %s\n", command)
		printBapUsage()
	}
}

func printBapUsage() {
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("             Bap — Bulaq Package Manager (Бетпақ)")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("Қолдану: butaq bap <команда> [аргументтер]")
	fmt.Println()
	fmt.Println("Командалар:")
	fmt.Println("  init      Жаңа Butaq жобасын бастау (bap.toml және main.btq жасау)")
	fmt.Println("  get       bap.toml файлдағы барлық тәуелділіктерді жүктеп алу")
	fmt.Println("  build     Жобаны жинақтау (main.btq компиляциясы)")
	fmt.Println("  run       Жобаны жинақтау және орындау")
	fmt.Println()
}

func bapInit() {
	dir, err := os.Getwd()
	if err != nil {
		fmt.Printf("❌ Қате: ағымдағы каталогты алу мүмкін болмады: %v\n", err)
		return
	}
	projName := filepath.Base(dir)

	tomlContent := fmt.Sprintf(`[package]
name = "%s"
version = "0.1.0"
description = "Butaq жобасы"

[dependencies]
# Мысалы:
# math_lib = "https://github.com/bulaq-lang/math_lib.git"
`, projName)

	if _, err := os.Stat("bap.toml"); err == nil {
		fmt.Println("⚠️  Ескерту: bap.toml файлы ағымдағы каталогта бұрыннан бар.")
	} else {
		err = os.WriteFile("bap.toml", []byte(tomlContent), 0644)
		if err != nil {
			fmt.Printf("❌ Қате: bap.toml жазу сәтсіз аяқталды: %v\n", err)
			return
		}
		fmt.Println("📝 bap.toml файлы сәтті жасалды!")
	}

	mainContent := `# Басты бағдарлама
"Сәлем, Butaq әлемі!" жазу
`

	if _, err := os.Stat("main.btq"); err == nil {
		fmt.Println("⚠️  Ескерту: main.btq файлы ағымдағы каталогта бұрыннан бар.")
	} else {
		err = os.WriteFile("main.btq", []byte(mainContent), 0644)
		if err != nil {
			fmt.Printf("❌ Қате: main.btq жазу сәтсіз аяқталды: %v\n", err)
			return
		}
		fmt.Println("🌱 main.btq файлы сәтті жасалды!")
	}

	fmt.Println("🎉 Жоба сәтті басталды! Іске қосу үшін: 'butaq bap run' теріңіз.")
}

type BapConfig struct {
	Name         string
	Version      string
	Dependencies map[string]string
}

func parseBapToml() (*BapConfig, error) {
	content, err := os.ReadFile("bap.toml")
	if err != nil {
		return nil, fmt.Errorf("bap.toml файлын оқу мүмкін болмады: %v", err)
	}

	config := &BapConfig{
		Dependencies: make(map[string]string),
	}

	lines := strings.Split(string(content), "\n")
	currentSection := ""

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentSection = strings.TrimSpace(line[1 : len(line)-1])
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		// Remove quotes
		if strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"") {
			val = val[1 : len(val)-1]
		}

		if currentSection == "package" {
			if key == "name" {
				config.Name = val
			} else if key == "version" {
				config.Version = val
			}
		} else if currentSection == "dependencies" {
			config.Dependencies[key] = val
		}
	}

	return config, nil
}

func bapGet() {
	config, err := parseBapToml()
	if err != nil {
		fmt.Printf("❌ Қате: %v\n", err)
		return
	}

	if len(config.Dependencies) == 0 {
		fmt.Println("ℹ️  Тәуелділіктер табылмады (dependencies секциясы бос).")
		return
	}

	err = os.MkdirAll("lib", 0755)
	if err != nil {
		fmt.Printf("❌ lib каталогын жасау сәтсіз: %v\n", err)
		return
	}

	for name, url := range config.Dependencies {
		fmt.Printf("📦 Тәуелділікті жүктеу: %s (%s)...\n", name, url)
		targetDir := filepath.Join("lib", name)

		// Clean old target if exists
		if _, err := os.Stat(targetDir); err == nil {
			fmt.Printf("🧹 Ескі '%s' каталогы тазартылуда...\n", targetDir)
			os.RemoveAll(targetDir)
		}

		// Run git clone
		cmd := exec.Command("git", "clone", url, targetDir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		if err != nil {
			fmt.Printf("❌ Тәуелділікті жүктеу кезінде қате орын алды: %v\n", err)
		} else {
			fmt.Printf("✅ %s сәтті орнатылды!\n", name)
		}
	}
}

func bapBuild() {
	if _, err := os.Stat("main.btq"); os.IsNotExist(err) {
		fmt.Println("❌ Қате: main.btq файлы табылмады. Алдымен 'butaq bap init' орындаңыз.")
		return
	}

	fmt.Println("🔨 Жоба жинақталуда...")
	// Run compiler on main.btq
	selfPath, err := os.Executable()
	if err != nil {
		selfPath = "butaq"
	}

	cmd := exec.Command(selfPath, "main.btq")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fmt.Printf("❌ Компиляция қатемен аяқталды: %v\n", err)
	} else {
		fmt.Println("✅ Жоба сәтті жинақталды!")
	}
}

func bapRun() {
	if _, err := os.Stat("main.btq"); os.IsNotExist(err) {
		fmt.Println("❌ Қате: main.btq файлы табылмады. Алдымен 'butaq bap init' орындаңыз.")
		return
	}

	fmt.Println("🚀 Жоба іске қосылуда...")
	selfPath, err := os.Executable()
	if err != nil {
		selfPath = "butaq"
	}

	cmd := exec.Command(selfPath, "-r", "main.btq")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		fmt.Printf("❌ Орындалу қатемен аяқталды: %v\n", err)
	}
}
