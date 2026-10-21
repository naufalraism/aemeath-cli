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

			// FIX 1: Simpan proses pertama ke dalam variabel appProcess
			appProcess := runApp()

			// FIX 2: Tangkap sinyal Ctrl+C agar server mati dengan rapi
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
				if info.IsDir() && info.Name() != ".git" {
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

	// Command: `aemeath generate api [Nama]`
	var generateCmd = &cobra.Command{
		Use:   "generate api [Nama]",
		Short: "Generate file handler API baru",
		Args:  cobra.ExactArgs(2), // Butuh 2 argumen: "api" dan "Nama"
		Run: func(cmd *cobra.Command, args []string) {
			if args[0] != "api" {
				fmt.Println("❌ Hanya mendukung 'api' untuk saat ini. Contoh: aemeath generate api Product")
				return
			}

			name := args[1]
			// Kapitalisasi huruf pertama (misal: product -> Product)
			capitalizedName := strings.ToUpper(name[:1]) + name[1:]
			lowerName := strings.ToLower(capitalizedName)

			// Buat folder handlers jika belum ada
			os.Mkdir("handlers", 0755)

			// Siapkan data untuk template
			data := struct {
				Name      string
				LowerName string
			}{
				Name:      capitalizedName,
				LowerName: lowerName,
			}

			// Render template
			destPath := filepath.Join("handlers", lowerName+".go")
			renderTemplate("templates/handler.go.tmpl", destPath, data)

			fmt.Printf("✨ File handler untuk '%s' berhasil dibuat di %s\n", capitalizedName, destPath)
			fmt.Printf("👉 Jangan lupa panggil handlers.%sHandler(app) di main.go kamu!\n", capitalizedName)
		},
	}

	rootCmd.AddCommand(newCmd, devCmd, generateCmd)
	rootCmd.Execute()
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
