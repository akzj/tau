package extensions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dop251/goja"
)

// Runtime wraps a goja VM for running extension scripts.
type Runtime struct {
	mu      sync.Mutex
	vm      *goja.Runtime
	api     *ExtensionAPI
	ctx     *ExtensionContext
	scripts []scriptInfo
	dir     string
}

type scriptInfo struct {
	path    string
	name    string
	program *goja.Program
}

// NewRuntime creates a new extension runtime for the given directory.
func NewRuntime(dir string) *Runtime {
	return &Runtime{dir: dir}
}

// LoadScripts reads all .js files from the extension directory and compiles them.
func (r *Runtime) LoadScripts() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entries, err := os.ReadDir(r.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read dir: %w", err)
	}

	var scripts []scriptInfo
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".js" {
			continue
		}
		path := filepath.Join(r.dir, e.Name())
		src, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		prog, err := goja.Compile(path, string(src), false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "extensions: compile %s: %v\n", path, err)
			continue
		}
		name := parseName(string(src))
		scripts = append(scripts, scriptInfo{path: path, name: name, program: prog})
	}
	r.scripts = scripts
	return nil
}

// Bind binds the API and context to a new VM and runs all scripts.
func (r *Runtime) Bind(api *ExtensionAPI, ctx *ExtensionContext) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.api = api
	r.ctx = ctx
	r.vm = goja.New()

	// Use uncapitalised field names for JS → Go mapping (tau.on → tau.On)
	r.vm.SetFieldNameMapper(goja.UncapFieldNameMapper())

	// Wire VM into API so Fire/Dispatch can create goja values
	api.setVM(r.vm)

	// Disable eval
	r.vm.Set("eval", goja.Undefined())

	// Expose tau global
	r.vm.Set("tau", api)

	// Expose console.log
	console := r.vm.NewObject()
	console.Set("log", func(call goja.FunctionCall) goja.Value {
		args := make([]string, len(call.Arguments))
		for i, a := range call.Arguments {
			args[i] = a.String()
		}
		fmt.Fprintln(os.Stderr, "[ext]", strings.Join(args, " "))
		return goja.Undefined()
	})
	r.vm.Set("console", console)

	// Run all scripts
	for _, s := range r.scripts {
		_, err := r.vm.RunProgram(s.program)
		if err != nil {
			fmt.Fprintf(os.Stderr, "extensions: run %s: %v\n", s.path, err)
		}
	}

	// Call default export if present
	defaultFn, ok := goja.AssertFunction(r.vm.Get("defaultExport"))
	if ok {
		_, err := defaultFn(goja.Undefined(), r.vm.ToValue(api))
		if err != nil {
			fmt.Fprintf(os.Stderr, "extensions: default export: %v\n", err)
		}
	}

	return nil
}

// VM returns the current goja runtime (caller must hold mu or be in single-goroutine context).
func (r *Runtime) VM() *goja.Runtime {
	return r.vm
}

// Reload recreates the VM and re-runs all scripts.
func (r *Runtime) Reload() error {
	if err := r.LoadScripts(); err != nil {
		return err
	}
	return r.Bind(r.api, r.ctx)
}

// Interrupt signals the VM to stop (for CPU limit).
func (r *Runtime) Interrupt() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.vm != nil {
		r.vm.Interrupt("timeout")
	}
}

func parseName(src string) string {
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "/// name:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "/// name:"))
		}
	}
	return ""
}
