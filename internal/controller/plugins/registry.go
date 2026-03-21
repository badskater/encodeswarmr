// Package plugins implements a lightweight plugin registry for encoding
// pipelines.  A plugin describes one encoder backend (e.g. x265, SVT-AV1)
// including its command, default arguments, and the codecs it supports.
//
// The registry is safe for concurrent use.  Plugins are registered at
// program start and may be enabled or disabled at run time via the API.
package plugins

import (
	"errors"
	"fmt"
	"sync"
)

// Plugin describes a single encoding backend.
type Plugin struct {
	// Name is a unique identifier for the plugin, e.g. "x265".
	Name string `yaml:"name" json:"name"`

	// Version is the advertised version string of the underlying tool.
	Version string `yaml:"version" json:"version"`

	// Description is a human-readable summary of the plugin.
	Description string `yaml:"description" json:"description"`

	// Enabled controls whether the plugin is available for job dispatch.
	Enabled bool `yaml:"enabled" json:"enabled"`

	// EncoderCmd is the executable name or full path, e.g. "x265" or
	// "/usr/local/bin/ffmpeg".
	EncoderCmd string `yaml:"encoder_cmd" json:"encoder_cmd"`

	// DefaultArgs holds the default command-line arguments passed to
	// EncoderCmd before any job-specific arguments.
	DefaultArgs []string `yaml:"default_args" json:"default_args"`

	// SupportedCodecs lists the output codec identifiers this plugin can
	// produce, e.g. ["hevc", "h265"].
	SupportedCodecs []string `yaml:"supported_codecs" json:"supported_codecs"`
}

// Registry is a thread-safe store for Plugin instances.
type Registry struct {
	mu      sync.RWMutex
	plugins map[string]*Plugin // keyed by Plugin.Name
}

// ErrPluginNotFound is returned by Enable and Disable when the named plugin
// does not exist in the registry.
var ErrPluginNotFound = errors.New("plugins: plugin not found")

// NewRegistry returns an empty Registry.
func NewRegistry() *Registry {
	return &Registry{plugins: make(map[string]*Plugin)}
}

// Register adds p to the registry.  It returns an error if a plugin with the
// same name already exists.
func (r *Registry) Register(p Plugin) error {
	if p.Name == "" {
		return fmt.Errorf("plugins: Register: Name must not be empty")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.plugins[p.Name]; exists {
		return fmt.Errorf("plugins: Register: plugin %q is already registered", p.Name)
	}
	clone := p // avoid aliasing
	r.plugins[p.Name] = &clone
	return nil
}

// List returns all registered plugins in an unspecified order.
func (r *Registry) List() []*Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Plugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		cp := *p
		out = append(out, &cp)
	}
	return out
}

// Get returns the plugin with the given name, or nil if it is not registered.
func (r *Registry) Get(name string) *Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[name]
	if !ok {
		return nil
	}
	cp := *p
	return &cp
}

// Enable marks the named plugin as enabled.  It returns an error if the
// plugin is not found.
func (r *Registry) Enable(name string) error {
	return r.setEnabled(name, true)
}

// Disable marks the named plugin as disabled.  It returns an error if the
// plugin is not found.
func (r *Registry) Disable(name string) error {
	return r.setEnabled(name, false)
}

func (r *Registry) setEnabled(name string, enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.plugins[name]
	if !ok {
		return fmt.Errorf("%w: %s", ErrPluginNotFound, name)
	}
	p.Enabled = enabled
	return nil
}
