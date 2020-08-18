package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"github.com/lucas-clemente/quic-go"
	"github.com/zizon/qsocks/pkg/internal"
)

type ClientConfig struct {
	RemoteHost string
	RemotePort int
	ListenHost string
	ListenPort int
}

func StartClient(ctx context.Context, config ClientConfig, logger func(error)) context.CancelFunc {
	clientCtx := internal.NewCanclableContext(ctx, logger)

	go func() {
		l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", config.ListenHost, config.ListenPort))
		if err != nil {
			clientCtx.CancleWithError(err)
			return
		}
		clientCtx.Cleanup(l.Close)

		s, err := quic.DialAddrContext(
			clientCtx,
			fmt.Sprintf("%s:%d", config.RemoteHost, config.RemotePort),
			&tls.Config{
				InsecureSkipVerify: true,
				NextProtos:         internal.PeerQuicProtocol,
			},
			&quic.Config{})
		if err != nil {
			clientCtx.CancleWithError(err)
			return
		}
		clientCtx.Cleanup(func() error {
			return s.CloseWithError(0, "")
		})

		internal.StartProxy(clientCtx, clientProxy{
			clientCtx,
			l,
			s,
		}).Cleanup(func() error {
			clientCtx.Cancle()
			return nil
		})
	}()

	return clientCtx.Cancle
}

type clientProxy struct {
	internal.CanclableContext
	l net.Listener
	s quic.Session
}

func (proxy clientProxy) Accept() (net.Conn, error) {
	return proxy.l.Accept()
}

func (proxy clientProxy) Connect() (net.Conn, error) {
	stream, err := proxy.s.OpenStreamSync(proxy.CanclableContext)
	if err != nil {
		return nil, err
	}

	return streamConn{
		stream,
		proxy,
	}, err
}

type streamConn struct {
	quic.Stream
	clientProxy
}

func (sn streamConn) LocalAddr() net.Addr {
	return sn.clientProxy.l.Addr()
}

func (sn streamConn) RemoteAddr() net.Addr {
	return sn.s.RemoteAddr()
}

func (sn streamConn) SetDeadline(t time.Time) error {
	return nil
}

func (sn streamConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (sn streamConn) SetWriteDeadline(t time.Time) error {
	return nil
}
