package socks

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/canceler"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/protocol/socks"
)

func TestBindPacketClient(t *testing.T) {
	server, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = server.Close() })
	client, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if err := server.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := client.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatal(err)
	}
	// SOCKS5 UDP header for 127.0.0.1:443, followed by a one-byte payload.
	wire := []byte{0, 0, 0, 1, 127, 0, 0, 1, 1, 187, 'x'}
	if _, err := client.WriteTo(wire, server.LocalAddr()); err != nil {
		t.Fatal(err)
	}
	control, peer := net.Pipe()
	t.Cleanup(func() { _ = peer.Close() })
	associate := socks.NewAssociatePacketConn(bufio.NewServerPacketConn(server), M.Socksaddr{}, control)
	t.Cleanup(func() { _ = associate.Close() })
	first := buf.NewPacket()
	destination, err := associate.ReadPacket(first)
	if err != nil {
		first.Release()
		t.Fatal(err)
	}
	_, timed := canceler.NewTimeoutPacketConn(t.Context(), associate, 5*time.Second)
	cached := bufio.NewCachedPacketConn(timed, first, destination)
	bound := bindPacketClient(cached)
	t.Cleanup(func() { _ = bound.Close() })
	if bound != cached || cached.PacketConn != timed || timed.Timeout() != 5*time.Second {
		t.Fatal("binding replaced the cache or timeout wrapper")
	}
	timeout, ok := timed.(*canceler.TimeoutPacketConn)
	if !ok {
		t.Fatalf("unexpected timeout wrapper: %T", timed)
	}
	boundAssociate, ok := timeout.PacketConn.(*socks.AssociatePacketConn)
	if !ok || boundAssociate == associate {
		t.Fatalf("UDP association was not rebound: %T", timeout.PacketConn)
	}
	wrapped, ok := boundAssociate.Upstream().(*bufio.ExtendedConnWrapper)
	if !ok {
		t.Fatalf("unexpected association wrapper: %T", boundAssociate.Upstream())
	}
	if fmt.Sprintf("%T", wrapped.Upstream()) != fmt.Sprintf("%T", bufio.NewBindPacketConn(server, client.LocalAddr())) {
		t.Fatalf("UDP reply endpoint is not bound: %T", wrapped.Upstream())
	}
	packets := bufio.NewNetPacketConn(bound)
	data := make([]byte, 64)
	n, addr, err := packets.ReadFrom(data)
	if err != nil || string(data[:n]) != "x" || M.SocksaddrFromNet(addr) != destination {
		t.Fatalf("cached first packet lost: n=%d addr=%v err=%v", n, addr, err)
	}
	// Exercise independent readers and writers, as QUIC does. With the old
	// serverPacketConn, -race reports remoteAddr writes racing with replies.
	const count = 100
	results := make(chan error, 4)
	for _, action := range []func() error{
		func() error { _, err := client.WriteTo(wire, server.LocalAddr()); return err },
		func() error {
			data := make([]byte, 64)
			n, _, err := packets.ReadFrom(data)
			if err == nil && string(data[:n]) != "x" {
				return fmt.Errorf("invalid request: %q", data[:n])
			}
			return err
		},
		func() error { _, err := packets.WriteTo([]byte("x"), destination); return err },
		func() error {
			data := make([]byte, 64)
			n, _, err := client.ReadFrom(data)
			if err == nil && string(data[:n]) != string(wire) {
				return fmt.Errorf("invalid reply: %x", data[:n])
			}
			return err
		},
	} {
		go func() {
			for range count {
				if err := action(); err != nil {
					results <- err
					return
				}
			}
			results <- nil
		}()
	}
	for range 4 {
		if err := <-results; err != nil {
			t.Error(err)
		}
	}
	_ = bound.Close()
	if _, err := peer.Write([]byte("closed")); err == nil {
		t.Fatal("binding lost TCP control connection ownership")
	}
}
