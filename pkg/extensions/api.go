package extensions

import (
	"fmt"
	"os"
	"sync"

	"github.com/dop251/goja"
)

// ExtensionAPI is exposed to goja scripts as the global `tau` object.
// It collects event handlers, commands, and tools registered by extensions.
type ExtensionAPI struct {
	mu       sync.Mutex
	vm       *goja.Runtime // set by Runtime.Bind
	handlers map[string][]goja.Callable
	commands map[string]*commandDef
	tools    []*toolDef
}

type commandDef struct {
	Name        string
	Description string
	Handler     goja.Callable
}

type toolDef struct {
	Name        string
	Description string
	Schema      map[string]interface{}
	Handler     goja.Callable
}

// NewAPI creates an ExtensionAPI.
func NewAPI() *ExtensionAPI {
	return &ExtensionAPI{
		handlers: make(map[string][]goja.Callable),
		commands: make(map[string]*commandDef),
	}
}

// setVM is called by Runtime.Bind to wire the goja runtime.
func (api *ExtensionAPI) setVM(vm *goja.Runtime) {
	api.vm = vm
}

// On registers an event handler for the given event name.
// Called from JS as: tau.on("turn:start", function(event, ctx) { ... })
func (api *ExtensionAPI) On(eventName string, handler goja.Callable) {
	api.mu.Lock()
	defer api.mu.Unlock()
	api.handlers[eventName] = append(api.handlers[eventName], handler)
}

// RegisterCommand registers a slash command.
// Called from JS as: tau.registerCommand("name", {description: "...", handler: fn})
func (api *ExtensionAPI) RegisterCommand(name string, def goja.Value) {
	api.mu.Lock()
	defer api.mu.Unlock()

	obj := def.ToObject(api.vm)
	if obj == nil {
		return
	}

	desc := ""
	if v := obj.Get("description"); v != nil {
		desc = v.String()
	}

	handler, ok := goja.AssertFunction(obj.Get("handler"))
	if !ok {
		return
	}

	api.commands[name] = &commandDef{
		Name:        name,
		Description: desc,
		Handler:     handler,
	}
}

// RegisterTool registers a tool.
// Called from JS as: tau.registerTool({name: "...", description: "...", schema: {...}, handler: fn})
func (api *ExtensionAPI) RegisterTool(def goja.Value) {
	api.mu.Lock()
	defer api.mu.Unlock()

	obj := def.ToObject(api.vm)
	if obj == nil {
		return
	}

	name := ""
	if v := obj.Get("name"); v != nil {
		name = v.String()
	}

	desc := ""
	if v := obj.Get("description"); v != nil {
		desc = v.String()
	}

	schema := map[string]interface{}{}
	if v := obj.Get("schema"); v != nil && !goja.IsUndefined(v) && !goja.IsNull(v) {
		exported := v.Export()
		if m, ok := exported.(map[string]interface{}); ok {
			schema = m
		}
	}

	handler, ok := goja.AssertFunction(obj.Get("handler"))
	if !ok {
		return
	}

	api.tools = append(api.tools, &toolDef{
		Name:        name,
		Description: desc,
		Schema:      schema,
		Handler:     handler,
	})
}

// RegisterProvider registers a custom provider (v1 stub).
// Called from JS as: tau.registerProvider("name", {stream: fn, complete: fn})
func (api *ExtensionAPI) RegisterProvider(name string, cfg goja.Value) {
	fmt.Printf("[ext] provider %q registered (v1 stub)\n", name)
}

// Fire dispatches an event to all registered handlers.
func (api *ExtensionAPI) Fire(eventName string, event goja.Value, ctx *ExtensionContext) {
	api.mu.Lock()
	handlers := make([]goja.Callable, len(api.handlers[eventName]))
	copy(handlers, api.handlers[eventName])
	api.mu.Unlock()

	for _, h := range handlers {
		_, err := h(goja.Undefined(), event, api.vm.ToValue(ctx))
		if err != nil {
			fmt.Fprintf(os.Stderr, "extensions: handler for %q error: %v\n", eventName, err)
		}
	}
}

// ListCommands returns a copy of all registered commands.
func (api *ExtensionAPI) ListCommands() []CommandInfo {
	api.mu.Lock()
	defer api.mu.Unlock()
	out := make([]CommandInfo, 0, len(api.commands))
	for _, c := range api.commands {
		out = append(out, CommandInfo{
			Name:        c.Name,
			Description: c.Description,
		})
	}
	return out
}

// ListTools returns a copy of all registered tools.
func (api *ExtensionAPI) ListTools() []ToolInfo {
	api.mu.Lock()
	defer api.mu.Unlock()
	out := make([]ToolInfo, 0, len(api.tools))
	for _, t := range api.tools {
		out = append(out, ToolInfo{
			Name:        t.Name,
			Description: t.Description,
		})
	}
	return out
}

// DispatchCommand looks up a command by name and invokes its handler.
// Returns true if the command was found and executed.
func (api *ExtensionAPI) DispatchCommand(name string, args string, ctx *ExtensionContext) bool {
	api.mu.Lock()
	cmd, ok := api.commands[name]
	api.mu.Unlock()

	if !ok || cmd == nil || api.vm == nil {
		return false
	}

	_, err := cmd.Handler(goja.Undefined(), api.vm.ToValue(args), api.vm.ToValue(ctx))
	if err != nil {
		fmt.Fprintf(os.Stderr, "extensions: command %q error: %v\n", name, err)
	}
	return true
}

// DispatchTool looks up a tool by name and invokes its handler.
// Returns (result, true) if found, ("", false) otherwise.
func (api *ExtensionAPI) DispatchTool(name string, params map[string]interface{}, ctx *ExtensionContext) (string, bool) {
	api.mu.Lock()
	var found *toolDef
	for _, t := range api.tools {
		if t.Name == name {
			found = t
			break
		}
	}
	api.mu.Unlock()

	if found == nil || api.vm == nil {
		return "", false
	}

	result, err := found.Handler(goja.Undefined(), api.vm.ToValue(params), api.vm.ToValue(ctx))
	if err != nil {
		fmt.Fprintf(os.Stderr, "extensions: tool %q error: %v\n", name, err)
		return "", false
	}
	return result.String(), true
}

// CommandInfo is the public view of a registered command.
type CommandInfo struct {
	Name        string
	Description string
}

// ToolInfo is the public view of a registered tool.
type ToolInfo struct {
	Name        string
	Description string
}
