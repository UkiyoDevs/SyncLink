// main.go
package main

import (
	"log"
	"synclink/cmd"
)

func main() {
	// Optional: Configure log output format if needed
	log.SetFlags(log.LstdFlags | log.Lshortfile) // Example: Add file/line number to logs

	// Execute the root command from the cmd package
	cmd.Execute()
}
