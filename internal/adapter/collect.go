package adapter

import (
	"os"

	"github.com/armaniacs/provsync/internal/i18n"
	"github.com/armaniacs/provsync/internal/model"
)

// ToolState is the collected per-tool configuration state.
// Err holds a per-tool Get/Pull failure and is nil on success.
// Callers keep their legacy error policy: list/status abort on the
// first Err, doctor classifies an Err as an NG item and continues.
type ToolState struct {
	Name      string
	Path      string
	Exists    bool
	Providers map[string]model.Provider
	Warnings  []*i18n.Message
	Err       error
}

// CollectToolStates runs the Names -> Get -> Stat -> Pull collection for
// each name in order and returns one ToolState per name.
// A missing file yields Exists=false with nil Err; Get/Pull failures yield
// Err with no policy attached. Get failures leave Path empty so callers
// can tell them apart from Pull failures (which keep the resolved Path).
func CollectToolStates(root Root, names []string) []ToolState {
	states := make([]ToolState, 0, len(names))
	for _, name := range names {
		a, err := Get(name, root)
		if err != nil {
			states = append(states, ToolState{Name: name, Err: err})
			continue
		}
		st := ToolState{Name: name, Path: a.Path()}
		if _, err := os.Stat(a.Path()); err != nil {
			states = append(states, st)
			continue
		}
		st.Exists = true
		providers, warnings, err := a.Pull()
		if err != nil {
			st.Err = err
			states = append(states, st)
			continue
		}
		st.Providers = providers
		st.Warnings = warnings
		states = append(states, st)
	}
	return states
}
