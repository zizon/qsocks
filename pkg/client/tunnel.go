package client

import (
	"fmt"
	"io"
	"net"

	"github.com/lucas-clemente/quic-go"
	"github.com/zizon/qsocks/pkg/logging"
	"github.com/zizon/qsocks/pkg/protocol"
)

type tunnel struct {
	from net.Conn
	toCh <-chan quic.Stream
	to   quic.Stream
}

func (t tunnel) serve() {
	t.to = <-t.toCh
	logging.Info("tunnel:%v created", t)
	if t.to == nil {
		t.close(fmt.Sprintf("no quic stream bound, invalid tunnel:%v", t))
		return
	}

	// 1. fast reply local with known message
	go t.replyLocal()

	// 2. consume unnecessary auth before forward to remote
	if err := (&protocol.Auth{}).Decode(t.from); err != nil {
		t.close(fmt.Sprintf("unkonw auth message for tunnel:%v from local:%v", t, err))
		return
	}

	// 3. then forward remainds to remote
	if _, err := io.Copy(t.to, t.from); err != nil {
		t.close(fmt.Sprintf("tunnel:%v fail pipe local to remote:%v", t, err))
		return
	}

	t.close(fmt.Sprintf("tunnel:%v stop pipe local to remote", t))
}

func (t tunnel) replyLocal() {
	// auth reply
	if err := (protocol.AuthReply{}).Encode(t.from); err != nil {
		t.close(fmt.Sprintf("fail auth reply tunnel:%v reason:%v", t, err))
		return
	}

	// proxy addr reply
	addr, ok := t.from.LocalAddr().(*net.TCPAddr)
	if !ok {
		t.close(fmt.Sprintf("expect addr to be tcp addr:%v", t.from.LocalAddr()))
		return
	} else if err := (&protocol.Reply{
		HOST: addr.IP,
		PORT: addr.Port,
	}).Encode(t.from); err != nil {
		t.close(fmt.Sprintf("fail proxy addr reply tunnel:%v reaosn:%v", t, err))
		return
	}

	// then copy
	if _, err := io.Copy(t.from, t.to); err != nil {
		t.close(fmt.Sprintf("tunnel:%v fail pipe remote to local:%v", t, err))
		return
	}

	t.close(fmt.Sprintf("tunnel:%v stop pipe remote to local", t))
}

func (t tunnel) close(reason string) {
	logging.Info(reason)
	t.from.Close()
	if t.to != nil {
		t.to.Close()
	}
}

func (t tunnel) String() string {
	if t.to != nil {
		return fmt.Sprintf("(%s->%v)", t.from.RemoteAddr(), t.to.StreamID())
	}

	return fmt.Sprintf("(%s->nil)", t.from.RemoteAddr())
}
