package main

import (
	"context"
	"log"
	"net"

	"net/http"
	_ "net/http/pprof"

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
		mode     bool
		timeout  int
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

start a http proxy server
./qsocks http -l 0.0.0.0:8080

start a blind tunnel server
./qsocks blind -l 0.0.0.0:10086 -t 127.0.0.1:10087
		`,
	}
	rootCmd.PersistentFlags().IntVarP(&logLevel, "verbose", "v", 2,
		"log verbose level from 0 - 4, higher means more verbose, default 2")
	rootCmd.PersistentFlags().BoolVarP(&mode, "mode", "", false, "qsocks command mode,deprecated")

	// qsocks
	qsocksCmd := &cobra.Command{
		Use:   "qsocks",
		Short: "start a local socks5 server",
		Run: func(cmd *cobra.Command, args []string) {
			<-pkg.StartSocks5Server(context.TODO(), pkg.Socks5Config{
				Listen:  listen,
				Connect: connect,
			}).Done()
		},
	}

	qsocksCmd.Flags().StringVarP(&listen, "listen", "l", "0.0.0.0:10086", "local socks5 listening  address")
	qsocksCmd.MarkFlagRequired("listen")

	qsocksCmd.Flags().StringVarP(&connect, "connect", "c", "",
		"remote server to connect for quic, sqserver://your.server:port, for direct, direct://, for http, http://your.server:port")
	qsocksCmd.MarkFlagRequired("connect")

	qsocksCmd.Flags().IntVarP(&timeout, "timeout", "t", 0, "timeout for connecting remote,in seconds")

	// sqsocks
	sqserverCmd := &cobra.Command{
		Use:   "sqserver",
		Short: "run a quic proxy server",
		Run: func(cmd *cobra.Command, args []string) {
			<-pkg.StartQuicServer(context.TODO(), pkg.QuicConfig{
				Listen: listen,
			}).Done()
		},
	}
	sqserverCmd.Flags().StringVarP(&listen, "listen", "l", "0.0.0.0:10086", "quic listening address")
	sqserverCmd.MarkFlagRequired("listen")

	// http
	httpCmd := &cobra.Command{
		Use:   "http",
		Short: "run a http proxy server",
		Run: func(cmd *cobra.Command, args []string) {
			<-pkg.StartHTTPServer(context.TODO(), pkg.HTTPConfig{
				Listen: listen,
			}).Done()
		},
	}
	httpCmd.Flags().StringVarP(&listen, "listen", "l", "0.0.0.0:10086", "quic listening address")
	httpCmd.MarkFlagRequired("listen")

	// http
	blindCmd := &cobra.Command{
		Use:   "blind",
		Short: "run a blind tunnel server",
		Run: func(cmd *cobra.Command, args []string) {
			<-pkg.StartBlindServer(context.TODO(), pkg.BlindConfig{
				Listen:  listen,
				Forward: connect,
			}).Done()
		},
	}
	blindCmd.Flags().StringVarP(&listen, "listen", "l", "0.0.0.0:10086", "blind listening address")
	blindCmd.MarkFlagRequired("listen")

	blindCmd.Flags().StringVarP(&connect, "target", "t", "",
		"blind forwrding target")
	blindCmd.MarkFlagRequired("target")

	// aggrate command
	rootCmd.AddCommand(qsocksCmd)
	rootCmd.AddCommand(sqserverCmd)
	rootCmd.AddCommand(httpCmd)
	rootCmd.AddCommand(blindCmd)

	// go
	rootCmd.Execute()
}
