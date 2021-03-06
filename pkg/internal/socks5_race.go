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
	timeout    int
}

// StartSocks5RaceServer start a server that rece connecto to remote with limited stream per session
func StartSocks5RaceServer(ctx context.Context, listen, connect string, timeout int) CanclableContext {
	return StartSessionLimitedSocks5RaceServer(ctx, listen, connect, timeout, 1)
}

// StartSessionLimitedSocks5RaceServer start a server that rece connecto to remote
func StartSessionLimitedSocks5RaceServer(ctx context.Context, listen, connect string, timeout int, streamPerSession int) CanclableContext {
	serverCtx := NewCanclableContext(ctx)

	connectors := make([]raceConnector, 0)
	for _, connector := range strings.Split(connect, ",") {
		LogInfo("setup connector: %s", connector)
		connector := strings.TrimSpace(connector)
		parts := strings.Split(connector, "://")
		scheme, connect := "sqserver", connector
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
		case "sqserver":
			connectorCtx := serverCtx.Derive(nil)
			c, err := quicConnector(quicConnectorBundle{
				connectorCtx,
				connect,
				streamPerSession,
				timeout,
			})
			if err != nil {
				connectorCtx.CancleWithError(err)
				continue
			}

			connectors = append(connectors, c)
		case "direct":
			c, err := directConnector()
			if err != nil {
				serverCtx.CollectError(err)
				continue
			}

			connectors = append(connectors, c)
		case "http":
			connectorCtx := serverCtx.Derive(nil)
			c, err := httpConnector(httpConnectorBundle{
				connectorCtx,
				connect,
			})

			if err != nil {
				connectorCtx.CancleWithError(err)
				continue
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
		timeout,
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

		go func() {
			// pull remote info
			connCtx := serverCtx.Derive(nil)
			connCtx.Cleanup(from.Close)

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
				bundle.timeout,
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
