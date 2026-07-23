package main

import "fmt"

// Injected by GoReleaser / ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	fmt.Println("Test")
	fmt.Printf("patcher %s (commit=%s date=%s)\n", version, commit, date)
}
