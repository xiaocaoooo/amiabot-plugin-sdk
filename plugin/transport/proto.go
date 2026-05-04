package transport

import (
	"context"
	"encoding/json"
	"time"

	"github.com/xiaocaoooo/amiabot-plugin-sdk/onebot/ob11"
	papi "github.com/xiaocaoooo/amiabot-plugin-sdk/plugin"
)

type DescribeReply = papi.Descriptor

type ConfigureArgs struct {
	Config json.RawMessage `json:"config"`
}

type HandleArgs struct {
	ListenerID   string          `json:"listener_id"`
	EventRawJSON json.RawMessage `json:"event_raw_json"`
	Match        *papi.CommandMatch
}

type HandleReply = papi.HandleResult

type InvokeArgs struct {
	Method         string          `json:"method"`
	Params         json.RawMessage `json:"params"`
	CallerPluginID string          `json:"caller_plugin_id"`
}

type InvokeReply struct {
	Result json.RawMessage       `json:"result"`
	Error  *papi.StructuredError `json:"error,omitempty"`
}

type CallOneBotArgs struct {
	Action string          `json:"action"`
	Params json.RawMessage `json:"params"`
	SelfID int64           `json:"self_id,omitempty"` // 可选：指定目标 bot；0 表示使用上下文推断
}

type CallOneBotReply struct {
	Resp ob11.APIResponse `json:"resp"`
}

type CallDependencyArgs struct {
	TargetPluginID string          `json:"target_plugin_id"`
	Method         string          `json:"method"`
	Params         json.RawMessage `json:"params"`
}

type CallDependencyReply struct {
	Result json.RawMessage       `json:"result"`
	Error  *papi.StructuredError `json:"error,omitempty"`
}

type GetStatsArgs struct{}

type GetStatsReply struct {
	RecvCount int64     `json:"recv_count"`
	SentCount int64     `json:"sent_count"`
	StartTime time.Time `json:"start_time"`
	Uptime    string    `json:"uptime"`
}

// HostAPI is implemented by host and exposed to plugin.
type HostAPI interface {
	CallOneBot(ctx context.Context, action string, params any) (ob11.APIResponse, error)
	CallDependency(ctx context.Context, targetPluginID string, method string, params json.RawMessage) (json.RawMessage, *papi.StructuredError)
	GetStats(ctx context.Context) (GetStatsReply, error)
}
