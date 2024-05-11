package main

import (
	"fmt"
	lcli "github.com/filecoin-project/lotus/cli"
	"github.com/filecoin-project/lotus/node"
	"github.com/mitchellh/go-homedir"
	"github.com/urfave/cli/v2"
	"net/http"
	"time"
)

var authCmd = &cli.Command{
	Name:  "auth",
	Usage: "manage API authentication",
	Subcommands: []*cli.Command{
		authCreateTokenCmd,
		authVerifyCmd,
	},
}

var authCreateTokenCmd = &cli.Command{
	Name:  "create-token",
	Usage: "create a new API token",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "secret",
			Usage:    "specify the secret to use for API authentication",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "user",
			Usage:    "specify the user to associate with the token",
			Required: true,
		},
		&cli.TimestampFlag{
			Name:   "expiry",
			Usage:  "specify the expiry time of the token",
			Layout: time.RFC3339Nano,
		},
	},
	Action: func(cctx *cli.Context) error {
		var d time.Duration
		if cctx.IsSet("expiry") {
			expiry := cctx.Timestamp("expiry")
			d = expiry.Sub(time.Now())
		}

		token, err := authNew([]byte(cctx.String("secret")),
			cctx.String("user"), d)
		if err != nil {
			return err
		}
		fmt.Println("token created successfully:")
		fmt.Println(token)
		return nil
	},
}

var authVerifyCmd = &cli.Command{
	Name:  "verify",
	Usage: "verify an API token",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:     "secret",
			Usage:    "specify the secret to use for API authentication",
			Required: true,
		},
		&cli.StringFlag{
			Name:     "token",
			Usage:    "specify the token to verify",
			Required: true,
		},
	},
	Action: func(cctx *cli.Context) error {
		user, err := authVerify([]byte(cctx.String("secret")), cctx.String("token"))
		if err != nil {
			return err
		}
		fmt.Println("token verified successfully: ", user)
		return nil
	},
}

var runCmd = &cli.Command{
	Name:  "run",
	Usage: "run the extend service",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "db",
			Value: "extend.db",
			Usage: "specify the database file to use",
		},
		&cli.StringFlag{
			Name:  "secret",
			Usage: "specify the secret to use for API authentication, if not set, no auth will be enabled",
		},
		&cli.BoolFlag{
			Name:  "debug",
			Value: false,
			Usage: "enable debug logging",
		},
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

		var secret []byte
		var authStatus = "disabled"
		if cctx.IsSet("secret") {
			secret = []byte(cctx.String("secret"))
			authStatus = "enabled"
		}

		srv := &http.Server{
			Handler: NewRouter(NewService(ctx, db, fullApi, shutdownChan), secret),
			Addr:    "127.0.0.1:8000",
			// Good practice: enforce timeouts for servers you create!
			WriteTimeout: 15 * time.Second,
			ReadTimeout:  15 * time.Second,
		}

		go func() {
			log.Infof("starting API server at %s, authentication is %s", srv.Addr, authStatus)
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
