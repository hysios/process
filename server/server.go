package server

import (
	"net"
	"net/http"
	"net/rpc"

	"github.com/fatih/structs"
	"github.com/hysios/log"
	"github.com/hysios/process"
)

type Server struct {
	Addr    string
	manager *process.Manager
}

func NewServer(addr string, cfg *process.ManagerConfig) *Server {
	var manager = process.NewManager(cfg)

	return &Server{Addr: addr, manager: manager}
}

func Listen(s *Server) error {
	rpc.Register(s)
	rpc.HandleHTTP()
	l, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	go s.manager.Run()

	return http.Serve(l, nil)
}

type StartReq struct {
	Name string
	Args []string
	Env  []string
	Dir  string
}

func (s *Server) StartProcess(req process.StartReq, reply *process.Process) error {
	process, err := s.manager.StartProcess(req.Name, req.Args, req.Env, req.Dir)
	if err != nil {
		return err
	}
	*reply = *process
	return nil
}

func (s *Server) StopProcess(name string, _ *int) error {
	err := s.manager.StopProcess(name)
	if err != nil {
		return err
	}

	return nil
}

func (s *Server) AllStatus(_ int, status *map[string]interface{}) error {
	log.Infof("call status")
	processes, err := s.manager.AllStatus()
	if err != nil {
		return err
	}

	for _, proc := range processes {
		m := structs.Map(proc)
		m["Status"] = proc.Status()
		m["StartAt"] = proc.StartAt().Format("2006-01-02 15:04:05")
		(*status)[proc.Name] = m
	}

	return nil
}
