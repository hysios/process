package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/hysios/log"
	"github.com/hysios/process/client"
	"github.com/olekukonko/tablewriter"

	"github.com/hysios/process/server"
)

var (
	climode bool
	run     bool
	status  bool
	stop    bool
)

func init() {
	flag.BoolVar(&climode, "client", false, "Client Mode")
	flag.BoolVar(&run, "run", false, "Run cmd")
	flag.BoolVar(&status, "status", false, "List All Processes Status")
	flag.BoolVar(&stop, "stop", false, "Stop Process running")

}

func main() {
	flag.Parse()
	if climode {
		cli, err := client.Open(&client.ClientOption{
			Addr: ":1234",
		})
		if err != nil {
			log.Fatalf("open client error %s", err)
		}

		switch {
		case run:
			{
				log.Infof("run: %s", flag.Args())

				if len(flag.Args()) > 0 {
					var args = flag.Args()

					proc, err := cli.StartProcess(args[0], args[1:], nil, args[0])
					if err != nil {
						log.Fatalf("start process %s", err)
					}
					log.Infof("process %s started", proc.Pid)
				} else {
					log.Fatalf("you must input cmd and args...")
				}
				status, err := cli.AllStatus()
				if err != nil {
					log.Fatalf("all status %s", err)
				}
				printTable(status)
			}
		case status:
			{

				status, err := cli.AllStatus()
				if err != nil {
					log.Fatalf("all status %s", err)
				}
				printTable(status)
				// log.Infof("all status %s", status)
			}
		case stop:
			log.Infof("stop %s", flag.Args()[0])
			if err := cli.StopProcess(flag.Args()[0]); err != nil {
				log.Fatalf("stop error %s", err)
			}

			status, err := cli.AllStatus()
			if err != nil {
				log.Fatalf("all status %s", err)
			}
			printTable(status)
		default:
			flag.Usage()
		}
	} else {
		s := server.NewServer(":1234", nil)
		log.Infof("process server listen on %s", s.Addr)
		log.Fatal(server.Listen(s))
	}
}

func printTable(status map[string]interface{}) {
	var (
		data    = make([][]string, 0)
		columns = []string{"Name"}
		first   bool
	)

	for name, val := range status {
		var row = []string{name}

		if first {
			vv, ok := val.(map[string]interface{})
			if !ok {
				continue
			}

			for _, col := range columns[1:] {
				row = append(row, fmt.Sprintf("%v", vv[col]))
			}
		} else {
			for col, v := range val.(map[string]interface{}) {

				row = append(row, fmt.Sprintf("%v", v))
				if !first {
					columns = append(columns, col)
				}
			}
			first = true
		}
		data = append(data, row)
	}

	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(columns)

	for _, v := range data {
		table.Append(v)
	}
	table.Render() // Send output
}
