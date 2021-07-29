package client

import (
	"encoding/gob"
	"net/rpc"

	"github.com/hysios/process"
)

type Client struct {
	*rpc.Client
}

type ClientOption struct {
	Addr string
}

var DefaultOption = ClientOption{
	Addr: ":6789",
}

func Open(opt *ClientOption) (*Client, error) {
	if opt == nil {
		_opt := DefaultOption
		opt = &_opt
	}

	client, err := rpc.DialHTTP("tcp", opt.Addr)
	if err != nil {
		return nil, err
	}
	var cli = Client{client}

	return &cli, nil
}

func (cli *Client) StartProcess(name string, fullbin string, args []string, env []string, dir string) (*process.Process, error) {
	var (
		reply process.Process
		err   error
	)
	if err = cli.Call("Server.StartProcess", process.StartReq{
		Name:   name,
		Binary: fullbin,
		Args:   args,
		Env:    env,
		Dir:    dir,
	}, &reply); err != nil {
		return nil, err
	}
	return &reply, nil
}

// RestartProcess restarts a process
func (cli *Client) RestartProcess(name string) error {
	if err := cli.Call("RestartProcess", []interface{}{
		name,
	}, nil); err != nil {
		return err
	}

	return nil
}

// LoadProcesses load process in config file
func (cli *Client) LoadProcesses(filename string) error {

	if err := cli.Call("LoadProcesses", []interface{}{
		filename,
	}, nil); err != nil {
		return err
	}

	return nil
}

func (cli *Client) StopProcess(name string) error {
	if err := cli.Call("Server.StopProcess", name, nil); err != nil {
		return err
	}
	return nil
}

func (cli *Client) RemoveProcess(name string) error {
	if err := cli.Call("Server.RemoveProcess", name, nil); err != nil {
		return err
	}
	return nil
}

func (cli *Client) AllStatus() (map[string]interface{}, error) {
	var processes = make(map[string]interface{})
	if err := cli.Call("Server.AllStatus", 0, &processes); err != nil {
		return nil, err
	}
	return processes, nil
}

func init() {
	gob.Register(make(map[string]interface{}))
}
