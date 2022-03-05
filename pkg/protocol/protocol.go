package protocol

import (
	"encoding/binary"
	"io"
	"net"
)

/*
Auth request of socks5 protocol
+----+----------+----------+
|VER | NMETHODS | METHODS  |
+----+----------+----------+
| 1  |    1     | 1 to 255 |
+----+----------+----------+
*/
type Auth struct {
	VER      byte
	NMETHODS byte
	METHODS  []byte
}

// Decode decode from reader
func (auth *Auth) Decode(r io.Reader) error {
	buf := make([]byte, 2)
	_, err := io.ReadFull(r, buf)
	if err != nil {
		return err
	}

	auth.VER = buf[0]
	auth.NMETHODS = buf[1]
	buf = make([]byte, int(auth.NMETHODS))
	_, err = io.ReadFull(r, buf)
	if err != nil {
		return err
	}

	return nil
}

/*
AuthReply socks5 auth reply
+----+--------+
|VER | METHOD |
+----+--------+
| 1  |   1    |
+----+--------+
*/
type AuthReply struct {
	VER    byte
	METHOD byte
}

// Encode encode into writer
func (reply AuthReply) Encode(w io.Writer) error {
	_, err := w.Write([]byte{0x05, 0x00})
	return err
}

func (reply *AuthReply) Decode(r io.Reader) error {
	buf := make([]byte, 2)
	if _, err := io.ReadFull(r, buf); err != nil {
		return err
	}
	reply.VER = buf[0]
	reply.METHOD = buf[1]
	return nil
}

/*
Request socks5 reqeust
+----+-----+-------+------+----------+----------+
|VER | CMD |  RSV  | ATYP | DST.ADDR | DST.PORT |
+----+-----+-------+------+----------+----------+
| 1  |  1  | X'00' |  1   | Variable |    2     |
+----+-----+-------+------+----------+----------+
*/
type Request struct {
	VER  byte
	CMD  byte
	RSV  byte
	ATYP byte
	HOST []byte
	PORT int
}

// Decode decode from reader
func (request *Request) Decode(r io.Reader) error {
	buf := make([]byte, 4)
	if _, err := io.ReadFull(r, buf); err != nil {
		return err
	}

	request.VER = buf[0]
	request.CMD = buf[1]
	request.RSV = buf[2]
	request.ATYP = buf[3]

	switch request.ATYP {
	case 0x01:
		buf = make([]byte, net.IPv4len)
		if _, err := io.ReadFull(r, buf); err != nil {
			return err
		}
		request.HOST = buf
	case 0x03:
		length := make([]byte, 1)
		if _, err := io.ReadFull(r, length); err != nil {
			return err
		}
		buf = make([]byte, int(length[0]))
		if _, err := io.ReadFull(r, buf); err != nil {
			return err
		}
		request.HOST = buf
	case 0x04:
		buf = make([]byte, net.IPv6len)
		if _, err := io.ReadFull(r, buf); err != nil {
			return err
		}
		request.HOST = buf
	}

	buf = make([]byte, 2)
	if _, err := io.ReadFull(r, buf); err != nil {
		return err
	}
	request.PORT = int(binary.BigEndian.Uint16(buf))
	return nil
}

/*
Reply socks5 reply
+----+-----+-------+------+----------+----------+
|VER | REP |  RSV  | ATYP | BND.ADDR | BND.PORT |
+----+-----+-------+------+----------+----------+
| 1  |  1  | X'00' |  1   | Variable |    2     |
+----+-----+-------+------+----------+----------+
*/
type Reply struct {
	VER  byte
	REP  byte
	RSV  byte
	ATYP byte
	HOST []byte
	PORT int
}

// Encode encode into writer
func (reply Reply) Encode(w io.Writer) error {
	buf := []byte{
		0x05, 0x00, 0x00,
		0x03,
		byte(len(reply.HOST)),
	}
	buf = append(buf, reply.HOST...)
	if _, err := w.Write(buf); err != nil {
		return err
	}

	binary.BigEndian.PutUint16(buf[:2], uint16(reply.PORT))
	if _, err := w.Write(buf[:2]); err != nil {
		return err
	}

	return nil
}

/*
UDPRequest socks5 upd request
+----+------+------+----------+----------+----------+
|RSV | FRAG | ATYP | DST.ADDR | DST.PORT |   DATA   |
+----+------+------+----------+----------+----------+
| 2  |  1   |  1   | Variable |    2     | Variable |
+----+------+------+----------+----------+----------+
*/
type UDPRequest struct {
	RSV  uint16
	FRAG byte
	ATYP byte
	DST  net.IP
	HOST []byte
	PORT int
	DATA []byte
}
