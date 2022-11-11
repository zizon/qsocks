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
	"github.com/zizon/qsocks/pkg/stream"
)

type Config struct {
	context.Context
	Listen string
}

type Server interface {
	context.Context
}

type server struct {
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

	s := &server{
		Context: config,
	}

	// create sessions
	sessions := stream.Of(func() (quic.Connection, error) {
		ss, err := l.Accept(s)
		if err != nil {
			logging.Error("fail to accept stream:%v", err)
			l.Close()
		}
		return ss, err
	})

	// accpet session sterams
	streams := stream.Flatten(sessions, func(session quic.Connection) (stream.State[sessionStream], error) {
		return stream.Finite(func() (sessionStream, bool) {
			ss, err := session.AcceptStream(s)
			if err != nil {
				logging.Error("fail to accept stream for session from:%v rason:%v", session.RemoteAddr(), err)
				session.CloseWithError(0, "")
				return sessionStream{}, false
			}

			return sessionStream{
				Connection: session,
				Stream:     ss,
			}, true
		}), nil
	}, false)

	// process each stream
	stream.Drain(streams, func(s sessionStream) error {
		go serve(s)
		return nil
	})

	return s, nil
}

func serve(s sessionStream) {
	defer s.Close()

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
