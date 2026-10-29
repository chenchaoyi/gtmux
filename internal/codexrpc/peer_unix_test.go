//go:build darwin || linux

package codexrpc

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestUnixPeerIdentityComesFromKernel(t *testing.T) {
	dir, err := os.MkdirTemp("", "gcp-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	listener, err := net.Listen("unix", filepath.Join(dir, "s"))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan net.Conn, 1)
	go func() {
		c, err := listener.Accept()
		if err == nil {
			accepted <- c
		}
	}()
	client, err := net.Dial("unix", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	server := <-accepted
	defer server.Close()
	for _, c := range []net.Conn{client, server} {
		pid, ok := LocalPeerPID(c)
		if !ok || pid != os.Getpid() {
			t.Fatal("wrong Unix peer", pid, ok)
		}
	}
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	if _, ok := LocalPeerPID(a); ok {
		t.Fatal("non-Unix transport gained local authority")
	}
}
