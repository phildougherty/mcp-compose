package workflow

import (
	"context"
	"fmt"
	"time"

	"github.com/dop251/goja"
)

type JavaScriptVM struct {
	timeout time.Duration
}

func NewJavaScriptVM(timeout time.Duration) *JavaScriptVM {
	if timeout == 0 {
		timeout = 30 * time.Second
	}

	return &JavaScriptVM{
		timeout: timeout,
	}
}

func (vm *JavaScriptVM) ExecuteCondition(ctx context.Context, code string, input map[string]interface{}) (bool, error) {
	runtime := goja.New()

	runtime.Set("input", input)

	done := make(chan struct{})
	var result bool
	var execErr error

	go func() {
		defer close(done)

		defer func() {
			if r := recover(); r != nil {
				execErr = fmt.Errorf("panic in JavaScript execution: %v", r)
			}
		}()

		wrappedCode := "(function() { " + code + " })()"

		value, err := runtime.RunString(wrappedCode)
		if err != nil {
			execErr = fmt.Errorf("failed to execute JavaScript: %w", err)
			return
		}

		result = value.ToBoolean()
	}()

	select {
	case <-done:
		return result, execErr
	case <-ctx.Done():
		runtime.Interrupt("execution timeout")

		return false, ctx.Err()
	case <-time.After(vm.timeout):
		runtime.Interrupt("execution timeout")

		return false, fmt.Errorf("execution timeout after %v", vm.timeout)
	}
}

func (vm *JavaScriptVM) ExecuteTransform(ctx context.Context, code string, input map[string]interface{}) (map[string]interface{}, error) {
	runtime := goja.New()

	runtime.Set("input", input)

	done := make(chan struct{})
	var result map[string]interface{}
	var execErr error

	go func() {
		defer close(done)

		defer func() {
			if r := recover(); r != nil {
				execErr = fmt.Errorf("panic in JavaScript execution: %v", r)
			}
		}()

		wrappedCode := "(function() { " + code + " })()"

		value, err := runtime.RunString(wrappedCode)
		if err != nil {
			execErr = fmt.Errorf("failed to execute JavaScript: %w", err)
			return
		}

		exported := value.Export()

		switch v := exported.(type) {
		case map[string]interface{}:
			result = v
		case nil:
			result = make(map[string]interface{})
		default:
			result = map[string]interface{}{"result": v}
		}
	}()

	select {
	case <-done:
		return result, execErr
	case <-ctx.Done():
		runtime.Interrupt("execution timeout")

		return nil, ctx.Err()
	case <-time.After(vm.timeout):
		runtime.Interrupt("execution timeout")

		return nil, fmt.Errorf("execution timeout after %v", vm.timeout)
	}
}

func (vm *JavaScriptVM) ExecuteCode(ctx context.Context, code string, input map[string]interface{}) (map[string]interface{}, error) {
	runtime := goja.New()

	runtime.Set("input", input)

	var outputBuffer []interface{}
	var errorBuffer []interface{}

	runtime.Set("console", map[string]interface{}{
		"log": func(args ...interface{}) {
			outputBuffer = append(outputBuffer, args...)
		},
		"error": func(args ...interface{}) {
			errorBuffer = append(errorBuffer, args...)
		},
	})

	done := make(chan struct{})
	var result interface{}
	var execErr error

	go func() {
		defer close(done)

		defer func() {
			if r := recover(); r != nil {
				execErr = fmt.Errorf("panic in JavaScript execution: %v", r)
			}
		}()

		wrappedCode := "(function() { " + code + " })()"

		value, err := runtime.RunString(wrappedCode)
		if err != nil {
			execErr = fmt.Errorf("failed to execute JavaScript: %w", err)
			return
		}

		result = value.Export()
	}()

	select {
	case <-done:
		if execErr != nil {
			return nil, execErr
		}

		output := map[string]interface{}{
			"result": result,
		}

		if len(outputBuffer) > 0 {
			output["stdout"] = outputBuffer
		}

		if len(errorBuffer) > 0 {
			output["stderr"] = errorBuffer
		}

		return output, nil
	case <-ctx.Done():
		runtime.Interrupt("execution timeout")

		return nil, ctx.Err()
	case <-time.After(vm.timeout):
		runtime.Interrupt("execution timeout")

		return nil, fmt.Errorf("execution timeout after %v", vm.timeout)
	}
}
