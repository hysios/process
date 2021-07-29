package process

import (
	"reflect"
	"testing"

	"github.com/tj/assert"
)

func TestSplitCmd(t *testing.T) {
	type args struct {
		cmd string
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{args: args{"python -m simpleServer"}, want: []string{"python", "-m", "simpleServer"}},
		{args: args{"python '-m simpleServer' -c 'print \"hello world\"'"}, want: []string{"python", "-m simpleServer", "-c", "print \"hello world\""}},
		{args: args{"python \"-m simpleServer\""}, want: []string{"python", "-m simpleServer"}},
		{args: args{"ls -la -e"}, want: []string{"ls", "-la", "-e"}},
		{args: args{"ls $ARGS -e"}, want: []string{"ls", "$ARGS", "-e"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SplitCmd(tt.args.cmd); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitCmd() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewManager(t *testing.T) {
	manger := NewManager(&ManagerConfig{
		WorkerDir: "./tmp",
	})

	defer manger.Stop()
	go manger.Run()

	assert.NotNil(t, manger)
	proc, err := manger.StartProcess("ls", "", []string{"-la"}, nil, "ls")
	assert.NoError(t, err)
	assert.NotNil(t, proc)
	t.Logf("process status %s", proc.Status())
}

func TestManager_StartProcess(t *testing.T) {
	manager := NewManager(&ManagerConfig{
		WorkerDir: "./tmp",
	})

	proc, err := manager.StartProcess("ls", "", []string{"-la"}, nil, ".")
	assert.NoError(t, err)
	assert.NotNil(t, proc)
	t.Logf("process status %s", proc.Status())
	_, ok := manager.getProcess("ls")
	assert.True(t, ok)
}
