package heimspyinspector

import (
	"errors"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json/badoption"
	"github.com/sagernet/sing/service"
	"net/netip"
	"strings"
)

func (s *Service) controlInbound(action, tag string, port int) (int, error) {
	if !strings.HasPrefix(tag, "heimspy-") || len(tag) > 128 || port < 0 || port > 65535 {
		return 0, errors.New("invalid window inlet")
	}
	manager := service.FromContext[adapter.InboundManager](s.ctx)
	if manager == nil {
		return 0, errors.New("missing inbound manager")
	}
	if action == "inbound-remove" {
		return 0, manager.Remove(tag)
	}
	if _, exists := manager.Get(tag); exists {
		return 0, errors.New("window inlet already exists")
	}
	address := badoption.Addr(netip.MustParseAddr("127.0.0.1"))
	options := &option.SocksInboundOptions{ListenOptions: option.ListenOptions{Listen: &address, ListenPort: uint16(port)}}
	err := manager.Create(s.ctx, service.FromContext[adapter.Router](s.ctx), s.logger, tag, "heimspy-mixed", options)
	if err != nil {
		return 0, err
	}
	inlet, ok := manager.Get(tag)
	if !ok {
		return 0, errors.New("window inlet missing after creation")
	}
	bound, ok := inlet.(interface{ ListenPort() int })
	if !ok {
		_ = manager.Remove(tag)
		return 0, errors.New("window inlet cannot report its port")
	}
	return bound.ListenPort(), nil
}
