package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/aidanlsb/perch/internal/cli"
	"github.com/aidanlsb/perch/internal/run"
)

func main() {
	if err := run.Main(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if !errors.Is(err, cli.ErrReported) {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}
