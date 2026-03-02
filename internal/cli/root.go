package cli

import "github.com/spf13/cobra"

func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "codex-gateway",
		Short:         "Codex OAuth to OpenAI-compatible gateway",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.AddCommand(newServeCommand())
	cmd.AddCommand(newAuthCommand())

	return cmd
}

func Execute() error {
	return NewRootCommand().Execute()
}
