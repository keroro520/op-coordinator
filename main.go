package main

import (
	"fmt"
	"github.com/node-real/op-coordinator/cmd"
	"os"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "coordinator-cli failed %v\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
