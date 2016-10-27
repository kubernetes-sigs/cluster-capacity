package main

import (
	"fmt"
	"os"

	"github.com/ingvagabund/cluster-capacity/cmd"
)

func main() {
	cmd := cmd.NewClusterCapacityCommand()
	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}
