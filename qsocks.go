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
		<-pkg.StartSocks5Server(context.TODO(), pkg.Socks5Config{
			Listen:  *listen,
			Connect: *connect,
		}).Done()

	case *mode == "sqserver" && *listen != "":
		<-pkg.StartQuicServer(context.TODO(), pkg.QuicConfig{
			Listen: *listen,
		}).Done()

	default:
		flag.Usage()
	}

}
