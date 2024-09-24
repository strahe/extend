package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/filecoin-project/lotus/build"

	lcli "github.com/filecoin-project/lotus/cli"
	"github.com/filecoin-project/lotus/node"
	logging "github.com/ipfs/go-log/v2"
	"github.com/mitchellh/go-homedir"
	"github.com/urfave/cli/v2"
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
			d = time.Until(*expiry)
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
			Name:  "listen",
			Value: "127.0.0.1:8000",
			Usage: "specify the address to listen on",
		},
		&cli.StringFlag{
			Name:  "db",
			Value: "sqlite3:extend.db",
			Usage: "specify the database URL to use， support sqlite3, mysql, postgres, https://github.com/xo/dburl?tab=readme-ov-file#example-urls",
		},
		&cli.StringFlag{
			Name:  "secret",
			Usage: "specify the secret to use for API authentication, if not set, no auth will be enabled",
		},
		&cli.DurationFlag{
			Name:  "max-wait",
			Usage: "[Warning] specify the maximum time to wait for messages on chain, otherwise try to replace them, only use this if you know what you are doing",
		},
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
	Action: func(cctx *cli.Context) error {
		dbPath, err := homedir.Expand(cctx.String("db"))
		if err != nil {
			return fmt.Errorf("failed to expand db path: %w", err)
		}
		db, err := NewDB(dbPath)
		if err != nil {
			return err
		}

		fullApi, nCloser, err := GetFullNodeAPI(cctx)
		if err != nil {
			return err
		}
		defer nCloser()

		ctx := lcli.ReqContext(cctx)

		gtp, err := fullApi.ChainGetGenesis(ctx)
		if err != nil {
			return err
		}
		genesisTime := time.Unix(int64(gtp.MinTimestamp()), 0)
		SetupGenesisTime(genesisTime)

		nn, err := fullApi.StateNetworkName(ctx)
		if err != nil {
			return err
		}
		if err := build.UseNetworkBundle(string(nn)); err != nil {
			return err
		}
		head, err := fullApi.ChainHead(ctx)
		if err != nil {
			return err
		}
		log.Infow("connected to lotus node",
			"network", nn, "head", head.Height(), "genesis", genesisTime)

		var secret []byte
		var authStatus = "disabled"
		if cctx.IsSet("secret") {
			secret = []byte(cctx.String("secret"))
			authStatus = "enabled"
		}

		service := NewService(ctx, db, fullApi, cctx.Duration("max-wait"))
		srv := &http.Server{
			Handler: NewRouter(service, secret),
			Addr:    cctx.String("listen"),
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
		finishCh := MonitorShutdown(
			node.ShutdownHandler{Component: "api", StopFunc: srv.Shutdown},
			node.ShutdownHandler{Component: "service", StopFunc: service.Shutdown},
		)
		<-finishCh
		return nil
	},
}

var testCmd = &cli.Command{
	Name:   "test",
	Usage:  "test some stuff in the context of the extend service",
	Hidden: true,
	Action: func(cctx *cli.Context) error {
		return nil
	},
}

func MonitorShutdown(handlers ...node.ShutdownHandler) <-chan struct{} {
	sigCh := make(chan os.Signal, 2)
	out := make(chan struct{})

	go func() {
		sig := <-sigCh
		log.Warnw("received shutdown", "signal", sig)
		log.Warn("Shutting down...")

		// Call all the handlers, logging on failure and success.
		for _, h := range handlers {
			if err := h.StopFunc(context.TODO()); err != nil {
				log.Errorf("shutting down %s failed: %s", h.Component, err)
				continue
			}
			log.Infof("%s shut down successfully ", h.Component)
		}

		log.Warn("Graceful shutdown successful")

		// Sync all loggers.
		_ = log.Sync() //nolint:errcheck
		close(out)
	}()

	signal.Reset(syscall.SIGTERM, syscall.SIGINT)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	return out
}
