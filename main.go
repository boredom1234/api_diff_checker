package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"api_diff_checker/config"
	"api_diff_checker/core"
	"api_diff_checker/logger"
	myServer "api_diff_checker/server" // Will create this package next
	"api_diff_checker/storage"
)

func main() {
	webMode := flag.Bool("web", false, "Start web server mode")
	flag.Parse()

	// Initialize components common to both modes
	l, err := logger.New("execution.log", true)
	if err != nil {
		log.Fatalf("Failed to init logger: %v", err)
	}
	defer l.Close()

	store := storage.NewStore("responses")
	engine := core.NewEngine(store, l)

	if *webMode {
		// Web Mode
		fmt.Println("Starting Web Server on :9876...")
		if err := myServer.Start(engine); err != nil {
			log.Fatalf("Server failed: %v", err)
		}
	} else {
		// CLI Mode
		args := flag.Args()
		if len(args) < 1 {
			fmt.Println("Usage: api_diff_checker <config_file> OR api_diff_checker --web")
			os.Exit(1)
		}
		configPath := args[0]

		cfg, err := config.Load(configPath)
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}

		result, err := engine.Run(cfg)
		if err != nil {
			log.Fatalf("Execution failed: %v", err)
		}

		// Print Results to Console (CLI Output)
		printResults(result)
		fmt.Println("\nDone. Check 'responses/' for files and 'execution.log' for logs.")
	}
}

func printResults(result *core.RunResult) {
	for _, cmdRes := range result.CommandResults {
		// fmt.Printf("\nCommand: %s\n", cmdRes.Command)
		// Execution logs already printed by engine via specific fmt.Printf calls?
		// Actually engine does fmt.Printf for "Executing Command".
		// We should print diffs here.

		for _, diff := range cmdRes.Diffs {
			fmt.Printf("\n=== Diff between %s and %s ===\n", diff.VersionA, diff.VersionB)
			if diff.Error != "" {
				fmt.Printf("Error: %s\n", diff.Error)
				continue
			}

			if diff.DiffResult.Summary != "No top-level changes" {
				fmt.Println(diff.DiffResult.TextDiff)
				fmt.Printf("Summary: %s\n", diff.DiffResult.Summary)
				// fmt.Printf("JSON Patch:\n%s\n", string(diff.DiffResult.JsonPatch))
				// Keeping it slightly cleaner for CLI, or uncomment if needed
			} else {
				fmt.Println("No significant differences.")
			}
		}
	}
}
