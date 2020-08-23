package internal

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/lucas-clemente/quic-go"
)

type socks5ProxyBundle struct {
	proxyCtx CanclableContext
	from     net.Conn
	newQuic  func() (quic.Stream, error)
}

func socks5Proxy(bundle socks5ProxyBundle) {
	from := bundle.from

	addr, err := net.ResolveTCPAddr(from.LocalAddr().Network(), from.LocalAddr().String())
	if err != nil {
		bundle.proxyCtx.CancleWithError(err)
		return
	}

	// 1. read auth
	auth := &Auth{}
	if err := auth.Decode(from); err != nil {
		bundle.proxyCtx.CancleWithError(err)
		return
	}

	// 2. auth
	authReply := AuthReply{}
	if err := authReply.Encode(from); err != nil {
		bundle.proxyCtx.CancleWithError(err)
		return
	}

	// 3. read request
	request := &Request{}
	if err := request.Decode(from); err != nil {
		bundle.proxyCtx.CancleWithError(err)
		return
	}

	// 4. reply
	reply := &Reply{
		HOST: []byte(addr.IP.String()),
		PORT: addr.Port,
	}
	if err := reply.Encode(from); err != nil {
		bundle.proxyCtx.CancleWithError(err)
		return
	}

	// 5. do command
	switch request.CMD {
	case 0x01:
		// connect
		// 1. setup connection
		LogInfo("receive connect command")
		to, err := bundle.newQuic()
		if err != nil {
			bundle.proxyCtx.CancleWithError(err)
			return
		}
		bundle.proxyCtx.Cleanup(to.Close)

		// 2. write request header
		packet := QsockPacket{
			TYPE: 1,
			PORT: request.PORT,
		}
		switch request.ATYP {
		case 0x01, 0x04:
			packet.HOST = net.IP(request.HOST).String()
		default:
			packet.HOST = string(request.HOST)
		}
		if err := packet.Encode(to); err != nil {
			bundle.proxyCtx.CancleWithError(err)
			return
		}
		LogInfo("socks5 connect -> %s:%d", packet.HOST, packet.PORT)

		// 3. assume every thing ok.
		BiCopy(bundle.proxyCtx, from, to, io.Copy)
		return
	case 0x02:
		// bind
		// TODO
	case 0x03:
		// udp associate
		// TODO
	default:
		bundle.proxyCtx.CancleWithError(fmt.Errorf("unkown command"))
		return
	}

	bundle.proxyCtx.CancleWithError(fmt.Errorf("not supported command"))
	return
}

type socks5ServerBundle struct {
	serverCtx CanclableContext
	listen    string
	remote    string
}

// StartSocks5Server start a local socks->quic server
func StartSocks5Server(ctx context.Context, listen, connect string) CanclableContext {
	serverCtx := NewCanclableContext(ctx)

	go socks5Server(socks5ServerBundle{
		serverCtx,
		listen,
		connect,
	})

	return serverCtx
}

func socks5Server(bundle socks5ServerBundle) {
	addr, err := net.ResolveTCPAddr("", bundle.listen)
	if err != nil {
		bundle.serverCtx.CancleWithError(err)
		return
	}

	l, err := net.ListenTCP(addr.Network(), addr)
	if err != nil {
		bundle.serverCtx.CancleWithError(err)
		return
	}
	bundle.serverCtx.Cleanup(l.Close)

	LogInfo("socks5 listen at:%s", addr)

	for {
		sessionCtx := bundle.serverCtx.Derive(nil)
		s, err := quic.DialAddrContext(sessionCtx, bundle.remote, &tls.Config{
			InsecureSkipVerify: true,
			NextProtos:         PeerQuicProtocol,
		}, &quic.Config{})
		if err != nil {
			sessionCtx.CancleWithError(err)
			continue
		}
		sessionCtx.Cleanup(func() error {
			s.CloseWithError(0, "")
			return nil
		})
		LogInfo("connect quic:%s", bundle.remote)

		// stream limit for each session
		wg := &sync.WaitGroup{}
		for i := 0; i < 10; i++ {
			conn, err := l.AcceptTCP()
			//conn.SetNoDelay(true)
			if err != nil {
				bundle.serverCtx.CancleWithError(err)
				return
			}

			LogInfo("accept socks5 from %s -> %s", conn.RemoteAddr(), conn.LocalAddr())
			wg.Add(1)
			connCtx := sessionCtx.Derive(nil)
			connCtx.Cleanup(conn.Close)
			connCtx.Cleanup(func() error {
				LogInfo("a stream closed")
				wg.Done()
				return nil
			})
			go socks5Proxy(socks5ProxyBundle{
				connCtx,
				conn,
				s.OpenStream,
			})
		}

		// free up session
		go func() {
			wg.Wait()
			LogInfo("freeup session:%v", s)
			sessionCtx.Cancle()
		}()
	}
}
