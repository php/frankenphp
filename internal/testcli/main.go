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

	os.Exit(frankenphp.ExecuteScriptCLI(os.Args))
}
