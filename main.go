package main

import (
	"fmt"

	"github.com/dragonflydb/piko/cli"
)

func main() {
	if err := cli.Start(); err != nil {
		fmt.Println(err)
	}
}
