package hooks

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

// RunTransform executes a transform hook, passing data as JSON on stdin
// and reading modified data from stdout.
func RunTransform(reg *Registry, point HookPoint, data any) (json.RawMessage, error) {
	hooks := reg.Get(point)
	if len(hooks) == 0 {
		return json.Marshal(data)
	}

	current, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal transform input: %w", err)
	}

	for _, h := range hooks {
		current, err = runTransformCommand(h.Command, current)
		if err != nil {
			return nil, fmt.Errorf("transform hook %s failed (%q): %w", point, h.Command, err)
		}
	}

	return current, nil
}

func runTransformCommand(command string, input []byte) ([]byte, error) {
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdin = bytes.NewReader(input)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w: %s", err, stderr.String())
	}

	output := stdout.Bytes()
	if len(output) == 0 {
		return input, nil // No output means no changes
	}

	// Validate it's valid JSON
	if !json.Valid(output) {
		return nil, fmt.Errorf("transform output is not valid JSON")
	}

	return output, nil
}
