// Package cli implements the aemeath command line interface.
package cli

import (
	"context"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// version is set at build time: go build -ldflags "-X github.com/naufalraism/aemeath-cli/internal/cli.version=v0.1.0".
var version = "dev"

// resolveVersion prefers the version set at build time, then the module version
// recorded by "go install ...@v0.1.0", and falls back to "dev".
func resolveVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

// Execute runs the CLI and returns the process exit code.
func Execute() int {
	if err := newRootCmd().Execute(); err != nil {
		// cobra has already printed the error.
		return 1
	}
	return 0
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "aemeath",
		Short:         "The Aemeath framework CLI",
		Version:       resolveVersion(),
		SilenceErrors: false,
	}
	root.AddCommand(newNewCmd(), newDevCmd(), newTSCmd(), newGenerateCmd())
	return root
}

func newTSCmd() *cobra.Command {
	var pkg, out string

	cmd := &cobra.Command{
		Use:   "ts",
		Short: "Generate the TypeScript client, Zod schemas and React Query hooks",
		Long: `Builds your project and asks it to write client.ts, schemas.ts and hooks.ts
(generated from your Go routes) into the output directory.

The files are only rewritten when their content changed, so a bundler that
watches the directory does not reload for nothing.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			if err := buildAndGenerate(ctx, ".", pkg, out, cmd.OutOrStdout(), cmd.ErrOrStderr()); err != nil {
				return err
			}
			outf(cmd.OutOrStdout(), "📝 Wrote %s to %s\n", strings.Join(tsFiles, ", "), out)
			outln(cmd.OutOrStdout(), "👉 schemas.ts needs the 'zod' package and hooks.ts needs '@tanstack/react-query' (v5).")
			return nil
		},
	}
	cmd.Flags().StringVarP(&out, "out", "o", "api-client", "directory to write the TypeScript files to")
	cmd.Flags().StringVar(&pkg, "pkg", ".", "package to build and run")
	return cmd
}

func newDevCmd() *cobra.Command {
	var pkg, tsOut string
	var delay time.Duration

	cmd := &cobra.Command{
		Use:   "dev",
		Short: "Run the server and restart it when Go files change",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.SilenceUsage = true
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return runDev(ctx, devOptions{
				dir:      ".",
				pkg:      pkg,
				tsOut:    tsOut,
				debounce: delay,
				stdout:   cmd.OutOrStdout(),
				stderr:   cmd.ErrOrStderr(),
			})
		},
	}
	cmd.Flags().StringVar(&pkg, "pkg", ".", "package to build and run")
	cmd.Flags().StringVar(&tsOut, "ts-out", "", "also regenerate the TypeScript client, schemas and hooks in this directory after every successful build")
	cmd.Flags().DurationVar(&delay, "delay", 300*time.Millisecond, "wait this long after the last change before rebuilding")
	return cmd
}
