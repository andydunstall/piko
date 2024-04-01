// Copyright 2024 Andrew Dunstall. All rights reserved.
//
// Use of this source code is governed by a MIT style license that can be
// found in the LICENSE file.

package cli

func Start() error {
	cmd := NewCommand()
	return cmd.Execute()
}
