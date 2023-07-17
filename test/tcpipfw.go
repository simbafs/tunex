package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"

	"golang.org/x/crypto/ssh"
)

type sshTunnel struct {
	SshConn *ssh.ServerConn
	Addr    string
	Port    uint32
}

var sshConnMap = make(map[string]*sshTunnel)

func main() {
	go StartHHTPServer()

	if err := StartTcpServer(); err != nil {
		fmt.Printf("Oops, there's an error: %v\n", err)
	}
}

func StartHHTPServer() {
	logf := GetLogf("StartHHTPServer")

	s := &http.Server{
		Addr: "localhost:3000",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			subdomain := strings.Split(r.Host, ".")[0]

			tunnel, ok := sshConnMap[subdomain]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "tunnel %s not found", subdomain)
				return
			}

			channel, _, err := tunnel.SshConn.OpenChannel("forwarded-tcpip", ssh.Marshal(struct {
				Addr       string
				Port       uint32
				OriginAddr string
				OriginPort uint32
			}{
				tunnel.Addr,
				tunnel.Port,
				tunnel.SshConn.RemoteAddr().String(),
				uint32(tunnel.SshConn.RemoteAddr().(*net.TCPAddr).Port),
			}))
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "unable to open channel: %v", err)
				logf("unable to open channel: %v", err)
				return
			}
			defer channel.Close()

			logf("channel opened")

			// TODO: connect tcp connection to HTTP connection
		}),
	}

	if err := s.ListenAndServe(); err != nil {
		log.Fatalf("Unable to start HTTP server: %v\n", err)
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

func GetLogf(name string) func(format string, args ...interface{}) {
	return func(format string, args ...interface{}) {
		log.Printf(name+": "+format, args...)
	}
}

func StartTcpServer() error {
	logf := GetLogf("StartTcpServer")
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

	log.Printf("Listening on %s\n", listener.Addr().String())

	for {
		tcpConn, err := listener.AcceptTCP()
		if err != nil {
			logf("Unable to accept connection: %v\n", err)
			continue
		}

		go HandleSSHConnection(tcpConn, sshConf)
	}
}

func HandleSSHConnection(tcpConn *net.TCPConn, sshConf *ssh.ServerConfig) {
	logf := GetLogf("HandleSSHConnection")
	defer func() {
		tcpConn.Close()
		logf("TCP connection from %s closed\n", tcpConn.RemoteAddr())
	}()

	tcpConn.SetKeepAlive(true)

	sshConn, _, reqs, err := ssh.NewServerConn(tcpConn, sshConf)
	if err != nil {
		logf("Unable to handshake: %v\n", err)
		return
	}
	defer func() {
		sshConn.Close()
		logf("SSH connection from %s closed\n", sshConn.RemoteAddr())
	}()

	logf("Connection from %s\n", sshConn.RemoteAddr())

	for req := range reqs {
		switch req.Type {
		case "tcpip-forward":
			HandleTCPForwardRequest(req, sshConn)
		default:
			logf("Global Req: Unknown request: %s\n", req.Type)
		}
	}
}

func HandleTCPForwardRequest(req *ssh.Request, sshConn *ssh.ServerConn) {
	logf := GetLogf("HandleTCPForwardRequest")

	var payload struct {
		Addr string
		Port uint32
	}
	if err := ssh.Unmarshal(req.Payload, &payload); err != nil {
		logf("Unable to unmarshal payload: %v\n", err)
		req.Reply(false, nil)
		return
	}

	logf("tcpip-forward: %s:%d\n", payload.Addr, payload.Port)
	logf("want reply: %v\n", req.WantReply)

	req.Reply(true, nil)

	sshConnMap[payload.Addr] = &sshTunnel{
		SshConn: sshConn,
		Addr:    payload.Addr,
		Port:    payload.Port,
	}

	// for {
	//
	// 	channel, _, err := sshConn.OpenChannel("forwarded-tcpip", ssh.Marshal(struct {
	// 		Addr       string
	// 		Port       uint32
	// 		OriginAddr string
	// 		OriginPort uint32
	// 	}{
	// 		payload.Addr,
	// 		payload.Port,
	// 		sshConn.RemoteAddr().String(),
	// 		uint32(sshConn.RemoteAddr().(*net.TCPAddr).Port),
	// 	}))
	// 	if err != nil {
	// 		logf("Unable to open channel: %v\n", err)
	// 		return
	// 	}
	// 	defer func() {
	// 		channel.Close()
	// 		logf("channel closed")
	// 	}()
	//
	// 	go forwardData(conn, channel)
	// }
}

func bicopyWRer(A io.ReadWriteCloser, B io.ReadWriteCloser) {
	logf := GetLogf("forwardData")

	var once sync.Once
	wait := make(chan int, 0)

	close := func() {
		A.Close()
		B.Close()
		logf("forwardData closed")
		wait <- 1
	}

	// Copy data from local connection to remote channel
	go func() {
		_, err := io.Copy(B, A)
		if err != nil {
			logf("Unable to copy from local to remote: %v\n", err)
		}
		logf("EOF from remote")
		once.Do(close)
	}()

	// Copy data from remote channel to local connection
	go func() {
		_, err := io.Copy(A, B)
		if err != nil {
			logf("Unable to copy from remote to local: %v\n", err)
		}
		logf("EOF from local")
		once.Do(close)
	}()

	<-wait
}
