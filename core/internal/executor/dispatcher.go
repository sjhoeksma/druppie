package executor

import (
	"errors"
)

// Dispatcher selects the correct executor for a step
type Dispatcher struct {
	executors []Executor
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		executors: []Executor{
			&SceneCreatorExecutor{},
			// Add more executors here as we refactor
		},
	}
}

func (d *Dispatcher) GetExecutor(action string) (Executor, error) {
	for _, e := range d.executors {
		if e.CanHandle(action) {
			return e, nil
		}
	}
	// Fallback? Or return error
	return nil, errors.New("no executor found for action: " + action)
}
