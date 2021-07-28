package process

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/hysios/log"
	"github.com/shirou/gopsutil/process"
	"golang.org/x/sync/errgroup"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Manager struct {
	WorkerDir  string
	ConfigFile string

	done        chan bool
	processExit chan *Process
	processStop chan *Process
	process     sync.Map
}

type ManagerConfig struct {
	Filename  string
	WorkerDir string
	Procs     []Process
}

var (
	DefaultConfig  = ManagerConfig{Filename: "process.yaml", WorkerDir: "./run"}
	DefaultManager = NewManager(&DefaultConfig)
)

func SplitCmd(cmd string) []string {
	return strings.FieldsFunc(cmd, MakeSplitQuota())
}

func NewManager(cfg *ManagerConfig) *Manager {
	if cfg == nil {
		_cfg := DefaultConfig
		cfg = &_cfg
	}

	m := &Manager{
		ConfigFile:  cfg.Filename,
		WorkerDir:   cfg.WorkerDir,
		done:        make(chan bool),
		processExit: make(chan *Process),
		processStop: make(chan *Process),
	}

	if len(cfg.Filename) > 0 {
		m.LoadProcesses(cfg.Filename)
	}

	return m
}

// StartProcess starts a process
func (m *Manager) StartProcess(name string, args []string, env []string, dir string) (*Process, error) {
	var (
		cmd     = exec.Command(name, args...)
		fulldir = path.Join(m.WorkerDir, dir)
	)
	log.Debugf("fulldir %s", fulldir)

	os.MkdirAll(fulldir, 0755)

	cmd.Dir = fulldir
	if env == nil {
		cmd.Env = os.Environ()
	} else {
		cmd.Env = env
	}

	process, err := m.runProcess(NewProcess(name, cmd, nil))
	if err != nil {
		return nil, err
	}

	m.process.Store(name, process)
	return process, nil
}

// RestartProcess restarts a process
func (m *Manager) RestartProcess(name string) error {
	process, ok := m.getProcess(name)
	if !ok {
		return ErrProcessNotFound
	}
	log.Infof("restart process name %s", name)
	// R: Running S: Sleep T: Stop I: Idle Z: Zombie W: Wait L: Lock The character is same within all supported platforms.
	switch process.Status() {
	case "R", "S", "I", "W", "L": // Running
		process.Kill()
	case "T", "Z": // Stopped
		process.Kill()
	}

	process.cmd = Clone(process.cmd)
	_, err := m.runProcess(process)

	return err
}

// Clone copying cmd struct
func Clone(cmd *exec.Cmd) *exec.Cmd {
	var cmd2 = new(exec.Cmd)
	cmd2.Path = cmd.Path
	cmd2.Args = cmd.Args
	cmd2.Env = cmd.Env
	cmd2.ExtraFiles = cmd.ExtraFiles
	cmd2.Dir = cmd.Dir
	return cmd2
}

// LoadProcesses load process in config file
func (m *Manager) LoadProcesses(filename string) error {
	// yaml.

	return errors.New("nonimplement")
}

// SaveConfig save all processes config to a file
func (m *Manager) SaveConfig() error {
	panic("nonimplement")
}

type inputReader struct {
	exit chan bool
}

func (r *inputReader) Read(p []byte) (n int, err error) {

	<-r.exit
	return 0, io.EOF
}

