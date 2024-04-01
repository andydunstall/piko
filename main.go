// Copyright 2024 Andrew Dunstall. All rights reserved.
//
// Use of this source code is governed by a MIT style license that can be
// found in the LICENSE file.

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
