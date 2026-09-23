package include

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"os"
	"slices"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/service/heimspyinspector"
	"github.com/sagernet/sing/common/bufio"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	sHTTP "github.com/sagernet/sing/protocol/http"
	"github.com/sagernet/sing/service"
)

// The private CONNECT hop preserves client identity across the sing-box ingress.
// All policy, TLS interception and upstream selection remain in heimspy-proxy.
type inspectOptions struct {
	Inspector  string `json:"inspector,omitempty"`
	ServerPort uint16 `json:"server_port"`
	Token      string `json:"token"`
}

type inspectOutbound struct {
	inspectorTag string
	manager      adapter.ServiceManager
	inspector    *heimspyinspector.Service
	outbound.Adapter
	dialer N.Dialer
	server M.Socksaddr
	token  string
}

func newInspectOutbound(ctx context.Context, _ adapter.Router, _ log.ContextLogger, tag string, options inspectOptions) (adapter.Outbound, error) {
	if options.Inspector != "" {
		if options.ServerPort != 0 || options.Token != "" {
			return nil, errors.New("choose an inspector service or a remote inspection endpoint")
		}
		return &inspectOutbound{
			Adapter:      outbound.NewAdapter("heimspy-inspect", tag, []string{N.NetworkTCP, N.NetworkUDP}, nil),
			inspectorTag: options.Inspector, manager: service.FromContext[adapter.ServiceManager](ctx),
		}, nil
	}
	if options.ServerPort == 0 || len(options.Token) < 32 {
		return nil, errors.New("inspection endpoint requires a port and private token")
	}
	d, err := dialer.New(ctx, option.DialerOptions{}, false)
	if err != nil {
		return nil, err
	}
	return &inspectOutbound{
		Adapter: outbound.NewAdapterWithDialerOptions("heimspy-inspect", tag, []string{N.NetworkTCP}, option.DialerOptions{}),
		dialer:  d,
		server:  M.ParseSocksaddrHostPort("127.0.0.1", options.ServerPort),
		token:   options.Token,
	}, nil
}

func (h *inspectOutbound) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateInitialize || h.inspectorTag == "" {
		return nil
	}
	if h.manager == nil {
		return errors.New("inspection service manager is missing")
	}
	item, _ := h.manager.Get(h.inspectorTag)
	inspector, ok := item.(*heimspyinspector.Service)
	if !ok {
		return errors.New("unknown heimspy-inspector service: " + h.inspectorTag)
	}
	h.inspector = inspector
	return nil
}
func (h *inspectOutbound) Close() error { return nil }

func (h *inspectOutbound) DialContext(ctx context.Context, network string, destination M.Socksaddr) (net.Conn, error) {
	if network != N.NetworkTCP {
		return nil, os.ErrInvalid
	}
	metadata := adapter.ContextFrom(ctx)
	if metadata == nil || !metadata.Source.IsValid() {
		return nil, errors.New("inspection connection is missing its source")
	}
	if h.inspectorTag != "" {
		if h.inspector == nil {
			return nil, errors.New("inspection outbound is not started")
		}
		rawHTTP := metadata.InboundType == "heimspy-mixed" && metadata.Protocol == "http"
		return h.inspector.DialContext(ctx, netip.AddrPortFrom(metadata.Source.Addr, metadata.Source.Port), destination.String(), rawHTTP)
	}
	headers := http.Header{
		"X-Heimspy-Token":  []string{h.token},
		"X-Heimspy-Source": []string{metadata.Source.String()},
	}
	if metadata.InboundType == "heimspy-mixed" && metadata.Protocol == "http" {
		headers.Set("X-Heimspy-Protocol", "http")
	}
	client := sHTTP.NewClient(sHTTP.Options{
		Dialer:  h.dialer,
		Server:  h.server,
		Headers: headers,
	})
	return client.DialContext(ctx, network, destination)
}

func (h *inspectOutbound) ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error) {
	return nil, os.ErrInvalid
}

// UDP flows enter the inspector directly, without a TCP CONNECT envelope.
func (h *inspectOutbound) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, onClose N.CloseHandlerFunc) {
	go func() {
		var err error
		defer func() {
			_ = conn.Close()
			if onClose != nil {
				onClose(err)
			}
		}()
		if h.inspector == nil {
			err = errors.New("UDP inspection requires an in-process inspector")
			return
		}
		source := netip.AddrPortFrom(metadata.Source.Addr, metadata.Source.Port)
		packets := &inspectionPacketConn{
			PacketConn: bufio.NewNetPacketConn(conn),
			source:     net.UDPAddrFromAddrPort(source), destination: metadata.Destination,
		}
		err = h.inspector.ServePacket(adapter.WithContext(ctx, &metadata), packets, source, metadata.Destination.String(), metadata.Domain, slices.Contains(metadata.ALPN, "h3"))
	}()
}

// sing's routed packet readers report the destination; a QUIC server expects
// ReadFrom to report its client and replies to retain the original destination.
type inspectionPacketConn struct {
	net.PacketConn
	source      *net.UDPAddr
	destination M.Socksaddr
}

func (c *inspectionPacketConn) ReadFrom(p []byte) (int, net.Addr, error) {
	n, _, err := c.PacketConn.ReadFrom(p)
	return n, c.source, err
}

func (c *inspectionPacketConn) WriteTo(p []byte, _ net.Addr) (int, error) {
	return c.PacketConn.WriteTo(p, c.destination)
}
