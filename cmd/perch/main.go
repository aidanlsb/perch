package main

import (
	"fmt"
	"os"

	"github.com/aidanlsb/perch/internal/run"
)

func main() {
	if err := run.Main(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
