package main

import (
	"fmt"
	"os"

	"github.com/node-real/op-coordinator/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "op-coordinator failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
