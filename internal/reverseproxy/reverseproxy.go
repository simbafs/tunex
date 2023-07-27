package reverseproxy

import (
	"fmt"
	"net/http"
	"strings"
	"tunex/internal/logf"
	"tunex/internal/tcphttp"
	"tunex/internal/tunnelmap"
)

type ReverseProxy struct {
	Addr      string
	Port      int
	TunnelMap *tunnelmap.TunnelMap
}

func NewReverseProxy(addr string, port int, tunnelMap *tunnelmap.TunnelMap) *ReverseProxy {
	return &ReverseProxy{
		Addr:      addr,
		Port:      port,
		TunnelMap: tunnelMap,
	}
}

func (rp *ReverseProxy) Start() error {
	logf := logf.New("StartReverseProxy")

	logf("Listening http on %s:%d", rp.Addr, rp.Port)

	if err := http.ListenAndServe(fmt.Sprintf("%s:%d", rp.Addr, rp.Port), rp); err != nil {
		return fmt.Errorf("unable to listen: %w", err)
	}
	return nil
}

func (rp *ReverseProxy) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	logf := logf.New("ServeHTTP")

	host := strings.Split(req.Host, ".")[0]
	logf("request: %s, host: %s", req.URL, host)

	if to, ok := rp.TunnelMap.Get(host); ok {
		logf("tunnel: %s", to.Name)
		// fmt.Fprintf(wr, "tunnel: %s", to.Name)
		rp.handleProxy(wr, req, to)
		return
	}

	fmt.Fprintf(wr, "not found")
}

func (rp *ReverseProxy) handleProxy(wr http.ResponseWriter, req *http.Request, tunnel *tunnelmap.SSHTunnel) {
	logf := logf.New("handleProxy")

	sshChannel, err := tunnel.OpenChannel()
	if err != nil {
		logf("unable to open channel: %v", err)
		return
	}
	defer func() {
		sshChannel.Close()
		logf("ssh channel closed")
	}()

	client := tcphttp.GetClient(&sshChannel, tunnel.ServerConn.RemoteAddr())

	DoProxy(DoProxyConfig{
		client:  client,
		to:      "localhost:3000",
		isHttps: false,
		wr:      wr,
		req:     req,
	})
}
