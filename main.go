package main

import (
	"fmt"

	"github.com/filecoin-project/lotus/build"
	lcli "github.com/filecoin-project/lotus/cli"
	logging "github.com/ipfs/go-log/v2"
	"github.com/urfave/cli/v2"
)

const version = "0.2.2"

var log = logging.Logger("extend")

func main() {
	app := &cli.App{
		Name:                 "extend",
		Usage:                "A tool and service to extend filecoin sector lifetime",
		Version:              fmt.Sprintf("%s-lotus-%s", version, build.NodeUserVersion()),
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
