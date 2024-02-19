// Package cuectx provides a single, central ["cuelang.org/go/cue".Context] and
// ["github.com/grafana/thema".Runtime] that can be used uniformly across
// Grafana, and related helper functions for loading Thema lineages.

package cuectx

import (
	"sync"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/grafana/thema"
	"github.com/grafana/thema/vmux"
)

var (
	ctx  *cue.Context
	rt   *thema.Runtime
	once sync.Once
)

func initContext() {
	once.Do(func() {
		ctx = cuecontext.New()
		rt = thema.NewRuntime(ctx)
	})
}

// GrafanaCUEContext returns Grafana's singleton instance of [cue.Context].
//
// All code within grafana/grafana that needs a *cue.Context should get it
// from this function, when one was not otherwise provided.
func GrafanaCUEContext() *cue.Context {
	initContext()
	return ctx
}

// GrafanaThemaRuntime returns Grafana's singleton instance of [thema.Runtime].
//
// All code within grafana/grafana that needs a *thema.Runtime should get it
// from this function, when one was not otherwise provided.
func GrafanaThemaRuntime() *thema.Runtime {
	initContext()
	return rt
}

// JSONtoCUE attempts to decode the given []byte into a cue.Value, relying on
// the central Grafana cue.Context provided in this package.
//
// The provided path argument determines the name given to the input bytes if
// later CUE operations (e.g. Thema validation) produce errors related to the
// returned cue.Value.
//
// This is a convenience function for one-off JSON decoding. It's wasteful to
// call it repeatedly. Most use cases should probably prefer making
// their own Thema/CUE decoders.
func JSONtoCUE(path string, b []byte) (cue.Value, error) {
	return vmux.NewJSONCodec(path).Decode(GrafanaCUEContext(), b)
}
