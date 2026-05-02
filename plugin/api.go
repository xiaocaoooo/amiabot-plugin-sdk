package plugin

import (
	"context"
	"encoding/json"

	"github.com/xiaocaoooo/amiabot-plugin-sdk/onebot/ob11"
)

// Plugin is the in-process plugin interface.
type Plugin interface {
	Descriptor(ctx context.Context) (Descriptor, error)
	Configure(ctx context.Context, config json.RawMessage) error
	Invoke(ctx context.Context, method string, paramsJSON json.RawMessage, callerPluginID string) (resultJSON json.RawMessage, err error)
	Handle(ctx context.Context, listenerID string, eventRaw ob11.Event, match *CommandMatch) (HandleResult, error)
	Shutdown(ctx context.Context) error
}

// CallOneBotFunc is provided to plugins so they can invoke OneBot actions.
type CallOneBotFunc func(ctx context.Context, action string, params any) (CallResult, error)

// Descriptor mirrors the plugin metadata contract.
type Descriptor struct {
	Name         string            `json:"name"`
	PluginID     string            `json:"plugin_id"`
	Version      string            `json:"version"`
	Author       string            `json:"author"`
	Description  string            `json:"description"`
	Dependencies []string          `json:"dependencies"`
	Exports      []ExportSpec      `json:"exports"`
	Config       *ConfigSpec       `json:"config,omitempty"`
	Commands     []CommandListener `json:"commands"`
	Events       []EventListener   `json:"events"`
	Crons        []CronListener    `json:"crons"`
}

// ConfigSpec describes a plugin's configuration schema and defaults.
type ConfigSpec struct {
	Version     string          `json:"version,omitempty"`
	Description string          `json:"description,omitempty"`
	Schema      json.RawMessage `json:"schema,omitempty"`
	Default     json.RawMessage `json:"default,omitempty"`
}

type ExportSpec struct {
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	ParamsSchema json.RawMessage `json:"params_schema"`
	ResultSchema json.RawMessage `json:"result_schema"`
}

type CommandListener struct {
	Name        string `json:"name"`
	ID          string `json:"id"`
	Description string `json:"description"`
	Pattern     string `json:"pattern"`
	MatchRaw    bool   `json:"match_raw"`
	Handler     string `json:"handler"`
}

type EventListener struct {
	Name        string `json:"name"`
	ID          string `json:"id"`
	Description string `json:"description"`
	Event       string `json:"event"`
	Handler     string `json:"handler"`
}

// CronListener defines a scheduled task.
type CronListener struct {
	Name        string `json:"name"`
	ID          string `json:"id"`
	Description string `json:"description"`
	Schedule    string `json:"schedule"`
	Handler     string `json:"handler"`
}

// CommandMatch captures regexp matches.
type CommandMatch struct {
	Full   string   `json:"full"`
	Groups []string `json:"groups"`
}

type HandleResult struct {
	// Reserved for future.
}

type CallResult struct {
	Raw ob11.APIResponse
}
