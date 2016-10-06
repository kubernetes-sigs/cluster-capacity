package main

import (
	"github.com/ingvagabund/cluster-capacity/cmd"
	"fmt"
	"os"
)

func main() {
	cmd := cmd.NewClusterCapacityCommand()
	if err := cmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
}