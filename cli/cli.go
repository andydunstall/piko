package cli

func Start() error {
	cmd := NewCommand()
	return cmd.Execute()
}
