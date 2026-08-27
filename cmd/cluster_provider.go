package cmd

import "github.com/spf13/cobra"

func newClusterProviderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provider",
		Short: "Manage provider configuration",
		Args:  cobra.NoArgs,
		RunE:  func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}
	cmd.AddCommand(newClusterProviderOpenStackCmd())
	return cmd
}
