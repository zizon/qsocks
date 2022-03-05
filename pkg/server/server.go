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

func NewServer(config Config) (Server, error) {
	// create listener
	l, err := quic.ListenAddr(config.Listen, generateTLSConfig(), &quic.Config{
		MaxIdleTimeout:  30 * time.Second,
		EnableDatagrams: true,
	})
	if err != nil {
		return nil, fmt.Errorf("fail to create quic listener:%v", err)
	}
	logging.Info("listening %v", l.Addr())

	s := &server{
		Context: config,
	}

	// create sessions
	sessions := stream.Of(func() (quic.Session, error) {
		return l.Accept(s)
	})

	// accpet session sterams
	streams := stream.Flatten(sessions, func(session quic.Session) (stream.State[quic.Stream], error) {
		return stream.Of(func() (quic.Stream, error) {
			return session.AcceptStream(s)
		}), nil
	}, false)

	// process each stream
	stream.Drain(streams, func(s quic.Stream) error {
		logging.Info("accept stream:%v", s.StreamID())
		go serve(s)
		return nil
	})

	return s, nil
}

func serve(s quic.Stream) {
	// 1. read request
	request := &protocol.Request{}
	if err := request.Decode(s); err != nil {
		logging.Error("unkonw request for stream:%v reaosn:%v", s, err)
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
		logging.Error("fail to resovle remote for stream:%v reason:%v", s, err)
		return
	}

	// 6. connect
	to, err := net.Dial("tcp", addr)
	if err != nil {
		logging.Error("fail to connect to:%v reason:%v", to, err)
		return
	}

	logging.Info("quic tcp %s -> %s", s.StreamID(), to.RemoteAddr())
	// one direction
	go func() {
		if _, err := io.Copy(s, to); err != nil {
			logging.Warn("copy stream:%v to remote:%v fail:%v", to.RemoteAddr(), s.StreamID(), err)
		}
	}()

	// the other direction
	go func() {
		if _, err := io.Copy(to, s); err != nil {
			logging.Warn("copy stream:%v to remote:%v fail:%v", s.StreamID(), to.RemoteAddr(), err)
		}
	}()
}
