package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/zizon/qsocks/pkg/server"
)

func NewSqserverCommand() *cobra.Command {
	ctx, cancler := context.WithCancel(context.TODO())
	config := server.Config{
		Context:    ctx,
		CancelFunc: cancler,
	}

	// sqsocks
	cmd := &cobra.Command{
		Use:   "sqserver",
		Short: "run a quic proxy server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return server.Run(config)
		},
	}
	cmd.Flags().StringVarP(&config.Listen, "listen", "l", "0.0.0.0:10086", "quic listening address")
	return cmd
}
