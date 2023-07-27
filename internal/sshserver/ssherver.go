package sshserver

import (
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"tunex/internal/logf"
	"tunex/internal/tunnelmap"

	"golang.org/x/crypto/ssh"
)

func CompareKey(key ssh.PublicKey, pubKeyStr string) bool {
	// compare two keys
	pubKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubKeyStr))
	if err != nil {
		return false
	}

	return ssh.FingerprintSHA256(key) == ssh.FingerprintSHA256(pubKey)
}

type SSHServer struct {
	Addr        string
	Port        int
	TunnelMap   *tunnelmap.TunnelMap
	AllowedUser map[string][]string
}

func NewSSHServer(addr string, port int, tunnelMap *tunnelmap.TunnelMap) *SSHServer {
	return &SSHServer{
		Addr:      addr,
		Port:      port,
		TunnelMap: tunnelMap,
		AllowedUser: map[string][]string{
			"simba": {
				"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHcLVJDmYggMFXJ3CqMOSMnBkkDX1982cdd3rmRqfpMC simba@simba-nb",
			},
		},
	}
}

func (s *SSHServer) Start() error {
	logf := logf.New("StartTcpServer")

	private, err := GetHostKey("./key/host")
	if err != nil {
		return fmt.Errorf("unable to read private key: %w", err)
	}

	sshConf := &ssh.ServerConfig{
		NoClientAuth: false,
		PublicKeyCallback: func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			// log.Printf("key: %s\n", ssh.FingerprintSHA256(key))
			// find if the public key is in the allowed list
			for user, keys := range s.AllowedUser {
				for _, pubKey := range keys {
					if CompareKey(key, pubKey) {
						log.Printf("User %q authenticated with key %s\n", user, ssh.FingerprintSHA256(key))

						return &ssh.Permissions{
							Extensions: map[string]string{
								"user":  user,
								"pk-fp": ssh.FingerprintSHA256(key),
							},
						}, nil
					}
				}
			}
			return &ssh.Permissions{
				Extensions: map[string]string{
					"user":  "anonymous",
					"pk-fp": "anonymous",
				},
			}, nil
		},
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
		defer func() {
			tcpConn.Close()
			logf("TCP connection from %s closed\n", tcpConn.RemoteAddr())
		}()

		tcpConn.SetKeepAlive(true)

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

	logf("Extensions: %v", sshConn.Permissions.Extensions)

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
