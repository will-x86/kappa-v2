package runtime

type Runtime string

const (
	RuntimeGo     Runtime = "go1.x"
	RuntimeNode   Runtime = "nodejs18.x"
	RuntimePython Runtime = "python3.9"
)

type RuntimeConfig struct {
	Type            Runtime
	BaseImage       string
	BootstrapScript string
	EntryPoint      []string
}

var runtimeConfigs = map[Runtime]RuntimeConfig{
	RuntimeGo: {
		Type:       RuntimeGo,
		BaseImage:  "golang:1.20-alpine",
		EntryPoint: []string{"/bootstrap"},
	},
	RuntimeNode: {
		Type:       RuntimeNode,
		BaseImage:  "node:18-alpine",
		EntryPoint: []string{"node", "/var/runtime/bootstrap.js"},
	},
	RuntimePython: {
		Type:       RuntimePython,
		BaseImage:  "python:3.9-alpine",
		EntryPoint: []string{"python", "/var/runtime/bootstrap.py"},
	},
}
