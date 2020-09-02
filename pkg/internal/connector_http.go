package internal

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
)

type httpConnectorBundle struct {
	ctx     CanclableContext
	gateway string
}

func httpConnector(bundle httpConnectorBundle) (raceConnector, error) {
	return func(connBundle connectBundle) {
		conn, err := net.Dial("tcp", bundle.gateway)
		if err != nil {
			connBundle.ctx.CancleWithError(err)
			return
		}
		connBundle.ctx.Cleanup(conn.Close)

		// make stream
		from := bufio.NewReadWriter(
			bufio.NewReader(io.LimitReader(conn, 4096)),
			bufio.NewWriter(conn),
		)

		// send http reqeust
		_, err = io.WriteString(from, strings.Join(
			[]string{
				"POST / HTTP/1.1 \r\n",
				fmt.Sprintf("HOST:%s \r\n", bundle.gateway),
				fmt.Sprintf("Proxy:%s:%d\r\n", connBundle.addr, connBundle.port),
				"TransferEncoding:identify\r\n", // maybe settign to compilant behavior
				"\r\n",
			},
			"",
		))
		if err != nil {
			connBundle.ctx.CancleWithError(err)
			return
		} else if err := from.Flush(); err != nil {
			connBundle.ctx.CancleWithError(err)
			return
		}

		// read status line
		status, err := HTTPReadline(from.Reader)
		if err != nil {
			bundle.ctx.CancleWithError(err)
			return
		}

		parts := strings.Split(string(status), " ")
		if len(parts) < 2 {
			connBundle.ctx.CancleWithError(fmt.Errorf("protocol not understand"))
			return
		} else if parts[1] != "201" {
			// just check status code
			connBundle.ctx.CancleWithError(fmt.Errorf("connection not created"))
			return
		}

		// skip headers
		for {
			line, err := HTTPReadline(from.Reader)
			if err != nil {
				bundle.ctx.CancleWithError(err)
				return
			}

			// last \r\n, header end
			if len(line) == 0 {
				break
			}
		}

		// set ready
		connBundle.pushReady(from)
	}, nil
}

// HTTPReadline read a http \r\n
func HTTPReadline(r *bufio.Reader) ([]byte, error) {
	line, trim, err := r.ReadLine()
	switch {
	case trim:
		return nil, bufio.ErrBufferFull
	case err != nil:
		return nil, err
	}

	return line, nil
}
