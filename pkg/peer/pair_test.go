package peer

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"testing"

	quic "github.com/lucas-clemente/quic-go"
)

type quicListener struct{}

func (l quicListener) OnLocalEnpointError(err error) {
	fmt.Errorf("local endpoint error:%v", err)
}
func (l quicListener) OnRemoteEnpointError(err error) {
	fmt.Errorf("remote endpoint error:%v", err)
}
func (l quicListener) OnTunnuelingError(err error) {
	fmt.Errorf("tunnel  error:%v", err)
}
func (l quicListener) OnTunnelEnd(send int64) {
	fmt.Errorf("tunnel end,read:%v", send)
}

func TestPair(t *testing.T) {
	localConfig := LocalPeerConfig{
		Addr: "localhost:10087",
	}
	remoteConfig := RemotePeerConfig{
		Addr: "localhost:10088",
	}

	lquic, err := quic.ListenAddr(
		remoteConfig.Addr,
		generateTLSConfig(),
		nil,
	)
	if err != nil {
		t.Errorf("fail to listen quic:%v reason:%v", localConfig, err)
	}
	t.Logf("start quic server:%v", remoteConfig)

	ctx := context.TODO()
	pairCancel, err := Pair(ctx, localConfig, remoteConfig, &quicListener{})
	if err != nil {
		t.Errorf("fail to start pairing, reason:%v", err)
	}

	check := []byte("hello kitty")

	c := make(chan bool)
	go startQuicServer(ctx, lquic, t, check, c)

	conn, err := net.Dial("tcp", localConfig.Addr)
	if err != nil {
		t.Errorf("fail to connect local endpoint:%v", localConfig.Addr)
	}
	t.Logf("connect to local:%v", localConfig)

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

func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   PeerQuicProtocol,
	}
}
