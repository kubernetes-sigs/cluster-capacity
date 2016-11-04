package main

import (
	"fmt"
	"os"
	"path"

	capp "github.com/ingvagabund/cluster-capacity/cmd/cluster-capacity/app"
	gapp "github.com/ingvagabund/cluster-capacity/cmd/genpod/app"
)

func main() {
	switch path.Base(os.Args[0]) {
	case "cluster-capacity":
		cmd := capp.NewClusterCapacityCommand()
		if err := cmd.Execute(); err != nil {
			fmt.Println(err)
			os.Exit(-1)
		}
	case "genpod":
		cmd := gapp.NewGenPodCommand()
		if err := cmd.Execute(); err != nil {
			fmt.Println(err)
			os.Exit(-1)
		}
	}
}
