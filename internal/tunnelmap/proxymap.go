package tunnelmap

import (
	"net"

	"golang.org/x/crypto/ssh"
)

type SSHTunnel struct {
	Name       string
	ServerConn *ssh.ServerConn
}

func (t *SSHTunnel) OpenChannel() (ssh.Channel, error) {
	channel, _, err := t.ServerConn.OpenChannel("forwarded-tcpip", ssh.Marshal(struct {
		Addr       string
		Port       uint32
		OriginAddr string
		OriginPort uint32
	}{
		t.Name,
		80,
		t.ServerConn.RemoteAddr().String(),
		uint32(t.ServerConn.RemoteAddr().(*net.TCPAddr).Port),
	}))
	return channel, err
}

type TunnelMap map[string]*SSHTunnel

func (t *TunnelMap) Add(name string, tunnel *ssh.ServerConn) {
	(*t)[name] = &SSHTunnel{
		Name:       name,
		ServerConn: tunnel,
	}
}

func (t *TunnelMap) Get(name string) (*SSHTunnel, bool) {
	tunnel, ok := (*t)[name]
	return tunnel, ok
}

func (t *TunnelMap) Del(name string) {
	delete(*t, name)
}
