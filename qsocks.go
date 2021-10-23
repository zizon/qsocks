package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"net/http"
	_ "net/http/pprof"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/zizon/qsocks/pkg"
)

func main() {
	// enable pprof
	go func() {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			log.Panic(err)
		}

		log.Printf("start pprof at:%s\n", l.Addr().String())
		http.Serve(l, nil)
	}()

	var (
		listen   string
		connect  string
		logLevel int
		timeout  int
		streams  int
	)

	rootCmd := cobra.Command{
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			pkg.SetLogLevel(logLevel)
		},
		Example: `
start a quic proxy server
./qsocks sqserver -l 0.0.0.0:10086

start a local socks5 server listening 10086 wich connect the remote quic server
which listen at port 10010
./qsocks qsocks -l 0.0.0.0. -c sqserver://{address.of.quic.server}:10010
		`,
	}
	rootCmd.PersistentFlags().IntVarP(&logLevel, "verbose", "v", 2,
		"log verbose level from 0 - 5, higher means more verbose, default 2")

	// qsocks
	qsocksCmd := &cobra.Command{
		Use:   "qsocks",
		Short: "start a local socks5 server",
		RunE: func(cmd *cobra.Command, args []string) error {
			addr, err := url.Parse(connect)
			if err != nil {
				return fmt.Errorf("fail to parse connect:%v", connect)
			}

			ctx := pkg.StartSocks5Server(context.TODO(), pkg.Socks5Config{
				Listen:           listen,
				Connect:          fmt.Sprintf("%s:%s", addr.Hostname(), addr.Port()),
				Timeout:          timeout,
				StreamPerSession: streams,
			})
			<-ctx.Done()

			return ctx.Err()
		},
	}

	qsocksCmd.Flags().StringVarP(&listen, "listen", "l", "0.0.0.0:10086", "local socks5 listening  address")

	qsocksCmd.Flags().StringVarP(&connect, "connect", "c", "",
		"remote server to connect for quic, sqserver://your.server:port")
	qsocksCmd.MarkFlagRequired("connect")

	qsocksCmd.Flags().IntVarP(&timeout, "timeout", "t", 0, "timeout for connecting remote,in seconds")

	qsocksCmd.Flags().IntVarP(&streams, "streams", "s", 1, "stream per quic session")

	// sqsocks
	sqserverCmd := &cobra.Command{
		Use:   "sqserver",
		Short: "run a quic proxy server",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := pkg.StartQuicServer(context.TODO(), pkg.QuicConfig{
				Listen: listen,
			})
			<-ctx.Done()

			return ctx.Err()
		},
	}
	sqserverCmd.Flags().StringVarP(&listen, "listen", "l", "0.0.0.0:10086", "quic listening address")

	// aggrate command
	rootCmd.AddCommand(qsocksCmd)
	rootCmd.AddCommand(sqserverCmd)

	// go
	panic(rootCmd.Execute())
}
