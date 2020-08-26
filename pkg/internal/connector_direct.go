package internal

import (
	"fmt"
	"net"
)

func directConnector() (raceConnector, error) {
	return func(connBundle connectBundle) {
		conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", connBundle.addr, connBundle.port))
		if err != nil {
			connBundle.ctx.CancleWithError(err)
			return
		}
		connBundle.ctx.Cleanup(conn.Close)

		LogInfo("direct connector -> %s:%d", connBundle.addr, connBundle.port)
		connBundle.pushReady(conn)
	}, nil
}
