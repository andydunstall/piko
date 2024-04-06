package main

import (
	"fmt"

	"github.com/andydunstall/pico/cli"
)

func main() {
	if err := cli.Start(); err != nil {
		fmt.Println(err)
	}
}
