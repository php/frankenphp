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

	// "-init-first script.php" starts FrankenPHP before running the CLI script,
	// to exercise the guard against executing the PHP CLI while FrankenPHP is
	// already running (https://github.com/php/frankenphp/issues/2342).
	if len(os.Args) == 3 && os.Args[1] == "-init-first" {
		if err := frankenphp.Init(); err != nil {
			log.Fatalln(err)
		}

		status := frankenphp.ExecuteScriptCLI(os.Args[2], os.Args[2:])
		frankenphp.Shutdown()
		os.Exit(status)
	}

	os.Exit(frankenphp.ExecuteScriptCLI(os.Args[1], os.Args))
}
