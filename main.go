package main

import (
	"embed"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"text/template"
	"unicode"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"
)

//go:embed templates/*
var templateFS embed.FS

func main() {
	var rootCmd = &cobra.Command{Use: "aemeath"}

	// Command: `aemeath new [nama-project]`
	var newCmd = &cobra.Command{
		Use:   "new [nama-project]",
		Short: "Membuat project Aemeath baru",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			projectName := args[0]
			err := os.Mkdir(projectName, 0755)
			if err != nil {
				fmt.Println("❌ Gagal membuat folder:", err)
				return
			}

			gomodData := struct{ ModuleName string }{ModuleName: projectName}
			renderTemplate("templates/go.mod.tmpl", filepath.Join(projectName, "go.mod"), gomodData)
			renderTemplate("templates/main.go.tmpl", filepath.Join(projectName, "main.go"), nil)

			fmt.Printf("✨ Project '%s' berhasil dibuat!\n", projectName)
			fmt.Printf("👉 Jalankan: cd %s && go mod tidy && aemeath dev\n", projectName)
		},
	}

	// Command: `aemeath dev`
	var devCmd = &cobra.Command{
		Use:   "dev",
		Short: "Menjalankan server dengan fitur Hot Reload",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("⚡ Memulai Aemeath Dev Server (Hot Reload)...")

			appProcess := runApp()

			// Tangkap sinyal Ctrl+C agar server mati dengan rapi
			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt, syscall.SIGTERM)
			go func() {
				<-c
				fmt.Println("\n👋 Mematikan Aemeath Dev Server...")
				if appProcess != nil && appProcess.Process != nil {
					if runtime.GOOS == "windows" {
						exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(appProcess.Process.Pid)).Run()
					} else {
						appProcess.Process.Kill()
					}
				}
				os.Exit(0)
			}()

			// Setup Watcher
			watcher, err := fsnotify.NewWatcher()
			if err != nil {
				fmt.Println("❌ Gagal membuat watcher:", err)
				return
			}
			defer watcher.Close()

			// Pantau folder saat ini dan sub-foldernya
			filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
				if info.IsDir() && info.Name() != ".git" && info.Name() != "node_modules" {
					watcher.Add(path)
				}
				return nil
			})

			// Loop untuk mendengarkan perubahan file
			for {
				select {
				case event, ok := <-watcher.Events:
					if !ok {
						return
					}
					if event.Op&fsnotify.Write == fsnotify.Write && filepath.Ext(event.Name) == ".go" {
						fmt.Printf("\n🔄 Perubahan terdeteksi di %s. Restarting...\n", event.Name)

						// Matikan seluruh proses (termasuk anaknya) sebelum restart
						if appProcess != nil && appProcess.Process != nil {
							if runtime.GOOS == "windows" {
								exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(appProcess.Process.Pid)).Run()
							} else {
								appProcess.Process.Kill()
							}
						}

						// Jalankan ulang dan simpan proses barunya
						appProcess = runApp()
					}
				case err, ok := <-watcher.Errors:
					if !ok {
						return
					}
					fmt.Println("❌ Watcher error:", err)
				}
			}
		},
	}

	// Command: `aemeath generate`
	var generateCmd = &cobra.Command{
		Use:   "generate",
		Short: "Generate berbagai komponen (api, model, middleware, docker, test, job)",
	}

	// Subcommand: `aemeath generate api <Nama>`
	var generateApiCmd = &cobra.Command{
		Use:   "api [Nama]",
		Short: "Generate file handler API baru",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := capitalize(args[0])
			lowerName := strings.ToLower(name)
			_ = os.Mkdir("handlers", 0755)
			destPath := filepath.Join("handlers", lowerName+".go")

			data := struct{ Name, LowerName string }{Name: name, LowerName: lowerName}
			renderTemplate("templates/handler.go.tmpl", destPath, data)

			fmt.Printf("✨ File handler untuk '%s' dibuat di %s\n", name, destPath)
		},
	}

	// Subcommand: `aemeath generate model <Nama>`
	var generateModelCmd = &cobra.Command{
		Use:   "model [Nama]",
		Short: "Generate file struct database (GORM)",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := capitalize(args[0])
			lowerName := strings.ToLower(name)
			_ = os.Mkdir("models", 0755)
			destPath := filepath.Join("models", lowerName+".go")

			data := struct{ Name string }{Name: name}
			renderTemplate("templates/model.go.tmpl", destPath, data)

			fmt.Printf("✨ File model untuk '%s' dibuat di %s\n", name, destPath)
		},
	}

	// Subcommand: `aemeath generate middleware <Nama>`
	var generateMiddlewareCmd = &cobra.Command{
		Use:   "middleware [Nama]",
		Short: "Generate file middleware baru",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := capitalize(args[0])
			lowerName := strings.ToLower(name)
			_ = os.Mkdir("middleware", 0755)
			destPath := filepath.Join("middleware", lowerName+".go")

			data := struct{ Name string }{Name: name}
			renderTemplate("templates/middleware.go.tmpl", destPath, data)

			fmt.Printf("✨ File middleware untuk '%s' dibuat di %s\n", name, destPath)
			fmt.Printf("👉 Jangan lupa panggil middleware.%sMiddleware() di main.go kamu!\n", name)
		},
	}

	// Subcommand: `aemeath generate docker`
	var generateDockerCmd = &cobra.Command{
		Use:   "docker",
		Short: "Generate Dockerfile dan docker-compose.yml",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			renderTemplate("templates/Dockerfile.tmpl", "Dockerfile", nil)
			fmt.Println("✨ File 'Dockerfile' berhasil dibuat!")

			renderTemplate("templates/docker-compose.yml.tmpl", "docker-compose.yml", nil)
			fmt.Println("✨ File 'docker-compose.yml' berhasil dibuat!")

			fmt.Println("👉 Jalankan: docker-compose up --build")
		},
	}

	// Subcommand: `aemeath generate test <Nama>`
	var generateTestCmd = &cobra.Command{
		Use:   "test [Nama]",
		Short: "Generate file unit test untuk handler",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := capitalize(args[0])
			lowerName := strings.ToLower(name)
			_ = os.Mkdir("handlers", 0755)
			destPath := filepath.Join("handlers", lowerName+"_test.go")

			data := struct{ Name, LowerName string }{Name: name, LowerName: lowerName}
			renderTemplate("templates/test.go.tmpl", destPath, data)

			fmt.Printf("✨ File test untuk '%s' dibuat di %s\n", name, destPath)
			fmt.Println("👉 Jalankan: go test ./...")
		},
	}

	// Subcommand: `aemeath generate job <Nama>`
	var generateJobCmd = &cobra.Command{
		Use:   "job [Nama]",
		Short: "Generate file background job baru",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := capitalize(args[0])
			lowerName := strings.ToLower(name)
			_ = os.Mkdir("jobs", 0755)
			destPath := filepath.Join("jobs", lowerName+".go")

			data := struct{ Name string }{Name: name}
			renderTemplate("templates/job.go.tmpl", destPath, data)

			fmt.Printf("✨ File job untuk '%s' dibuat di %s\n", name, destPath)
		},
	}

	generateCmd.AddCommand(generateApiCmd, generateModelCmd, generateMiddlewareCmd, generateDockerCmd, generateTestCmd, generateJobCmd)
	rootCmd.AddCommand(newCmd, devCmd, generateCmd)
	rootCmd.Execute()
}

// --- Fungsi Helper ---

func capitalize(s string) string {
	if len(s) == 0 {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])
	return string(r)
}

func renderTemplate(tmplPath, destPath string, data any) {
	content, err := templateFS.ReadFile(tmplPath)
	if err != nil {
		fmt.Println("❌ Gagal baca template:", err)
		return
	}

	tmpl, err := template.New("").Parse(string(content))
	if err != nil {
		fmt.Println("❌ Gagal parse template:", err)
		return
	}

	file, err := os.Create(destPath)
	if err != nil {
		fmt.Println("❌ Gagal buat file:", err)
		return
	}
	defer file.Close()

	err = tmpl.Execute(file, data)
	if err != nil {
		fmt.Println("❌ Gagal eksekusi template:", err)
	}
}

func runApp() *exec.Cmd {
	cmd := exec.Command("go", "run", "main.go")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Start()
	return cmd
}
