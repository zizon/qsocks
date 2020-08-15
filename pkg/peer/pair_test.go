package peer

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"reflect"
	"runtime"
	"testing"
)

type quicCollector struct{}

func (l quicCollector) Collect(err error) {
	switch err {
	case context.Canceled:
		return
	}
	buf := make([]byte, 1024)
	runtime.Stack(buf, false)
	fmt.Printf("collect error: %s:%v\n%s\n", reflect.TypeOf(err), err, buf)
}

func TestPair(t *testing.T) {
	pairConfig := PairConfig{
		LocalPeerConfig: LocalPeerConfig{
			Addr: "localhost:10087",
		},

		RemotePeerConfig: RemotePeerConfig{
			Addr: "localhost:10088",
		},
		ContextErrorAggregator: quicCollector{},
	}

	// root context
	ctx, cancler := context.WithCancel(context.TODO())

	// start server
	server, err := NewServerPeer(ctx, ServerPeerConfig{
		Addr:                   pairConfig.RemotePeerConfig.Addr,
		ContextErrorAggregator: pairConfig.ContextErrorAggregator,
	})
	if err != nil {
		t.Errorf("fail to start quic server,reason:%v", err)
	}

	// start paring
	if err := Pair(ctx, pairConfig); err != nil {
		t.Errorf("fail to start local peer,reason:%v config:%v", err, pairConfig)
	}

	// initialize connection
	conn, err := net.Dial("tcp", pairConfig.LocalPeerConfig.Addr)
	if err != nil {
		t.Errorf("fail to connect local endpoint:%v", pairConfig.LocalPeerConfig.Addr)
	}

	check := []byte("hello kitty")
	// write out
	go func() {
		n, err := conn.Write(check)
		if err != nil {
			t.Errorf("fail to write to conn:%v reason%v", conn, err)
		}

		if n != len(check) {
			t.Errorf("fail to write to remote,expect write:%v but wrote:%v", len(check), n)
		}
	}()

	// read in
	go func() {
		defer cancler()
		remote, err := server.PollNewChannel()
		if err != nil {
			t.Errorf("fail to accept quic channel,reason:%v", err)
		}

		buf := make([]byte, len(check))
		if _, err := remote.Read(buf); err != nil {
			t.Errorf("fail to read input quic")
		}

		if !bytes.Equal(check, buf) {
			t.Errorf("receive not match, expected:%v got:%v", check, buf)
		}
	}()

	<-ctx.Done()
}
