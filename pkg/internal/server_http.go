package internal

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"strings"
)

type httpBundle struct {
	ctx  CanclableContext
	addr string
}

type connBundle struct {
	ctx CanclableContext
	rw  *bufio.ReadWriter
}

func httpProxy(bundle httpBundle) {
	l, err := net.Listen("tcp", bundle.addr)
	if err != nil {
		bundle.ctx.CancleWithError(err)
		return
	}
	bundle.ctx.Cleanup(l.Close)

	go func() {
		for {
			conn, err := l.Accept()
			if err != nil {
				bundle.ctx.CancleWithError(err)
				return
			}

			connCtx := bundle.ctx.Derive(nil)
			connCtx.Cleanup(conn.Close)

			go serveConn(connBundle{
				connCtx,
				bufio.NewReadWriter(
					bufio.NewReader(io.LimitReader(conn, 4096)),
					bufio.NewWriter(conn),
				),
			})
		}
	}()
}

func serveConn(bundle connBundle) {
	line, err := HTTPReadline(bundle.rw.Reader)
	if err != nil {
		bundle.ctx.CancleWithError(err)
		return
	}

	// minimum protocol check
	parts := strings.Split(string(line), " ")
	switch {
	case len(parts) < 3:
		bundle.ctx.CancleWithError(fmt.Errorf("not supported protocol"))
		return
	case parts[0] != "CONNECT":
		bundle.ctx.CancleWithError(fmt.Errorf("method not support: %s", parts[0]))
		return
	case parts[2] != "HTTP/1.1":
		bundle.ctx.CancleWithError(fmt.Errorf("http 1.1 only: %s", parts[2]))
		return
	}

	for {
		// proxy header found
		LogInfo("try connect to: %s", parts[1])
		to, err := net.Dial("tcp", parts[1])
		if err != nil {
			bundle.ctx.CancleWithError(err)
			return
		}

		// attach context
		toCtx := bundle.ctx.Derive(nil)
		toCtx.Cleanup(to.Close)

		// send response
		_, err = io.WriteString(bundle.rw, strings.Join(
			[]string{
				"HTTP/1.1 201 Created\r\n",
				"\r\n",
			},
			"",
		))
		if err != nil {
			bundle.ctx.CancleWithError(err)
			return
		} else if err := bundle.rw.Flush(); err != nil {
			bundle.ctx.CancleWithError(err)
			return
		}

		// skip remainoing headers,if any
		for {
			line, err = HTTPReadline(bundle.rw.Reader)
			if err != nil {
				bundle.ctx.CancleWithError(err)
				return
			}

			// final \r\n
			if len(line) == 0 {
				break
			}
		}

		// then copy ok
		go BiCopy(toCtx, to, bundle.rw, io.Copy)
		return
	}
}

// StartHTTPServer start a http proxy server used by qsock client
func StartHTTPServer(ctx context.Context, listen string) CanclableContext {
	serverCtx := NewCanclableContext(ctx)

	go httpProxy(httpBundle{
		serverCtx,
		listen,
	})

	return serverCtx
}
