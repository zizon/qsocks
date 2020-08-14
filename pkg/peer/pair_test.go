package peer

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"testing"

	quic "github.com/lucas-clemente/quic-go"
)

type quicCollector struct{}

func (l quicCollector) Collect(err error) {
	fmt.Errorf("local endpoint error:%v", err)
}

func TestPair(t *testing.T) {
	config := PairConfig{
		LocalPeerConfig: LocalPeerConfig{
			Addr: "localhost:10087",
		},

		RemotePeerConfig: RemotePeerConfig{
			Addr: "localhost:10088",
		},
		Collector: quicCollector{},
	}

	lquic, err := quic.ListenAddr(
		config.RemotePeerConfig.Addr,
		generateTLSConfig(),
		nil,
	)
	if err != nil {
		t.Errorf("fail to listen quic:%v reason:%v", config.LocalPeerConfig, err)
	}
	t.Logf("start quic server:%v", config.RemotePeerConfig)

	ctx := context.TODO()
	pairCancel, err := Pair(ctx, config)
	if err != nil {
		t.Errorf("fail to start pairing, reason:%v", err)
	}

	check := []byte("hello kitty")

	c := make(chan bool)
	go startQuicServer(ctx, lquic, t, check, c)

	conn, err := net.Dial("tcp", config.LocalPeerConfig.Addr)
	if err != nil {
		t.Errorf("fail to connect local endpoint:%v", config.LocalPeerConfig.Addr)
	}
	t.Logf("connect to local:%v", config.LocalPeerConfig)

	n, err := conn.Write(check)
	if err != nil {
		t.Errorf("fail to write to conn:%v reason%v", conn, err)
	}
	t.Logf("wait for response... write:%v", n)
	<-c
	pairCancel()
}

func startQuicServer(ctx context.Context, lquic quic.Listener, t *testing.T, check []byte, c chan bool) {
	session, err := lquic.Accept(ctx)
	t.Logf("receive session:%v", session)
	if err != nil {
		t.Errorf("fail to accept quic session, reason:%v", err)
	}

	stream, err := session.AcceptStream(ctx)
	t.Logf("accept quic stream:%v", stream)

	if err != nil {
		t.Errorf("fail to accept quic srream, reason:%v", err)
	}

	buf := make([]byte, len(check))
	n, err := stream.Read(buf)
	if err != nil {
		t.Errorf("fail to read quic stream")
	}

	if n != len(check) || !bytes.Equal(check, buf) {
		t.Errorf("receive bytes not match, expected:%s, got:%s", check, buf)
	}

	t.Log("receive match")
	stream.Close()

	c <- true
}
