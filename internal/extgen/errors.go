package extgen

import "fmt"

type GeneratorError struct {
	Stage   string
	Message string
	Err     error
}

func (e *GeneratorError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("generator error at %s: %s", e.Stage, e.Message)
	}

	return fmt.Sprintf("generator error at %s: %s: %v", e.Stage, e.Message, e.Err)
}
