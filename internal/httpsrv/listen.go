package httpsrv

import (
	"context"
	"net"
)

// listenTCP opens a TCP listener with Multipath TCP explicitly disabled.
//
// Go 1.24+ enables MPTCP on Linux listen sockets by default. MPTCP listeners are
// the exposure path behind CVE-2026-98432 (kernel IPv4 TCP use-after-free).
// SetMultipathTCP(false) overrides GODEBUG so an environment cannot re-enable
// MPTCP for this listener. Patching the kernel remains the actual fix.
func listenTCP(addr string) (net.Listener, error) {
	if addr == "" {
		addr = ":http"
	}
	var lc net.ListenConfig
	lc.SetMultipathTCP(false)
	return lc.Listen(context.Background(), "tcp", addr)
}
