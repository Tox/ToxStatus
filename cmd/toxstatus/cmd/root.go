package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/Tox/ToxStatus/internal/crawler"
	"github.com/alexbakker/tox4go/toxstatus"
	"github.com/lmittmann/tint"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var (
	Root = &cobra.Command{
		Use:   "toxstatus",
		Short: "Status page for the Tox network that keeps track of bootstrap nodes",
		Run:   startRoot,
	}
	rootFlags = struct {
		HTTPAddr          string
		HTTPClientTimeout time.Duration
		ToxUDPAddr        string
		DB                string
		LogLevel          string
		Workers           int
	}{}
)

func init() {
	const maxDefaultWorkers = 8
	Root.Flags().StringVar(&rootFlags.HTTPAddr, "http-addr", ":8003", "the network address to listen on for the HTTP server")
	Root.Flags().DurationVar(&rootFlags.HTTPClientTimeout, "http-client-timeout", 10*time.Second, "the http client timeout for requests to nodes.tox.chat")
	Root.Flags().StringVar(&rootFlags.ToxUDPAddr, "tox-udp-addr", ":33450", "the UDP network address to listen on for Tox")
	//root.Flags().StringVar(&rootFlags.DB, "db", "", "the sqlite database to use")
	Root.Flags().StringVar(&rootFlags.LogLevel, "log-level", "info", "the log level to use")
	Root.Flags().IntVar(&rootFlags.Workers, "workers", min(maxDefaultWorkers, runtime.NumCPU()), "the amount of workers to use")
	//Root.MarkFlagRequired("db")
}

func startRoot(cmd *cobra.Command, args []string) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(rootFlags.LogLevel)); err != nil {
		fmt.Fprintf(os.Stderr, "bad log level: %s\n", rootFlags.LogLevel)
		os.Exit(1)
		return
	}

	logger := slog.New(tint.NewHandler(os.Stderr, &tint.Options{
		Level:   level,
		NoColor: !isatty.IsTerminal(os.Stderr.Fd()),
	}))

	cr, err := crawler.New(crawler.CrawlerOptions{
		HTTPAddr:   rootFlags.HTTPAddr,
		ToxUDPAddr: rootFlags.ToxUDPAddr,
		Workers:    rootFlags.Workers,
	}, logger)
	if err != nil {
		logErrorAndExit(logger, "Unable to initialize Tox crawler", slog.Any("err", err))
		return
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger.Info("Querying nodes.tox.chat for bootstrap nodes")

	// Kick off by bootstrapping from nodes in the nodes.tox.chat list
	tsClient := toxstatus.Client{HTTPClient: &http.Client{Timeout: rootFlags.HTTPClientTimeout}}
	bsNodes, err := tsClient.GetNodes(ctx)
	if err != nil {
		logErrorAndExit(logger, "Unable to fetch nodes from", slog.Any("err", err))
		return
	}

	for _, node := range bsNodes {
		logger.Debug("Found bootstrap node",
			slog.String("public_key", node.PublicKey.String()),
			slog.String("net", node.Type.Net()),
			slog.String("addr", node.Addr().String()))
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		logger.Info("Starting Tox crawler")

		if err := cr.Run(ctx, bsNodes); err != nil && !errors.Is(err, context.Canceled) {
			logErrorAndExit(logger, "Unable to run Tox crawler", slog.Any("err", err))
		}
	}()

	<-ctx.Done()
	logger.Info("Stopping Tox crawler")
	wg.Wait()

	logger.Info("Bye!")
}

func logErrorAndExit(logger *slog.Logger, msg string, args ...any) {
	logger.Error(msg, args...)
	os.Exit(1)
}
