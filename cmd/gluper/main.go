// Command gluper upgrades self-managed GitLab safely.
package main

import (
	"os"

	"github.com/LaszloMocsy/gluper/internal/cli"
)

func main() {
	os.Exit(cli.Execute(os.Args[1:], os.Stdout, os.Stderr))
}