// runProcess runs a process
func (m *Manager) runProcess(pproc *Process) (*Process, error) {
	cmd := pproc.cmd
	var g = new(errgroup.Group)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	// var inputExit = make(chan bool)

	// var b = new(bytes.Buffer)
	// cmd.Stdin = bufio.NewReader(b)
	// b.WriteString("\n")

	// cmd.Stdin = &inputReader{exit: inputExit}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	go func() {
		io.WriteString(stdin, "\n")
		// io.WriteString(stdin, "blob\n")
		// io.WriteString(stdin, "booo\n")
	}()
	// defer stdin.Close()
	// log.Infof("stdin %v", stdin)

	if err = cmd.Start(); err != nil {
		return nil, err
	}
	name := path.Base(cmd.Path)

	g.Go(func() error {
		log.Infof("create '%s' out file to %s", name, cmd.Dir+"/"+name+".out")
		var logger = m.createLogger(cmd.Dir, name+".out")
		defer logger.Close()
		w := io.MultiWriter(logger, os.Stdout)
		_, err := io.Copy(w, stdout)
		return err
	})

	g.Go(func() error {
		log.Infof("create '%s' err file to %s", name, cmd.Dir+"/"+name+".err")
		var logger = m.createLogger(cmd.Dir, name+".err")
		defer logger.Close()
		w := io.MultiWriter(logger, os.Stdout)
		_, err := io.Copy(w, stderr)
		return err
	})

	// g.Go(func() error {
	// 	io.Copy(w, ioutil.Discard)
	// })

	proc, err := process.NewProcess(int32(cmd.Process.Pid))
	if err != nil {
		return nil, err
	}

	pproc.Process = proc
	// if err != nil {
	// 	return nil, err
	// }

	err = m.createPidfile(cmd, cmd.Dir, name+".pid")
	if err != nil {
		return nil, err
	}

	pproc.OutputFile = path.Join(cmd.Dir, name+".out")
	pproc.ErrorFile = path.Join(cmd.Dir, name+".err")
	pproc.PidFile = path.Join(cmd.Dir, name+".pid")

	// pproc := &Process{Name: cmd.Args[0], cmd: cmd, g: g, Process: proc}

	g.Go(func() error {
		err := cmd.Wait()
		// inputExit <- true
		m.processExit <- pproc
		return err
	})

	return pproc, nil
}

func (m *Manager) createLogger(dir, nameAndExt string) io.WriteCloser {
	var output = path.Join(dir, nameAndExt)

	return &lumberjack.Logger{
		Filename:   output,
		MaxSize:    500, // megabytes
		MaxBackups: 3,
		MaxAge:     28,   //days
		Compress:   true, // disabled by default
	}
}

func (m *Manager) createPidfile(cmd *exec.Cmd, dir, nameAndExt string) error {
	pidfile, err := os.OpenFile(path.Join(dir, nameAndExt), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	fmt.Fprintf(pidfile, "%d", cmd.Process.Pid)
	return nil
}

// StopProcess stops a process
func (m *Manager) StopProcess(name string) error {
	proc, ok := m.getProcess(name)
	if !ok {
		return ErrProcessNotFound
	}

	runing, err := proc.IsRunning()
	if err != nil {
		return err
	}

	m.processStop <- proc
	proc.daemon.Store(0)
	if runing {
		if err := proc.cmd.Process.Signal(os.Interrupt); err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) getProcess(name string) (*Process, bool) {
	var val, ok = m.process.Load(name)
	if !ok {
		return nil, false
	}

	proc, ok := val.(*Process)
	return proc, ok
}

// AttachProcess attachs a process
func (m *Manager) AttachProcess(id int) error {
	panic("nonimplement")
}

func (m *Manager) AllStatus() ([]*Process, error) {
	var processes = make([]*Process, 0)
	m.process.Range(func(key, value interface{}) bool {
		// convert value to *Process and append to processes
		processes = append(processes, value.(*Process))
		return true
	})

	return processes, nil
}

// Run startup manager
func (m *Manager) Run() error {
	if m.done == nil {
		m.done = make(chan bool)
	}

	if m.processStop == nil {
		m.processStop = make(chan *Process)
	}

	if m.processExit == nil {
		m.processExit = make(chan *Process)
	}

	for {
		select {
		case process := <-m.processExit:
			log.Infof("exit process %s", process.Name)
			// m.process.Delete(process.Name)
			if process.daemon.Load() > 0 {
				go func() {
					if process.daemon.Load() > 0 {
						time.Sleep(5 * time.Second)
						m.RestartProcess(process.Name)
					}
				}()
			}
		case process := <-m.processStop:
			log.Infof("stop process %s", process.Name)
			// m.process.Delete(process.Name)
			// process.daemon.Store(0)
		case <-m.done:
			return io.EOF
		}
	}
}

func (m *Manager) Stop() error {
	m.done <- true
	return nil
}

func init() {
	gob.Register(new(Process))
}
