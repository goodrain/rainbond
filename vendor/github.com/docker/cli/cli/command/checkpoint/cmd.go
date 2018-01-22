package checkpoint

import (
	"github.com/docker/cli/cli"
	"github.com/docker/cli/cli/command"
	"github.com/spf13/cobra"
)

// NewCheckpointCommand returns the `checkpoint` subcommand (only in experimental)
func NewCheckpointCommand(dockerCli *command.DockerCli) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "checkpoint",
		Short: "Manage checkpoints",
		Args:  cli.NoArgs,
		RunE:  dockerCli.ShowHelp,
		Tags:  map[string]string{"experimental": "", "version": "1.25"},
	}
	cmd.AddCommand(
		newCreateCommand(dockerCli),
		newListCommand(dockerCli),
		newRemoveCommand(dockerCli),
	)
	return cmd
}
