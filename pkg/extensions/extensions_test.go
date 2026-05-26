package extensions

import (
	"testing"
	"time"

	"github.com/akzj/tau/core"
	"github.com/dop251/goja"
)

func TestNewAPI(t *testing.T) {
	api := NewAPI()
	if api == nil {
		t.Fatal("NewAPI returned nil")
	}
	if len(api.ListCommands()) != 0 {
		t.Error("expected zero commands")
	}
	if len(api.ListTools()) != 0 {
		t.Error("expected zero tools")
	}
}

func TestAPIRegistration(t *testing.T) {
	api := NewAPI()
	vm := goja.New()
	api.setVM(vm)

	// Simulate JS: tau.on("test:event", function(event, ctx) { ... })
	called := false
	fn := func(this goja.Value, args ...goja.Value) (goja.Value, error) {
		called = true
		return goja.Undefined(), nil
	}
	api.On("test:event", goja.Callable(fn))

	// Fire the event
	ctx := NewContext("sess-1")
	event := vm.ToValue(map[string]interface{}{"key": "val"})
	api.Fire("test:event", event, ctx)

	if !called {
		t.Error("expected handler to be called")
	}
}

func TestStaleContext(t *testing.T) {
	ctx := NewContext("sess-1")
	if !ctx.IsActive() {
		t.Error("new context should be active")
	}

	// Simulate new session (old becomes stale)
	ctx.SetCallbacks(
		func(text string) {},
		func() (*ExtensionContext, error) {
			return NewContext("sess-2"), nil
		},
		nil,
		nil,
	)

	newCtx, err := ctx.NewSession()
	if err != nil {
		t.Fatalf("NewSession: %v", err)
	}
	if newCtx.SessionID != "sess-2" {
		t.Errorf("expected sess-2, got %s", newCtx.SessionID)
	}
	if ctx.IsActive() {
		t.Error("old context should be stale")
	}

	// Stale context should reject SendMessage
	err = ctx.SendMessage("hello")
	if err == nil {
		t.Error("expected error from stale context SendMessage")
	}
}

func TestRuntimeLoadAndBind(t *testing.T) {
	rt := NewRuntime("../../demo/extensions")
	err := rt.LoadScripts()
	if err != nil {
		t.Fatalf("LoadScripts: %v", err)
	}
	if len(rt.scripts) == 0 {
		t.Fatal("expected at least one script (hello.js)")
	}

	api := NewAPI()
	ctx := NewContext("test-sess")
	err = rt.Bind(api, ctx)
	if err != nil {
		t.Fatalf("Bind: %v", err)
	}

	// hello.js registers a "hello" command and "hello-tool" tool
	cmds := api.ListCommands()
	if len(cmds) == 0 {
		t.Error("expected at least one command registered by hello.js")
	}
	found := false
	for _, c := range cmds {
		if c.Name == "hello" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'hello' command, got: %v", cmds)
	}

	tools := api.ListTools()
	if len(tools) == 0 {
		t.Error("expected at least one tool registered by hello.js")
	}
	found = false
	for _, t := range tools {
		if t.Name == "hello-tool" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'hello-tool' tool, got: %v", tools)
	}
}

func TestDispatchCommand(t *testing.T) {
	api := NewAPI()
	vm := goja.New()
	api.setVM(vm)

	received := ""
	fn := func(this goja.Value, args ...goja.Value) (goja.Value, error) {
		received = args[0].String()
		return goja.Undefined(), nil
	}
	api.commands["test-cmd"] = &commandDef{
		Name:        "test-cmd",
		Description: "test",
		Handler:     goja.Callable(fn),
	}

	ctx := NewContext("sess-1")
	ok := api.DispatchCommand("test-cmd", "arg1 arg2", ctx)
	if !ok {
		t.Error("DispatchCommand should return true")
	}
	if received != "arg1 arg2" {
		t.Errorf("expected 'arg1 arg2', got %q", received)
	}
}

func TestBridgeHandleEvent(t *testing.T) {
	api := NewAPI()
	vm := goja.New()
	api.setVM(vm)
	ctx := NewContext("sess-1")
	bridge := NewBridge(api, ctx)

	// Register handler for turn:start
	received := ""
	fn := func(this goja.Value, args ...goja.Value) (goja.Value, error) {
		// args[0] = event, args[1] = ctx
		obj := args[0].ToObject(vm)
		received = obj.Get("turnID").String()
		return goja.Undefined(), nil
	}
	api.On("turn:start", goja.Callable(fn))

	// Simulate a TurnStart AgentEvent
	ev := core.TurnStart{Timestamp_: time.Now(), TurnID: "turn-42"}
	bridge.HandleAgentEvent(ev)

	if received != "turn-42" {
		t.Errorf("expected 'turn-42', got %q", received)
	}
}

func TestMultiTurnEvents(t *testing.T) {
	api := NewAPI()
	vm := goja.New()
	api.setVM(vm)
	ctx := NewContext("sess-1")

	var events []string
	api.On("turn:start", goja.Callable(func(this goja.Value, args ...goja.Value) (goja.Value, error) {
		events = append(events, "turn:start")
		return goja.Undefined(), nil
	}))
	api.On("turn:end", goja.Callable(func(this goja.Value, args ...goja.Value) (goja.Value, error) {
		events = append(events, "turn:end")
		return goja.Undefined(), nil
	}))
	api.On("tool:start", goja.Callable(func(this goja.Value, args ...goja.Value) (goja.Value, error) {
		events = append(events, "tool:start")
		return goja.Undefined(), nil
	}))

	// Simulate a multi-turn sequence
	api.Fire("turn:start", vm.ToValue(map[string]string{"turnID": "1"}), ctx)
	api.Fire("tool:start", vm.ToValue(map[string]string{"callID": "c1", "toolName": "echo"}), ctx)
	api.Fire("turn:end", vm.ToValue(map[string]string{"turnID": "1"}), ctx)

	if len(events) != 3 {
		t.Errorf("expected 3 events, got %d: %v", len(events), events)
	}
	if events[0] != "turn:start" {
		t.Errorf("expected turn:start first, got %s", events[0])
	}
	if events[1] != "tool:start" {
		t.Errorf("expected tool:start second, got %s", events[1])
	}
	if events[2] != "turn:end" {
		t.Errorf("expected turn:end third, got %s", events[2])
	}
}

func TestStaleContextFork(t *testing.T) {
	ctx := NewContext("sess-1")
	ctx.SetCallbacks(
		func(text string) {},
		nil,
		func() (*ExtensionContext, error) {
			return NewContext("sess-fork"), nil
		},
		nil,
	)

	// Fork → old context goes stale
	newCtx, err := ctx.Fork()
	if err != nil {
		t.Fatalf("Fork: %v", err)
	}
	if newCtx.SessionID != "sess-fork" {
		t.Errorf("expected sess-fork, got %s", newCtx.SessionID)
	}

	// Old context should be stale after fork
	if ctx.IsActive() {
		t.Error("old context should be stale after fork")
	}

	// Stale context rejects all operations
	if err := ctx.SendMessage("test"); err == nil {
		t.Error("expected stale context error from SendMessage")
	}
	if _, err := ctx.NewSession(); err == nil {
		t.Error("expected stale context error from NewSession")
	}
	if _, err := ctx.Fork(); err == nil {
		t.Error("expected stale context error from Fork")
	}
}
