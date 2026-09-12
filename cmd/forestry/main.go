package main

import (
	"fmt"
	"os"

	"github.com/Open-Source-Lodge/forestry/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "forestry: "+err.Error())
		os.Exit(1)
	}
}
