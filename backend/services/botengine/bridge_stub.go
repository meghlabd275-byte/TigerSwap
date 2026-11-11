//go:build !rust

// A default build has no Rust engine linked in. This stub keeps the package
// compiling and makes construction report ErrUnavailable, so a caller can
// never mistake a missing engine for a bot that simply had nothing to do.
package botengine

// New reports that the engine is not part of this build.
func New() (Engine, error) {
	return nil, ErrUnavailable
}

// Available reports whether the Rust engine is compiled into this binary.
func Available() bool {
	return false
}
