package main

import (
	"fmt"
	"github.com/mitchellh/go-homedir"
	"net/http"
	"time"

	"github.com/filecoin-project/lotus/build"
	lcli "github.com/filecoin-project/lotus/cli"
	"github.com/filecoin-project/lotus/node"
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
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "db",
				Value: "extend.db",
				Usage: "specify the database file to use",
			},
			&cli.BoolFlag{
				Name:  "debug",
				Value: false,
				Usage: "enable debug logging",
			},
		},
		Before: func(cctx *cli.Context) error {
			if cctx.Bool("debug") {
				logging.SetLogLevel("extend", "debug")
			} else {
				logging.SetLogLevel("extend", "info")
			}
			return nil
		},
		Action: func(cctx *cli.Context) error {
			dbPath, err := homedir.Expand(cctx.String("db"))
			if err != nil {
				return fmt.Errorf("failed to expand db path: %w", err)
			}
			db, err := NewDB(dbPath)
			if err != nil {
				return err
			}

			fullApi, nCloser, err := lcli.GetFullNodeAPI(cctx)
			if err != nil {
				return err
			}
			defer nCloser()

			ctx := lcli.DaemonContext(cctx)

			shutdownChan := make(chan struct{})

			srv := &http.Server{
				Handler: NewRouter(NewService(ctx, db, fullApi, shutdownChan)),
				Addr:    "127.0.0.1:8000",
				// Good practice: enforce timeouts for servers you create!
				WriteTimeout: 15 * time.Second,
				ReadTimeout:  15 * time.Second,
			}

			go func() {
				log.Infof("Starting API server at %s", srv.Addr)
				if err := srv.ListenAndServe(); err != nil {
					log.Error(err)
				}
			}()

			// Monitor for shutdown.
			finishCh := node.MonitorShutdown(shutdownChan,
				node.ShutdownHandler{Component: "api", StopFunc: srv.Shutdown})
			<-finishCh
			return nil
		},
	}
	app.Setup()
	lcli.RunApp(app)
}
