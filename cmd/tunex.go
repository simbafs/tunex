package main

import (
	"tunex/internal/reverseproxy"
	"tunex/internal/sshserver"
	"tunex/internal/tunnelmap"
)

var tunnelMap = make(tunnelmap.TunnelMap)

func GorutineErr(fn ...func() error) error {
	errCh := make(chan error)
	for _, f := range fn {
		go func(f func() error) {
			errCh <- f()
		}(f)
	}
	for range fn {
		if err := <-errCh; err != nil {
			return err
		}
	}
	return nil
}

func main() {
	rp := reverseproxy.NewReverseProxy("0.0.0.0", 3000, &tunnelMap)
	ss := sshserver.NewSSHServer("0.0.0.0", 2222, &tunnelMap)

	if err := GorutineErr(rp.Start, ss.Start); err != nil {
		panic(err)
	}
}
