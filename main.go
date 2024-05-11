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
		Version:              build.UserVersion(),
		EnableBashCompletion: true,
		Commands: []*cli.Command{
			runCmd,
			authCmd,
		},
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "debug",
				Value: false,
				Usage: "enable debug logging",
			},
		},
		Before: func(cctx *cli.Context) error {
			if cctx.Bool("debug") {
				_ = logging.SetLogLevel("extend", "debug")
			} else {
				_ = logging.SetLogLevel("extend", "info")
			}
			return nil
		},
	}
	app.Setup()
	lcli.RunApp(app)
}
