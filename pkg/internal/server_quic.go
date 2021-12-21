package internal

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"math/big"
	"net"
	"time"

	quic "github.com/lucas-clemente/quic-go"
)

var (
	// PeerQuicProtocol quic peer protocl
	peerQuicProtocol = []string{"quic-peer"}
)

type quicServer struct {
	CanclableContext
}

type quicStream struct {
	CanclableContext
	quic.Stream
}

// StartQuicServer start a quic proxy server
func StartQuicServer(ctx context.Context, listen string) CanclableContext {
	s := quicServer{
		CanclableContext: NewCanclableContext(ctx),
	}

	go func() {
		l, err := quic.ListenAddr(listen, generateTLSConfig(), &quic.Config{
			MaxIdleTimeout:  30 * time.Second,
			EnableDatagrams: true,
		})
		if err != nil {
			s.Cancle(fmt.Errorf("fail to listen quic:%v reason:%v", listen, err))
			return
		}
		s.OnCancle(func(e error) {
			l.Close()
		})

		for {
			session, err := l.Accept(s)
			if err != nil {
				s.Cancle(fmt.Errorf("fail to accept session:%v reason:%v", listen, err))
				return
			}
			LogInfo("start quic server session:%v\n", session.RemoteAddr())

			sessionCtx := s.Fork()
			sessionCtx.OnCancle(func(err error) {
				msg := fmt.Sprintf("session closed reason:%v", err)
				LogInfo(msg)
				session.CloseWithError(0, msg)
			})

			go func() {
				for {
					stream, err := session.AcceptStream(sessionCtx)
					if err != nil {
						sessionCtx.Cancle(fmt.Errorf("session fail to accept stream:%v", err))
						return
					}
					LogInfo("accept stream from:%v", session.RemoteAddr())

					streamCtx := sessionCtx.Fork()
					streamCtx.OnCancle(func(err error) {
						LogInfo("stream clsoed reason:%v", err)
						stream.Close()
					})

					go (&quicStream{
						CanclableContext: streamCtx,
						Stream:           stream,
					}).start()
				}
			}()
		}
	}()

	return s
}

// Setup a bare-bones TLS config for the server
func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
	}
	ca, err := x509.CreateCertificate(
		rand.Reader,
		&template,
		&template,
		&key.PublicKey,
		key,
	)
	if err != nil {
		panic(err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{{
			Certificate: [][]byte{ca},
			PrivateKey:  key,
		}},
		NextProtos: peerQuicProtocol,
	}
}

func (s *quicStream) start() {
	// 1. read request
	request := &Request{}
	if err := request.Decode(s); err != nil {
		s.Cancle(fmt.Errorf("unkonw request for stream:%v reaosn:%v", s, err))
		return
	}

	// 2. do command
	addr, err := func() (string, error) {
		switch request.CMD {
		case 0x01:
			// connect
			// build remote host & port
			switch request.ATYP {
			case 0x01, 0x04:
				return fmt.Sprintf("%s:%d", net.IP(request.HOST).String(), request.PORT), nil
			default:
				return fmt.Sprintf("%s:%d", string(request.HOST), request.PORT), nil
			}
		default:
			// 0x02 bind
			// 0x03 udp associate
			return "", fmt.Errorf("unkown command")
		}
	}()
	if err != nil {
		s.Cancle(fmt.Errorf("fail to resovle remote for stream:%v reason:%v", s, err))
		return
	}

	// 6. connect
	to, err := net.Dial("tcp", addr)
	if err != nil {
		s.Cancle(fmt.Errorf("fail to connect to:%v reason:%v", to, err))
		return
	}
	s.OnCancle(func(error) {
		to.Close()
	})

	LogInfo("quic tcp -> %s", addr)
	// one direction
	go func() {
		if _, err := io.Copy(s, to); err != nil {
			s.Cancle(fmt.Errorf("copy stream:%v to remote:%v fail:%v", s, to, err))
		} else {
			s.Cancle(nil)
		}
	}()

	// the other direction
	if _, err := io.Copy(to, s); err != nil {
		s.Cancle(fmt.Errorf("copy stream:%v to remote:%v fail:%v", s, to, err))
	} else {
		s.Cancle(nil)
	}
}
