//go:build with_heimspy

package include

import (
	"context"
	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter/certificate"
	"github.com/sagernet/sing-box/adapter/endpoint"
	"github.com/sagernet/sing-box/adapter/inbound"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/adapter/service"
	"github.com/sagernet/sing-box/dns"
	"github.com/sagernet/sing-box/dns/transport"
	"github.com/sagernet/sing-box/dns/transport/fakeip"
	"github.com/sagernet/sing-box/dns/transport/local"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/protocol/direct"
	"github.com/sagernet/sing-box/protocol/http"
	"github.com/sagernet/sing-box/protocol/socks"
	"github.com/sagernet/sing-box/protocol/tun"
	"github.com/sagernet/sing-box/service/heimspyinspector"
)

// Context retains the upstream entry point with only Heimspy's transport capabilities.
func Context(ctx context.Context) context.Context {
	return box.Context(ctx, InboundRegistry(), OutboundRegistry(), EndpointRegistry(), DNSTransportRegistry(), ServiceRegistry(), CertificateProviderRegistry())
}
func InboundRegistry() *inbound.Registry {
	registry := inbound.NewRegistry()
	tun.RegisterInbound(registry)
	http.RegisterInbound(registry)
	socks.RegisterInbound(registry)
	direct.RegisterInbound(registry)
	inbound.Register[option.SocksInboundOptions](registry, "heimspy-mixed", newProxyInbound)
	return registry
}
func OutboundRegistry() *outbound.Registry {
	registry := outbound.NewRegistry()
	direct.RegisterOutbound(registry)
	http.RegisterOutbound(registry)
	socks.RegisterOutbound(registry)
	outbound.Register[inspectOptions](registry, "heimspy-inspect", newInspectOutbound)
	return registry
}
func DNSTransportRegistry() *dns.TransportRegistry {
	registry := dns.NewTransportRegistry()
	local.RegisterTransport(registry)
	fakeip.RegisterTransport(registry)
	transport.RegisterUDP(registry)
	transport.RegisterTCP(registry)
	transport.RegisterTLS(registry)
	transport.RegisterHTTPS(registry)
	return registry
}
func EndpointRegistry() *endpoint.Registry { return endpoint.NewRegistry() }
func ServiceRegistry() *service.Registry {
	registry := service.NewRegistry()
	heimspyinspector.Register(registry)
	return registry
}
func CertificateProviderRegistry() *certificate.Registry { return certificate.NewRegistry() }
