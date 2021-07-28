package process

import (
	"testing"

	"github.com/tj/assert"
)

func TestProcesses(t *testing.T) {
	process, err := Processes()
	assert.NoError(t, err)
	assert.NotNil(t, process)
	assert.Greater(t, len(process), 1)
	t.Logf("process %v", process)
	for _, proc := range process {
		t.Logf("proc %s start at %s", proc.Name, proc.StartAt())
	}

}
