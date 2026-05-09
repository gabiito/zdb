package db

import "fmt"

// DriverFactory creates a new Driver instance for the given engine.
type DriverFactory func() Driver

var registry = map[string]DriverFactory{}

// Register registers a driver factory for the given engine name.
// Called from each adapter's init() function.
func Register(engine string, factory DriverFactory) {
	registry[engine] = factory
}

// New returns a new Driver for the given engine name.
// Returns an error if the engine is not registered.
func New(engine string) (Driver, error) {
	factory, ok := registry[engine]
	if !ok {
		return nil, fmt.Errorf("db: engine %q not registered", engine)
	}
	return factory(), nil
}
