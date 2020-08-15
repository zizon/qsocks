package peer

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"
	"math/big"

	quic "github.com/lucas-clemente/quic-go"
)

// ServerPeer server side of quic peering
type ServerPeer interface {
	io.Closer
	PollNewChannel() (io.ReadWriteCloser, error)
}

// ServerPeerConfig config for generateing new server peer
type ServerPeerConfig struct {
	quic.Config
	Addr string
	ContextErrorAggregator
}

type serverPeer struct {
	context.Context
	quic.Listener
	streams chan quic.Stream
	ContextErrorAggregator
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

// NewServerPeer create new server side peer
func NewServerPeer(ctx context.Context, serverPeerConfig ServerPeerConfig) (ServerPeer, error) {
	lquic, err := quic.ListenAddr(
		serverPeerConfig.Addr,
		generateTLSConfig(),
		&serverPeerConfig.Config,
	)
	if err != nil {
		return nil, err
	}

	peer := serverPeer{
		ctx,
		lquic,
		make(chan quic.Stream),
		serverPeerConfig,
	}

	go peer.serveQuic()

	go func() {
		if block := peer.Done(); block != nil {
			defer peer.Close()
			<-block
		}
	}()
	return peer, nil
}

type peerSession struct {
	quic.Session
	context.Context
	streams chan quic.Stream
	ContextErrorAggregator
}

func (peer serverPeer) serveQuic() {
	defer peer.Close()

	for {
		session, err := peer.Accept(peer.Context)
		if err != nil {
			peer.Collect(err)
			break
		}

		go peerSession{
			session,
			peer.Context,
			peer.streams,
			peer,
		}.serveQuicSession()
	}
}

// Close close server peer
func (peer serverPeer) Close() error {
	close(peer.streams)

	if err := peer.Listener.Close(); err != nil {
		peer.Collect(err)
	}

	return nil
}

func (session peerSession) Close() {
	if err := session.CloseWithError(0, "close"); err != nil {
		session.Collect(err)
	}
}

func (session peerSession) serveQuicSession() {
	defer session.Close()

	for {
		select {
		case <-session.Done():
			if err := session.Context.Err(); err != nil {
				session.Collect(err)
			}
			break
		default:
		}

		stream, err := session.AcceptStream(session.Context)
		if err != nil {
			session.Collect(err)
			break
		}

		session.streams <- stream
	}
}

// PollNewChannel poll a new channel
func (peer serverPeer) PollNewChannel() (io.ReadWriteCloser, error) {
	return <-peer.streams, nil
}
