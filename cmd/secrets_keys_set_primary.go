package cmd

import (
	"fmt"

	"github.com/opencenter-cloud/opencenter-cli/internal/secrets"
	"github.com/spf13/cobra"
)

func newSecretsKeysSetPrimaryCmd() *cobra.Command {
	var cluster, keyType, fingerprint string

	cmd := &cobra.Command{
		Use:   "set-primary",
		Short: "Select an existing key as the cluster primary",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if keyType != string(secrets.KeyTypeAge) && keyType != string(secrets.KeyTypeSSH) {
				return fmt.Errorf("invalid key type %q: expected age or ssh", keyType)
			}
			logger := createSecretsLogger()
			sopsManager := createSOPSManager(logger)
			registryPath, err := getSecretsRegistryPath()
			if err != nil {
				return err
			}
			registry := secrets.NewDefaultKeyRegistry(registryPath, &sopsEncryptorAdapter{manager: sopsManager}, logger)
			if err := registry.SetPrimaryKey(cmd.Context(), cluster, secrets.KeyType(keyType), fingerprint); err != nil {
				return fmt.Errorf("failed to set primary %s key %q for cluster %q: %w", keyType, fingerprint, cluster, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Primary %s key for cluster %s set to %s\n", keyType, cluster, fingerprint)
			return nil
		},
	}
	cmd.Flags().StringVar(&cluster, "cluster", "", "cluster name or organization/cluster")
	cmd.Flags().StringVar(&keyType, "type", "", "key type: age or ssh")
	cmd.Flags().StringVar(&fingerprint, "fingerprint", "", "exact key fingerprint")
	_ = cmd.MarkFlagRequired("cluster")
	_ = cmd.MarkFlagRequired("type")
	_ = cmd.MarkFlagRequired("fingerprint")
	return cmd
}
