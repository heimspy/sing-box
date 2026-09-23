package proxy

import "context"

type inboundControlKey struct{}

// InboundControl changes window listeners on the sing-box manager.
type InboundControl func(action, tag string, port int) (int, error)

func WithInboundControl(ctx context.Context, handler InboundControl) context.Context {
	return context.WithValue(ctx, inboundControlKey{}, handler)
}
