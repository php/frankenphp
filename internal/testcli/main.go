//go:build !windows

package main

import (
	"log"
	"os"

	"github.com/dunglas/frankenphp"
)

func main() {
	if len(os.Args) <= 1 {
		log.Println("Usage: testcli script.php")
		os.Exit(1)
	}

	if len(os.Args) == 3 && os.Args[1] == "-r" {
		os.Exit(frankenphp.ExecutePHPCode(os.Args[2]))
	}

	os.Exit(frankenphp.ExecuteScriptCLI(os.Args[1], os.Args))
}
