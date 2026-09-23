// Package heimspyinspector embeds Heimspy's HTTP inspection engine in sing-box.
package heimspyinspector

import (
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"sync/atomic"

	"github.com/sagernet/sing-box/adapter"
	boxService "github.com/sagernet/sing-box/adapter/service"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/service/heimspyinspector/internal/proxy"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

const Type = "heimspy-inspector"

type Options struct {
	PacketEgress string `json:"packet_egress,omitempty"`
}

type control struct {
	input    io.ReadCloser
	output   io.WriteCloser
	shutdown context.CancelFunc
	claimed  atomic.Bool
}

// WithControl grants one inspector service exclusive ownership of the inherited
// IPC streams. EOF or failure requests shutdown of the owning sing-box instance.
func WithControl(ctx context.Context, input io.ReadCloser, output io.WriteCloser, shutdown context.CancelFunc) context.Context {
	return service.ContextWith(ctx, &control{input: input, output: output, shutdown: shutdown})
}

func Register(registry *boxService.Registry) {
	boxService.Register[Options](registry, Type, New)
}

type Service struct {
	boxService.Adapter
	ctx          context.Context
	logger       log.ContextLogger
	engine       *proxy.Engine
	done         chan struct{}
	packetEgress string
}

func New(ctx context.Context, logger log.ContextLogger, tag string, options Options) (adapter.Service, error) {
	return &Service{Adapter: boxService.NewAdapter(Type, tag), ctx: ctx, logger: logger, packetEgress: options.PacketEgress}, nil
}

func (s *Service) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateInitialize {
		return nil
	}
	c := service.FromContext[*control](s.ctx)
	if c == nil || c.input == nil || c.output == nil || c.shutdown == nil {
		return errors.New("heimspy-inspector requires an exclusive IPC control channel")
	}
	if !c.claimed.CompareAndSwap(false, true) {
		return errors.New("only one heimspy-inspector may own the IPC control channel")
	}
	var packetDialer func(context.Context, string) (net.Conn, error)
	if s.packetEgress != "" {
		manager := service.FromContext[adapter.OutboundManager](s.ctx)
		packetDialer = func(ctx context.Context, destination string) (net.Conn, error) {
			if manager == nil {
				return nil, errors.New("packet egress manager is missing")
			}
			egress, ok := manager.Outbound(s.packetEgress)
			if !ok || egress.Type() == "heimspy-inspect" {
				return nil, errors.New("invalid inspector packet egress")
			}
			return egress.DialContext(ctx, N.NetworkUDP, M.ParseSocksaddr(destination))
		}
	}
	s.engine = proxy.NewEngine(proxy.WithInboundControl(s.ctx, s.controlInbound), c.input, c.output, packetDialer)
	s.done = make(chan struct{})
	go func() {
		defer close(s.done)
		if err := s.engine.Wait(); err != nil && s.ctx.Err() == nil {
			s.logger.Error("inspection stopped: ", err)
		}
		c.shutdown()
	}()
	return nil
}

func (s *Service) Close() error {
	if s.engine == nil {
		return nil
	}
	err := s.engine.Close()
	<-s.done
	return err
}

func (s *Service) DialContext(ctx context.Context, source netip.AddrPort, destination string, rawHTTP bool) (net.Conn, error) {
	if s.engine == nil {
		return nil, errors.New("heimspy-inspector is not started")
	}
	return s.engine.DialContext(ctx, source, destination, rawHTTP)
}

func (s *Service) ServePacket(ctx context.Context, conn net.PacketConn, source netip.AddrPort, destination, serverName string, http3 bool) error {
	if s.engine == nil {
		return errors.New("heimspy-inspector is not started")
	}
	return s.engine.ServePacket(ctx, conn, source, destination, serverName, http3)
}
