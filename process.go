package process

import (
	"os/exec"
	"time"

	"github.com/shirou/gopsutil/process"
	"go.uber.org/atomic"
	"golang.org/x/sync/errgroup"
)

type Process struct {
	*process.Process
	OutputFile string
	ErrorFile  string
	PidFile    string
	Name       string
	Dir        string
	Cmd        string
	Args       []string
	daemon     atomic.Int32
	cmd        *exec.Cmd
	g          *errgroup.Group
}

func Processes() ([]*Process, error) {
	var processes = make([]*Process, 0)
	_processes, err := process.Processes()
	if err != nil {
		return nil, err
	}

	for _, process := range _processes {
		name, _ := process.Name()
		processes = append(processes, &Process{Process: process, Name: name})
	}

	return processes, nil
}

func NewProcess(name string, cmd *exec.Cmd, proc *process.Process) *Process {
	p := &Process{Name: name, cmd: cmd, Process: proc}
	p.Args = cmd.Args
	p.daemon.Inc()

	return p
}

// StartAt procss start time
func (p *Process) StartAt() time.Time {
	startAt, err := p.CreateTime()
	if err != nil {
		return time.Time{}
	}

	return time.Unix(startAt/1000, startAt%1000*1000)
}

// Status process run status
func (p *Process) Status() string {
	if p.cmd == nil {
		return "E"
	}

	status, err := p.Process.Status()
	if err != nil {
		return "E"
	}

	return status
}
