package process

import (
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/fatih/structs"
	"github.com/hysios/log"
	"github.com/hysios/utils/convert"
	"github.com/shirou/gopsutil/process"
	"golang.org/x/sync/errgroup"
	"gopkg.in/natefinch/lumberjack.v2"
	"gopkg.in/yaml.v3"
)

type Manager struct {
	WorkerDir  string
	ConfigFile string
	Echo       bool

	done        chan bool
	processExit chan *Process
	processStop chan *Process
	process     sync.Map
}

type ManagerConfig struct {
	Filename  string
	WorkerDir string
	Procs     []Process
	Echo      bool
}

var (
	DefaultConfig = ManagerConfig{Filename: "process.yaml", WorkerDir: "./run"}
	// DefaultManager = NewManager(&DefaultConfig)
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
		Echo:        cfg.Echo,
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
func (m *Manager) StartProcess(name string, binary string, args []string, env []string, dir string) (*Process, error) {
	// var (
	// 	// cwd, _  = os.Getwd()
	// 	fullbin string
	// 	err     error
	// )

	// if IsRelPath(name) {
	// 	fullbin, err = filepath.Abs(name)
	// 	if err != nil {
	// 		return nil, err
	// 	}
	// } else {
	// 	fullbin = name
	// }

	// name = filepath.Base(name)

	// if err != nil {
	// 	return nil, err
	// }
	var fullbin string
	if len(binary) > 0 {
		fullbin = binary
	} else {
		fullbin = name
	}
	var (
		cmd     = exec.Command(fullbin, args...)
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

	return process, m.SaveConfig()
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
	f, err := os.OpenFile(filename, os.O_RDONLY, os.ModePerm)
	if err != nil {
		return err
	}

	var dec = yaml.NewDecoder(f)
	var mm = make(map[string]interface{})
	if err = dec.Decode(&mm); err != nil {
		return err
	}

	if workdir, ok := mm["WorkerDir"].(string); ok {
		m.WorkerDir = workdir
	}

	if configFile, ok := mm["ConfigFile"].(string); ok {
		m.ConfigFile = configFile
	}

	if procs, ok := mm["Procs"].([]interface{}); ok {
		for _, pm := range procs {
			if mmm, ok := pm.(map[string]interface{}); ok {
				log.Infof("pm %v", mmm)
				proc := m.loadProc(mmm)
				if proc == nil {
					log.Errorf("load process is nil")
					continue
				}

				proc, err := m.runProcess(proc)
				if err != nil {
					log.Errorf("run process %s error %s", proc.Name, err)
					continue
				}
				m.process.Store(proc.Name, proc)
				// m.
			}
		}
	}

	return nil
}

func (m *Manager) loadProc(pm map[string]interface{}) *Process {
	var (
		name   string
		binary string
		args   []string
		env    []string
		dir    string
		ok     bool
	)

	if name, ok = pm["Name"].(string); !ok {
		return nil
	}

	if binary, ok = pm["Binary"].(string); !ok {
		return nil
	}

	args, ok = convert.SliceString(pm["Args"])
	env, ok = convert.SliceString(pm["Env"])
	dir, ok = pm["Dir"].(string)

	var (
		cmd     = exec.Command(binary, args...)
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

	return NewProcess(name, cmd, nil)
}

// SaveConfig save all processes config to a file
func (m *Manager) SaveConfig() error {
	if len(m.ConfigFile) == 0 {
		return nil
	}

	var (
		mm    = structs.Map(m)
		procs = make([]interface{}, 0)
	)
	m.process.Range(func(key, value interface{}) bool {
		proc := value.(*Process)
		procs = append(procs, structs.Map(proc))
		return true
	})

	mm["Procs"] = procs
	f, err := os.OpenFile(m.ConfigFile, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := yaml.NewEncoder(f)
	return enc.Encode(mm)
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

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	go func() {
		io.WriteString(stdin, "\n")
	}()

	if err = cmd.Start(); err != nil {
		return nil, err
	}
	name := path.Base(cmd.Path)

	g.Go(func() error {
		log.Infof("create '%s' out file to %s", name, cmd.Dir+"/"+name+".out")
		var logger = m.createLogger(cmd.Dir, name+".out")
		defer logger.Close()
		var w io.Writer = logger
		if m.Echo {
			w = io.MultiWriter(logger, os.Stdout)
		}
		_, err := io.Copy(w, stdout)
		return err
	})

	g.Go(func() error {
		log.Infof("create '%s' err file to %s", name, cmd.Dir+"/"+name+".err")
		var logger = m.createLogger(cmd.Dir, name+".err")
		defer logger.Close()
		var w io.Writer = logger
		if m.Echo {
			w = io.MultiWriter(logger, os.Stdout)
		}
		_, err := io.Copy(w, stderr)
		return err
	})

	proc, err := process.NewProcess(int32(cmd.Process.Pid))
	if err != nil {
		return nil, err
	}

	pproc.Process = proc

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

// RemoveProcess removes a process
func (m *Manager) RemoveProcess(name string) error {
	proc, ok := m.getProcess(name)
	if !ok {
		return ErrProcessNotFound
	}
	m.processStop <- proc
	proc.daemon.Store(0)
	m.process.Delete(proc.Name)
	proc.cmd.Process.Signal(os.Interrupt)

	return m.SaveConfig()
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

func IsRelPath(p string) bool {
	return strings.HasPrefix(p, ".")
}
