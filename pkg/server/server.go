package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/lucas-clemente/quic-go"
	"github.com/zizon/qsocks/pkg/logging"
	"github.com/zizon/qsocks/pkg/protocol"
)

type Config struct {
	context.Context
	context.CancelFunc
	Listen string
}

type Server interface {
	context.Context
}

type sessionStream struct {
	quic.Connection
	quic.Stream
}

func NewServer(config Config) (Server, error) {
	// create listener
	l, err := quic.ListenAddr(config.Listen, generateTLSConfig(), &quic.Config{
		EnableDatagrams: true,
		KeepAlivePeriod: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("fail to create quic listener:%v", err)
	}
	logging.Info("listening %v", l.Addr())

	go func() {
		defer func() {
			if err := l.Close(); err != nil {
				logging.Warn("fail close listener:%v", err)
			}

			config.CancelFunc()
		}()

		for {
			c, err := l.Accept(config)
			if err != nil {
				logging.Error("fail to accept stream:%v abort listening", err)
				return
			}

			go func() {
				defer c.CloseWithError(0, "close connection")
				for {
					stream, err := c.AcceptStream(config)
					if err != nil {
						logging.Error("fail to accept stream for session from:%v rason:%v", c.RemoteAddr(), err)
						return
					}

					go serve(sessionStream{
						Connection: c,
						Stream:     stream,
					})
				}
			}()
		}
	}()

	return config, nil
}

func serve(s sessionStream) {
	defer s.Stream.Close()

	logging.Info("accept stream:%v", s.remote())

	// 1. read request
	request := &protocol.Request{}
	if err := request.Decode(s); err != nil {
		logging.Error("unkonw request for stream:%v reaosn:%v", s.remote(), err)
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
		logging.Error("fail to resovle remote for stream:%v reason:%v", s.remote(), err)
		return
	}

	// 6. connect
	logging.Info("try piping: %v -> %v", s.remote(), addr)
	to, err := net.Dial("tcp", addr)
	if err != nil {
		logging.Error("fail piping connection %v -> %v reason:%v", s.remote(), addr, err)
		return
	}
	defer to.Close()

	logging.Info("quic pipe %v -> %v(%s)", s.remote(), addr, to.RemoteAddr())

	// reuse goroutine
	// one direction
	go func() {
		if _, err := io.Copy(s, to); err != nil {
			logging.Warn("piping %v(%v) -> %v fail:%v", addr, to.RemoteAddr(), s.remote(), err)
		}
	}()

	// the other direction
	if _, err := io.Copy(to, s); err != nil {
		logging.Warn("piping %v -> %v(%v) fail:%v", s.remote(), addr, to.RemoteAddr(), err)
	}
}

func (s *sessionStream) remote() string {
	return fmt.Sprintf("%v[%v](%v)", s.RemoteAddr(), s.StreamID(), s.LocalAddr())
}
