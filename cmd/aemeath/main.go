// Command aemeath is the command line interface of the Aemeath web framework.
//
//	go install github.com/naufalraism/aemeath-cli/cmd/aemeath@latest
package main

import (
	"os"

	"github.com/naufalraism/aemeath-cli/internal/cli"
)

func main() { os.Exit(cli.Execute()) }
