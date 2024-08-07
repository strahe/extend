package main

import (
	"github.com/filecoin-project/lotus/build"
	lcli "github.com/filecoin-project/lotus/cli"
	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"
)

var log = logging.Logger("extend")

func main() {
	app := &cli.App{
		Name:                 "extend",
		Usage:                "A tool and service to extend filecoin sector lifetime",
		Version:              string(build.NodeUserVersion()),
		EnableBashCompletion: true,
		Commands: []*cli.Command{
			runCmd,
			authCmd,
			testCmd,
		},
	}
	app.Setup()
	lcli.RunApp(app)
}
