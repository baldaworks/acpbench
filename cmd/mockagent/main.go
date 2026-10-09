package main

import (
	"os"

	"github.com/baldaworks/acpbench/internal/testmock"
)

func main() {
	if err := testmock.RunStdioMock(os.Stdin, os.Stdout); err != nil {
		os.Exit(1)
	}
}
