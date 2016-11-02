package main

import (
	"fmt"
	"os"
)

func main() {
	cmd := NewGenPodCommand()
	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}
