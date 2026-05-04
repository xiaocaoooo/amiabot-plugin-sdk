package transport

import (
	"context"
	"encoding/json"
	"net/rpc"

	"github.com/hashicorp/go-plugin"
	"github.com/xiaocaoooo/amiabot-plugin-sdk/onebot/ob11"
	papi "github.com/xiaocaoooo/amiabot-plugin-sdk/plugin"
)

// Context keys for self_id and trace_id
type contextKey string

const (
	contextKeySelfID  contextKey = "nyanyabot_self_id"
	contextKeyTraceID contextKey = "nyanyabot_trace_id"
)

// WithSelfID injects self_id into context
func WithSelfID(ctx context.Context, selfID int64) context.Context {
	return context.WithValue(ctx, contextKeySelfID, selfID)
}

// GetSelfID extracts self_id from context, returns 0 if not found
func GetSelfID(ctx context.Context) int64 {
	if ctx == nil {
		return 0
	}
	if v := ctx.Value(contextKeySelfID); v != nil {
		if selfID, ok := v.(int64); ok {
			return selfID
		}
	}
	return 0
}

// WithTraceID injects trace_id into context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, contextKeyTraceID, traceID)
}

// GetTraceID extracts trace_id from context, returns empty string if not found
func GetTraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v := ctx.Value(contextKeyTraceID); v != nil {
		if traceID, ok := v.(string); ok {
			return traceID
		}
	}
	return ""
}

// ===== Plugin-side service (called by host) =====

type PluginRPCServer struct {
	Impl papi.Plugin
	B    *plugin.MuxBroker
}

func (s *PluginRPCServer) Describe(_ struct{}, resp *DescribeReply) error {
	d, err := s.Impl.Descriptor(context.Background())
	if err != nil {
		return err
	}
	*resp = d
	return nil
}

func (s *PluginRPCServer) Configure(args ConfigureArgs, _ *struct{}) error {
	return s.Impl.Configure(context.Background(), args.Config)
}

func (s *PluginRPCServer) Invoke(args InvokeArgs, resp *InvokeReply) error {
	result, err := s.Impl.Invoke(context.Background(), args.Method, args.Params, args.CallerPluginID)
	if err != nil {
		resp.Error = papi.NormalizeStructuredError(err, papi.ErrorCodeInternal)
		return nil
	}
	resp.Result = result
	resp.Error = nil
	return nil
}

func (s *PluginRPCServer) Handle(args HandleArgs, resp *HandleReply) error {
	// 从事件 JSON 中提取 self_id
	var selfID int64
	var eventData map[string]interface{}
	if err := json.Unmarshal(args.EventRawJSON, &eventData); err == nil {
		if selfIDVal, ok := eventData["self_id"]; ok {
			switch v := selfIDVal.(type) {
			case float64:
				selfID = int64(v)
			case int64:
				selfID = v
			case int:
				selfID = int64(v)
			case json.Number:
				if i, err := v.Int64(); err == nil {
					selfID = i
				}
			}
		}
	}

	// 构建带有 self_id 的 context（SDK 版本暂不处理 trace_id）
	ctx := context.Background()
	if selfID != 0 {
		ctx = WithSelfID(ctx, selfID)
	}

	r, err := s.Impl.Handle(ctx, args.ListenerID, args.EventRawJSON, args.Match)
	if err != nil {
		return err
	}
	*resp = r
	return nil
}

func (s *PluginRPCServer) Shutdown(_ struct{}, _ *struct{}) error {
	return s.Impl.Shutdown(context.Background())
}

type AttachHostArgs struct {
	BrokerID uint32 `json:"broker_id"`
}

func (s *PluginRPCServer) AttachHost(args AttachHostArgs, _ *struct{}) error {
	if s.B == nil {
		return nil
	}
	conn, err := s.B.Dial(args.BrokerID)
	if err != nil {
		return err
	}
	hc := &HostRPCClient{client: rpc.NewClient(conn)}
	SetHost(hc)
	return nil
}

// PluginRPCClient is used on the host to call into the plugin.
type PluginRPCClient struct {
	client *rpc.Client
	broker *plugin.MuxBroker
}

var hostClient *HostRPCClient

func SetHost(c *HostRPCClient) { hostClient = c }

func Host() *HostRPCClient { return hostClient }

func (c *PluginRPCClient) Descriptor(ctx context.Context) (papi.Descriptor, error) {
	return c.Describe(ctx)
}

func (c *PluginRPCClient) Describe(ctx context.Context) (papi.Descriptor, error) {
	_ = ctx
	var resp DescribeReply
	if err := c.client.Call("Plugin.Describe", struct{}{}, &resp); err != nil {
		return papi.Descriptor{}, err
	}
	return resp, nil
}

