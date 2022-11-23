package server

import (
	"fmt"
	"io"
	"net"

	"github.com/lucas-clemente/quic-go"
	"github.com/zizon/qsocks/pkg/logging"
	"github.com/zizon/qsocks/pkg/protocol"
)

type tunnel struct {
	quic.Stream
	remote string
}

func (t tunnel) serve() {
	// 1. read request
	request := &protocol.Request{}
	if err := request.Decode(t.Stream); err != nil {
		t.close(fmt.Sprintf("unknonw request in tunnel:%v reason:%v", t, err))
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
		t.close(fmt.Sprintf("tunnel:%v fail to resolve target addr:%v", t, err))
		return
	}
	t.remote = addr

	logging.Info("creating tunnel:%v", t)
	to, err := net.Dial("tcp", addr)
	if err != nil {
		t.close(fmt.Sprintf("fail to create tunnel:%v reason:%v", t, err))
		return
	}

	// copy from remote
	go func() {
		if _, err := io.Copy(t, to); err != nil {
			t.close(fmt.Sprintf("tunnel:%v fail pipe from remote:%v", t, err))
			return
		}

		t.close(fmt.Sprintf("tunnel:%v stop pipe from reomote", t))
	}()

	// copy to remote
	if _, err := io.Copy(to, t); err != nil {
		t.close(fmt.Sprintf("tunnel:%v fail pipe to remote:%v", t, err))
	}
	t.close(fmt.Sprintf("tunnel:%v stop pipe to reomote", t))
}

func (t tunnel) close(reason string) {
	logging.Info(reason)
	t.Close()
}

func (t tunnel) String() string {
	return fmt.Sprintf("(%v->%s)", t.StreamID(), t.remote)
}
