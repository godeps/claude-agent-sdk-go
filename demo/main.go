package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

var demos = map[string]func(){
	"mcp":        demoMCP,
	"typed":      demoTyped,
	"events":     demoEvents,
	"logging":    demoLogging,
	"retry":      demoRetry,
	"middleware":  demoMiddleware,
	"pool":       demoPool,
	"cost":       demoCost,
	"auth":       demoAuth,
	"session":    demoSession,
	"flags":      demoFlags,
}

func main() {
	demoFlag := flag.String("demo", "all", "Which demo to run: all|mcp|typed|events|logging|retry|middleware|pool|cost|auth|session|flags")
	flag.Parse()

	if *demoFlag == "all" {
		for name, fn := range demos {
			printHeader(name)
			fn()
			fmt.Println()
		}
		return
	}

	for _, name := range strings.Split(*demoFlag, ",") {
		name = strings.TrimSpace(name)
		fn, ok := demos[name]
		if !ok {
			fmt.Fprintf(os.Stderr, "unknown demo: %s\nAvailable: all, %s\n", name, availableDemos())
			os.Exit(1)
		}
		printHeader(name)
		fn()
		fmt.Println()
	}
}

func printHeader(name string) {
	fmt.Printf("=== Demo: %s ===\n", name)
}

func availableDemos() string {
	names := make([]string, 0, len(demos))
	for k := range demos {
		names = append(names, k)
	}
	return strings.Join(names, ", ")
}
