package main

import (
	"fmt"
	"log"
	"os"
)

func main() {
	if err := run(); err != nil {
		log.Printf("Erro: %v", err)
		os.Exit(1)
	}
}

func run() error {
	tui, err := CreateTUI()
	if err != nil {
		return fmt.Errorf("iniciar interface: %w", err)
	}
	defer tui.DestroyTUI()

	return RunMenu(os.Stdin, os.Stdout, tui.Callbacks())
}
