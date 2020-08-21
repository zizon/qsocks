package main

import (
	"context"
	"flag"
	"log"
	"net"

	"github.com/zizon/qsocks/pkg"

	"net/http"
	_ "net/http/pprof"
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

	// enable pprof
	go func() {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			log.Panic(err)
		}

		log.Printf("start pprof at:%s\n", l.Addr().String())
		http.Serve(l, nil)
	}()

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
