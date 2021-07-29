package process

import (
	"encoding/gob"
)

type StartReq struct {
	Name   string
	Binary string
	Args   []string
	Env    []string
	Dir    string
}

func init() {
	gob.Register(new(StartReq))
	gob.Register(new(Process))

}
