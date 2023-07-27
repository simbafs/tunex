package tcphttp

import (
	"net"
	"net/http"
	"time"

	"golang.org/x/crypto/ssh"
)

type connDialer struct {
	c          ssh.Channel
	remoteAddr net.Addr
}

func (cd connDialer) Dial(network, addr string) (net.Conn, error) {
	return netConnWrap{
		cd.c,
		cd.remoteAddr,
	}, nil
}

type addr struct {
	addr string
}

func (a addr) Network() string {
	return "tcp"
}

func (a addr) String() string {
	return a.addr
}

type netConnWrap struct {
	ssh.Channel
	remoteAddr net.Addr
}

func (ncw netConnWrap) LocalAddr() net.Addr {
	return addr{addr: "0.0.0.0"}
}

func (ncw netConnWrap) RemoteAddr() net.Addr {
	return ncw.remoteAddr
}

func (ncw netConnWrap) SetDeadline(t time.Time) error {
	return nil
}

func (ncw netConnWrap) SetReadDeadline(t time.Time) error {
	return nil
}

func (ncw netConnWrap) SetWriteDeadline(t time.Time) error {
	return nil
}

func GetClient(conn *ssh.Channel, remoteAddr net.Addr) *http.Client {
	client := &http.Client{
		Transport: &http.Transport{
			Dial: connDialer{*conn, remoteAddr}.Dial,
		},
	}

	return client
}
