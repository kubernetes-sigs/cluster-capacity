package main

import (
	"k8s.io/cluster-capacity/cmd"
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