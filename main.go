package main

import (
	"fmt"

	"github.com/andydunstall/piko/cli"
)

func main() {
	if err := cli.Start(); err != nil {
		fmt.Println(err)
	}
}
