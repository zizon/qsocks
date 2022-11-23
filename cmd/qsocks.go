package cmd

import (
	"context"
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/cobra"
	"github.com/zizon/qsocks/pkg/client"
)

func NewQsocksCommand() *cobra.Command {
	ctx, cancle := context.WithCancel(context.TODO())
	config := client.Config{
		Context:    ctx,
		CancelFunc: cancle,
	}

	// qsocks
	cmd := &cobra.Command{
		Use:   "qsocks",
		Short: "start a local socks5 server",
		RunE: func(cmd *cobra.Command, args []string) error {
			parsed, err := url.Parse(config.Connect)
			if err != nil {
				return fmt.Errorf("fail to parse quic server:%v", err)
			}
			config.Connect = parsed.Host

			c, err := client.NewClient(config)
			if err != nil {
				return fmt.Errorf("fail to ceate local socks5 server:%v", err)
			}
			<-c.Done()

			return c.Err()
		},
	}

	cmd.Flags().StringVarP(&config.Listen, "listen", "l", "0.0.0.0:10086", "local socks5 listening  address")

	cmd.Flags().StringVarP(&config.Connect, "connect", "c", "",
		"remote server to connect for quic, sqserver://your.server:port")
	cmd.MarkFlagRequired("connect")

	cmd.Flags().DurationVarP(&config.Timeout, "timeout", "t", 10*time.Second, "timeout for connecting remote,in seconds")

	cmd.Flags().IntVarP(&config.StreamPerSession, "streams", "s", 5, "stream per quic session")

	return cmd
}
