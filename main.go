package main

import (
	"fmt"
	"os"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "check" {
		// TODO: implement check subcommand
		fmt.Println("nihao check: not yet implemented")
		os.Exit(0)
	}

	// Default: run setup
	fmt.Println("nihao 👋")
	fmt.Println("setting up your nostr identity...")
	// TODO: implement setup flow
}
