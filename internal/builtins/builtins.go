// Package builtins wires the built-in identity providers and targets into
// their registries. Custom distributions can build their own main package
// that registers additional providers/targets alongside (or instead of)
// these.
package builtins

import (
	"sync"

	"github.com/doriansobacki/agentpack/pkg/identity"
	"github.com/doriansobacki/agentpack/pkg/identity/static"
	"github.com/doriansobacki/agentpack/pkg/target"
	"github.com/doriansobacki/agentpack/pkg/target/agentsmd"
	"github.com/doriansobacki/agentpack/pkg/target/claude"
	"github.com/doriansobacki/agentpack/pkg/target/cursor"
)

var once sync.Once

// Register registers all built-ins. Safe to call more than once.
func Register() {
	once.Do(func() {
		identity.Register(static.New())
		target.Register(claude.New())
		target.Register(agentsmd.New())
		target.Register(cursor.New())
	})
}
