package main

import (
	"context"
	"flag"

	"github.com/zizon/qsocks/pkg"
)

func main() {
	mode := flag.String("mode", "", `
server mode, avalibale:
	qsocks: 
		start a socks5 server locally,and forwarding connectio to remote quic server
	sqserver:
		start a quic server, which talks to qsocks endpoint
`,
	)
	listen := flag.String("listen", "", "listening address, in e.g 0.0.0.0:10086")
	connect := flag.String("connect", "", `
when useing qsocks mode, connect refer to remote quic listening addess,
simliry to listen.
`,
	)

	flag.Parse()

	switch {
	case *mode == "qsocks" && *listen != "" && *connect != "":
		ctx := context.TODO()
		pkg.StartSocks5Server(ctx, pkg.Socks5Config{
			Listen:  *listen,
			Connect: *connect,
		})

		<-ctx.Done()
	case *mode == "sqserver" && *listen != "":
		ctx := context.TODO()
		pkg.StartQuicServer(ctx, pkg.QuicConfig{
			Listen: *listen,
		})

		<-ctx.Done()
	default:
		flag.Usage()
	}

}
