//go:build !no_plugins

package core

import "sync"

// Plugin is the interface for tau plugins.
// Plugins can register tools and providers that are merged into the agent session.
type Plugin interface {
	Name() string
	Version() string
	Tools() []Tool
	Providers() []Provider
}

var (
	pluginMu sync.Mutex
	plugins  []Plugin
)

// RegisterPlugin registers a plugin. Call during init() or before session creation.
func RegisterPlugin(p Plugin) {
	pluginMu.Lock()
	defer pluginMu.Unlock()
	plugins = append(plugins, p)
}

// AllPluginTools returns all tools from all registered plugins.
func AllPluginTools() []Tool {
	pluginMu.Lock()
	defer pluginMu.Unlock()
	var tools []Tool
	for _, p := range plugins {
		tools = append(tools, p.Tools()...)
	}
	return tools
}

// AllPluginProviders returns all providers from all registered plugins.
func AllPluginProviders() []Provider {
	pluginMu.Lock()
	defer pluginMu.Unlock()
	var provs []Provider
	for _, p := range plugins {
		provs = append(provs, p.Providers()...)
	}
	return provs
}
