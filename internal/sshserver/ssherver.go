package sshserver

import (
	"fmt"
	"io/ioutil"
	"net"
	"tunex/internal/logf"
	"tunex/internal/tunnelmap"

	"golang.org/x/crypto/ssh"
)

type SSHServer struct {
	Addr      string
	Port      int
	TunnelMap *tunnelmap.TunnelMap
}

func NewSSHServer(addr string, port int, tunnelMap *tunnelmap.TunnelMap) *SSHServer {
	return &SSHServer{
		Addr:      addr,
		Port:      port,
		TunnelMap: tunnelMap,
	}
}

func (s *SSHServer) Start() error {
	logf := logf.New("StartTcpServer")

	private, err := GetHostKey("./key/host")
	if err != nil {
		return fmt.Errorf("unable to read private key: %w", err)
	}

	sshConf := &ssh.ServerConfig{
		NoClientAuth: true,
	}
	sshConf.AddHostKey(private)

	listener, err := net.ListenTCP("tcp", &net.TCPAddr{
		IP:   net.IPv4(0, 0, 0, 0),
		Port: 2222,
	})
	if err != nil {
		return fmt.Errorf("unable to listen: %w", err)
	}
	defer func() {
		listener.Close()
		logf("TCP listener closed")
	}()

	logf("Listening ssh on %s\n", listener.Addr().String())

	for {
		tcpConn, err := listener.AcceptTCP()
		if err != nil {
			logf("Unable to accept connection: %v\n", err)
			continue
		}

		go s.handleSSHConnection(tcpConn, sshConf)
	}
}

func GetHostKey(keyPath string) (ssh.Signer, error) {
	privateBytes, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		return nil, err
	}

	return private, nil
}

func (s *SSHServer) handleSSHConnection(tcpConn *net.TCPConn, sshConf *ssh.ServerConfig) {
	logf := logf.New("HandleSSHConnection")
	defer func() {
		tcpConn.Close()
		logf("TCP connection from %s closed\n", tcpConn.RemoteAddr())
	}()

	tcpConn.SetKeepAlive(true)

	name := ""

	sshConn, _, reqs, err := ssh.NewServerConn(tcpConn, sshConf)
	if err != nil {
		logf("Unable to handshake: %v\n", err)
		return
	}
	defer func() {
		sshConn.Close()
		logf("SSH connection from %s closed\n", sshConn.RemoteAddr())
		s.TunnelMap.Del(name)
	}()

	logf("Connection from %s\n", sshConn.RemoteAddr())

	for req := range reqs {
		switch req.Type {
		case "tcpip-forward":
			s.handleTCPForwardRequest(req, sshConn, &name)
		default:
			logf("Global Req: Unknown request: %s\n", req.Type)
		}
	}
}

func (s *SSHServer) handleTCPForwardRequest(req *ssh.Request, sshConn *ssh.ServerConn, name *string) {
	logf := logf.New("HandleTCPForwardRequest")

	var payload struct {
		Addr string
		Port uint32
	}
	if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
		logf("Unable to unmarshal payload: %v\n", err)
		req.Reply(false, nil)
		return
	}

	logf("tcpip-forward: %s:%d want reply: %v\n", payload.Addr, payload.Port, req.WantReply)

	*name = payload.Addr

	req.Reply(true, nil)

	s.TunnelMap.Add(payload.Addr, sshConn)
}
