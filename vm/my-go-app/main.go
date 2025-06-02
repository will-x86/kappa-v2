package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	fmt.Println("Hello from the Go application inside the NixOS VM!")
	fmt.Printf("VM booted at: %s\n", time.Now().Format(time.RFC3339))
	fmt.Printf("Arguments received: %v\n", os.Args)
	fmt.Println("Go application finished.")
	for {
		time.Sleep(1 * time.Second)
		fmt.Println("Zeheheha")
	}
}
