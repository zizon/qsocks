package internal

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
)

type socks5RaceServerBundle struct {
	serverCtx  CanclableContext
	listen     string
	connectors []raceConnector
}

// StartSocks5RaceServer start a server that rece connecto to remote
func StartSocks5RaceServer(ctx context.Context, listen, connect string) CanclableContext {
	serverCtx := NewCanclableContext(ctx)

	connectors := make([]raceConnector, 0)
	for _, connector := range strings.Split(connect, ",") {
		connector := strings.TrimSpace(connector)
		parts := strings.Split(connector, "://")
		scheme, connect := "quic", connector
		switch len(parts) {
		case 1:
		case 2:
			scheme = parts[0]
			connect = parts[1]
		default:
			serverCtx.CancleWithError(fmt.Errorf("not supoorted connect string:%s", connector))
			return serverCtx
		}

		switch scheme {
		case "quic":
			connectorCtx := serverCtx.Derive(nil)
			c, err := quicConnector(quicConnectorBundle{
				connectorCtx,
				connect,
			})
			if err != nil {
				LogWarn("connector:%s fail to initialize, thus disabled", connector)
			}
			connectors = append(connectors, c)
		default:
			LogWarn("unsupported connector:%s")
		}
	}

	go socks5RaceServer(socks5RaceServerBundle{
		serverCtx,
		listen,
		connectors,
	})
	return serverCtx
}

func socks5RaceServer(bundle socks5RaceServerBundle) {
	serverCtx := bundle.serverCtx
	addr, err := net.ResolveTCPAddr("", bundle.listen)
	if err != nil {
		serverCtx.CancleWithError(err)
		return
	}

	l, err := net.ListenTCP(addr.Network(), addr)
	if err != nil {
		serverCtx.CancleWithError(err)
		return
	}
	serverCtx.Cleanup(l.Close)

	LogInfo("socks5 race listen at:%s", addr)

	for {
		from, err := l.AcceptTCP()
		if err != nil {
			serverCtx.CancleWithError(err)
			return
		}

		// pull remote info
		connCtx := serverCtx.Derive(nil)
		connCtx.Cleanup(from.Close)
		go func() {
			addr, port, err := pullSocks5Request(from)
			if err != nil {
				connCtx.CancleWithError(err)
				return
			}

			LogInfo("race connect -> %s:%d", addr, port)
			// race connect
			to := receConnect(raceBundle{
				connCtx,
				bundle.connectors,
				addr,
				port,
			})
			if to == nil {
				connCtx.CancleWithError(
					fmt.Errorf("pull no connections for remote: %s:%d",
						addr, port))
				return
			}

			// blocking copy
			BiCopy(connCtx, from, to, io.Copy)
		}()
	}
}

func pullSocks5Request(from net.Conn) (string, int, error) {
	addr, err := net.ResolveTCPAddr(from.LocalAddr().Network(), from.LocalAddr().String())
	if err != nil {
		return "", 0, err
	}

	// 1. read auth
	auth := &Auth{}
	if err := auth.Decode(from); err != nil {
		return "", 0, err
	}

	// 2. auth
	authReply := AuthReply{}
	if err := authReply.Encode(from); err != nil {
		return "", 0, err
	}

	// 3. read request
	request := &Request{}
	if err := request.Decode(from); err != nil {
		return "", 0, err
	}

	// 4. reply
	reply := &Reply{
		HOST: []byte(addr.IP.String()),
		PORT: addr.Port,
	}
	if err := reply.Encode(from); err != nil {
		return "", 0, err
	}

	// 5. do command
	switch request.CMD {
	case 0x01:
		// connect
		// build remote host & port
		switch request.ATYP {
		case 0x01, 0x04:
			return net.IP(request.HOST).String(), request.PORT, nil
		default:
			return string(request.HOST), request.PORT, nil
		}
	case 0x02:
		// bind
		// TODO
	case 0x03:
		// udp associate
		// TODO
	default:
		return "", 0, fmt.Errorf("unkown command")
	}

	return "", 0, fmt.Errorf("not supported command")
}
