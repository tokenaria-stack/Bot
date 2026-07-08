package core

import (
	"encoding/json"
	"fmt"
)

// UIComponent describes one render binding for Data-Driven Rendering (DDR).
// Go treats RenderOpts as opaque bytes; the frontend parses LWC-specific options.
type UIComponent struct {
	ID         string          `json:"id"`
	Pane       string          `json:"pane"`
	Kind       string          `json:"kind"`
	DataMode   string          `json:"dataMode"`
	Slot       Slot            `json:"-"`
	RenderOpts json.RawMessage `json:"renderOptions,omitempty"`
	HostID     string          `json:"hostId,omitempty"`
}

// UIRegistry holds UI components grouped by pane. Built explicitly via NewUIRegistry.
type UIRegistry struct {
	panes map[string][]UIComponent
}

// NewUIRegistry returns an empty registry. No globals, no init().
func NewUIRegistry() *UIRegistry {
	return &UIRegistry{
		panes: make(map[string][]UIComponent),
	}
}

// Register adds components to the registry. Returns an error on duplicate ID.
func (r *UIRegistry) Register(components ...UIComponent) error {
	for _, c := range components {
		if c.ID == "" {
			return fmt.Errorf("ui component: empty id")
		}
		if r.hasID(c.ID) {
			return fmt.Errorf("ui component: duplicate id %q", c.ID)
		}
		if c.Pane == "" {
			return fmt.Errorf("ui component %q: empty pane", c.ID)
		}
		r.panes[c.Pane] = append(r.panes[c.Pane], c)
	}
	return nil
}

func (r *UIRegistry) hasID(id string) bool {
	for _, comps := range r.panes {
		for _, c := range comps {
			if c.ID == id {
				return true
			}
		}
	}
	return false
}

// UIManifest is the wire shape for GET /api/ui/manifest.
type UIManifest struct {
	Panes map[string][]UIComponent `json:"panes"`
}

// Manifest returns a defensive copy of all panes and their components.
func (r *UIRegistry) Manifest() UIManifest {
	if r == nil || len(r.panes) == 0 {
		return UIManifest{Panes: map[string][]UIComponent{}}
	}
	out := make(map[string][]UIComponent, len(r.panes))
	for pane, comps := range r.panes {
		out[pane] = append([]UIComponent(nil), comps...)
	}
	return UIManifest{Panes: out}
}

// Components returns every registered component (flat list for wire projection).
func (r *UIRegistry) Components() []UIComponent {
	if r == nil || len(r.panes) == 0 {
		return nil
	}
	var out []UIComponent
	for _, comps := range r.panes {
		out = append(out, comps...)
	}
	return out
}