func (c *PluginRPCClient) Invoke(ctx context.Context, method string, paramsJSON json.RawMessage, callerPluginID string) (json.RawMessage, error) {
	_ = ctx
	var resp InvokeReply
	args := InvokeArgs{Method: method, Params: paramsJSON, CallerPluginID: callerPluginID}
	if err := c.client.Call("Plugin.Invoke", args, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	return resp.Result, nil
}

func (c *PluginRPCClient) Handle(ctx context.Context, listenerID string, eventRaw ob11.Event, match *papi.CommandMatch) (papi.HandleResult, error) {
	_ = ctx
	var resp HandleReply
	args := HandleArgs{ListenerID: listenerID, EventRawJSON: json.RawMessage(eventRaw), Match: match}
	if err := c.client.Call("Plugin.Handle", args, &resp); err != nil {
		return papi.HandleResult{}, err
	}
	return resp, nil
}

func (c *PluginRPCClient) Configure(ctx context.Context, config json.RawMessage) error {
	_ = ctx
	var out struct{}
	return c.client.Call("Plugin.Configure", ConfigureArgs{Config: config}, &out)
}

func (c *PluginRPCClient) Shutdown(ctx context.Context) error {
	_ = ctx
	var out struct{}
	return c.client.Call("Plugin.Shutdown", struct{}{}, &out)
}

func (c *PluginRPCClient) AttachHost(ctx context.Context, host HostAPI) error {
	_ = ctx
	if c.broker == nil || host == nil {
		return nil
	}
	bid := ServeHostAPI(c.broker, host)
	if bid == 0 {
		return nil
	}
	var out struct{}
	return c.client.Call("Plugin.AttachHost", AttachHostArgs{BrokerID: bid}, &out)
}

// ===== Host-side service (called by plugin) =====

type HostRPCServer struct {
	Impl HostAPI
}

func (s *HostRPCServer) CallOneBot(args CallOneBotArgs, resp *CallOneBotReply) error {
	var params any
	if len(args.Params) > 0 {
		if err := json.Unmarshal(args.Params, &params); err != nil {
			return err
		}
	}
	// SDK 版本：直接使用 args.SelfID，不处理 trace_id
	r, err := s.Impl.CallOneBot(context.Background(), args.Action, params)
	if err != nil {
		return err
	}
	resp.Resp = r
	return nil
}

func (s *HostRPCServer) CallDependency(args CallDependencyArgs, resp *CallDependencyReply) error {
	result, serr := s.Impl.CallDependency(context.Background(), args.TargetPluginID, args.Method, args.Params)
	resp.Result = result
	resp.Error = serr
	return nil
}

func (s *HostRPCServer) GetStats(_ GetStatsArgs, resp *GetStatsReply) error {
	r, err := s.Impl.GetStats(context.Background())
	if err != nil {
		return err
	}
	*resp = r
	return nil
}

// HostRPCClient is used in the plugin process to call host services.
type HostRPCClient struct {
	client *rpc.Client
}

func (c *HostRPCClient) CallOneBot(ctx context.Context, action string, params any) (ob11.APIResponse, error) {
	b, err := json.Marshal(params)
	if err != nil {
		return ob11.APIResponse{}, err
	}
	var resp CallOneBotReply

	// 从 context 中提取 SelfID
	selfID := GetSelfID(ctx)

	if err := c.client.Call("Plugin.CallOneBot", CallOneBotArgs{Action: action, Params: b, SelfID: selfID}, &resp); err != nil {
		return ob11.APIResponse{}, err
	}
	return resp.Resp, nil
}

func (c *HostRPCClient) CallDependency(ctx context.Context, targetPluginID string, method string, params any) (json.RawMessage, error) {
	_ = ctx
	b, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	var resp CallDependencyReply
	if err := c.client.Call("Plugin.CallDependency", CallDependencyArgs{TargetPluginID: targetPluginID, Method: method, Params: b}, &resp); err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, resp.Error
	}
	return resp.Result, nil
}

func (c *HostRPCClient) GetStats(ctx context.Context) (GetStatsReply, error) {
	_ = ctx
	var resp GetStatsReply
	if err := c.client.Call("Plugin.GetStats", GetStatsArgs{}, &resp); err != nil {
		return GetStatsReply{}, err
	}
	return resp, nil
}

// ===== go-plugin wiring =====

const PluginName = "nyanyabot_plugin"

var handshake = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "NYANYABOT_PLUGIN",
	MagicCookieValue: "1",
}

// Map implements plugin.Plugin for the main plugin RPC service.
type Map struct {
	PluginImpl papi.Plugin
	Host       HostAPI
}

func Handshake() plugin.HandshakeConfig { return handshake }

func (m *Map) Server(b *plugin.MuxBroker) (interface{}, error) {
	return &PluginRPCServer{Impl: m.PluginImpl, B: b}, nil
}

func (m *Map) Client(b *plugin.MuxBroker, c *rpc.Client) (interface{}, error) {
	pc := &PluginRPCClient{client: c, broker: b}
	if m.Host != nil {
		_ = pc.AttachHost(context.Background(), m.Host)
	}
	return pc, nil
}

// ServeHostAPI serves the host API over a brokered net/rpc stream.
func ServeHostAPI(b *plugin.MuxBroker, host HostAPI) (brokerID uint32) {
	if b == nil || host == nil {
		return 0
	}
	bid := b.NextId()
	go b.AcceptAndServe(bid, &HostRPCServer{Impl: host})
	return bid
}
