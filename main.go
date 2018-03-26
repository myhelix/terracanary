package main

import (
	"log"
	"os"

	"github.com/myhelix/terracanary/cmd"
)

func main() {
	// Stderr is used for all human-readable output; stdout is reserved for data output
	log.SetOutput(os.Stderr)

	cmd.Execute()
}
