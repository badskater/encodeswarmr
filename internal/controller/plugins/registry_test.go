package plugins

import (
	"errors"
	"testing"
)

// ---------------------------------------------------------------------------
// Registry.Register
// ---------------------------------------------------------------------------

func TestRegistry_Register(t *testing.T) {
	t.Run("registers a plugin successfully", func(t *testing.T) {
		r := NewRegistry()
		p := Plugin{Name: "test-enc", EncoderCmd: "enc"}
		if err := r.Register(p); err != nil {
			t.Fatalf("Register: unexpected error: %v", err)
		}
	})

	t.Run("returns error on empty name", func(t *testing.T) {
		r := NewRegistry()
		err := r.Register(Plugin{Name: ""})
		if err == nil {
			t.Fatal("expected error for empty name, got nil")
		}
	})

	t.Run("returns error on duplicate name", func(t *testing.T) {
		r := NewRegistry()
		p := Plugin{Name: "enc", EncoderCmd: "enc"}
		if err := r.Register(p); err != nil {
			t.Fatalf("first Register: %v", err)
		}
		if err := r.Register(p); err == nil {
			t.Fatal("expected error on duplicate Register, got nil")
		}
	})
}

// ---------------------------------------------------------------------------
// Registry.List
// ---------------------------------------------------------------------------

func TestRegistry_List(t *testing.T) {
	t.Run("empty registry returns empty slice", func(t *testing.T) {
		r := NewRegistry()
		list := r.List()
		if len(list) != 0 {
			t.Errorf("len(List()) = %d, want 0", len(list))
		}
	})

	t.Run("returns all registered plugins", func(t *testing.T) {
		r := NewRegistry()
		_ = r.Register(Plugin{Name: "a"})
		_ = r.Register(Plugin{Name: "b"})
		_ = r.Register(Plugin{Name: "c"})

		list := r.List()
		if len(list) != 3 {
			t.Errorf("len(List()) = %d, want 3", len(list))
		}
	})

	t.Run("returns copies not pointers to originals", func(t *testing.T) {
		r := NewRegistry()
		_ = r.Register(Plugin{Name: "orig", Enabled: true})

		list := r.List()
		if len(list) == 0 {
			t.Fatal("expected at least one plugin")
		}
		// Mutate the copy; the registry original should not change.
		list[0].Enabled = false

		got := r.Get("orig")
		if got == nil {
			t.Fatal("Get returned nil")
		}
		if !got.Enabled {
			t.Error("mutating List() copy changed registry's internal value")
		}
	})
}

// ---------------------------------------------------------------------------
// Registry.Get
// ---------------------------------------------------------------------------

func TestRegistry_Get(t *testing.T) {
	t.Run("returns nil for missing plugin", func(t *testing.T) {
		r := NewRegistry()
		if got := r.Get("missing"); got != nil {
			t.Errorf("Get(missing) = %v, want nil", got)
		}
	})

	t.Run("returns copy of registered plugin", func(t *testing.T) {
		r := NewRegistry()
		p := Plugin{Name: "enc", EncoderCmd: "myenc", Enabled: true}
		_ = r.Register(p)

		got := r.Get("enc")
		if got == nil {
			t.Fatal("Get returned nil for registered plugin")
		}
		if got.Name != "enc" {
			t.Errorf("Name = %q, want enc", got.Name)
		}
		if got.EncoderCmd != "myenc" {
			t.Errorf("EncoderCmd = %q, want myenc", got.EncoderCmd)
		}
	})
}

// ---------------------------------------------------------------------------
// Registry.Enable / Disable
// ---------------------------------------------------------------------------

func TestRegistry_Enable(t *testing.T) {
	t.Run("enables a disabled plugin", func(t *testing.T) {
		r := NewRegistry()
		_ = r.Register(Plugin{Name: "enc", Enabled: false})

		if err := r.Enable("enc"); err != nil {
			t.Fatalf("Enable: %v", err)
		}
		got := r.Get("enc")
		if !got.Enabled {
			t.Error("plugin should be enabled after Enable()")
		}
	})

	t.Run("returns ErrPluginNotFound for missing plugin", func(t *testing.T) {
		r := NewRegistry()
		err := r.Enable("nope")
		if !errors.Is(err, ErrPluginNotFound) {
			t.Errorf("Enable(missing) error = %v, want ErrPluginNotFound", err)
		}
	})
}

func TestRegistry_Disable(t *testing.T) {
	t.Run("disables an enabled plugin", func(t *testing.T) {
		r := NewRegistry()
		_ = r.Register(Plugin{Name: "enc", Enabled: true})

		if err := r.Disable("enc"); err != nil {
			t.Fatalf("Disable: %v", err)
		}
		got := r.Get("enc")
		if got.Enabled {
			t.Error("plugin should be disabled after Disable()")
		}
	})

	t.Run("returns ErrPluginNotFound for missing plugin", func(t *testing.T) {
		r := NewRegistry()
		err := r.Disable("nope")
		if !errors.Is(err, ErrPluginNotFound) {
			t.Errorf("Disable(missing) error = %v, want ErrPluginNotFound", err)
		}
	})
}

// ---------------------------------------------------------------------------
// RegisterBuiltins
// ---------------------------------------------------------------------------

func TestRegisterBuiltins(t *testing.T) {
	r := NewRegistry()
	if err := RegisterBuiltins(r); err != nil {
		t.Fatalf("RegisterBuiltins: %v", err)
	}

	t.Run("registers exactly 4 built-in plugins", func(t *testing.T) {
		list := r.List()
		if len(list) != 4 {
			t.Errorf("len(List()) = %d, want 4", len(list))
		}
	})

	expectedNames := []string{"x265", "x264", "svt-av1", "ffmpeg-copy"}
	for _, name := range expectedNames {
		t.Run("builtin "+name+" is registered and enabled", func(t *testing.T) {
			p := r.Get(name)
			if p == nil {
				t.Fatalf("plugin %q not registered", name)
			}
			if !p.Enabled {
				t.Errorf("plugin %q: Enabled = false, want true", name)
			}
			if p.EncoderCmd == "" {
				t.Errorf("plugin %q: EncoderCmd is empty", name)
			}
			if len(p.SupportedCodecs) == 0 {
				t.Errorf("plugin %q: SupportedCodecs is empty", name)
			}
		})
	}

	t.Run("returns error on second call with same registry", func(t *testing.T) {
		err := RegisterBuiltins(r)
		if err == nil {
			t.Error("expected error registering builtins into already-populated registry")
		}
	})
}

// ---------------------------------------------------------------------------
// Thread safety — concurrent Register / List / Get
// ---------------------------------------------------------------------------

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(Plugin{Name: "shared", Enabled: true})

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			_ = r.List()
			_ = r.Get("shared")
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}
