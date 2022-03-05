package main

import (
	"fmt"
	"net"

	"net/http"
	_ "net/http/pprof"

	"github.com/spf13/cobra"
	"github.com/zizon/qsocks/cmd"
	"github.com/zizon/qsocks/pkg/logging"
)

func main() {
	// enable pprof
	go func() {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			panic(fmt.Errorf("fail to start pprof:%v", err))
		}

		logging.Info("start pprof at:%v", l.Addr())
		http.Serve(l, nil)
	}()

	var (
		logLevel int
	)

	root := cobra.Command{
		Example: `
start a quic proxy server
./qsocks sqserver -l 0.0.0.0:10086

start a local socks5 server listening 10086 wich connect the remote quic server
which listen at port 10010
./qsocks qsocks -l 0.0.0.0. -c sqserver://{address.of.quic.server}:10010
		`,
	}
	root.PersistentFlags().IntVarP(&logLevel, "verbose", "v", 2,
		"log verbose level from 0 - 5, higher means more verbose, default 2")

	// aggrate command
	root.AddCommand(cmd.NewQsocksCommand())
	root.AddCommand(cmd.NewSqserverCommand())

	// go
	if err := root.Execute(); err != nil {
		logging.Error("fail execute command:%v", err)
	}
}
