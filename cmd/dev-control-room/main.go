package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/knowgyu/dev-control-room/internal/app"
	"github.com/knowgyu/dev-control-room/internal/contract"
)

const version = "0.1.0-milestone-0"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		os.Exit(runVersion(os.Args[2:]))
	}
	if len(os.Args) > 1 && os.Args[1] != "serve" && os.Args[1][0] != '-' {
		os.Exit(writeCLIError(contract.InvalidInput("unknown command: " + os.Args[1])))
	}
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "serve" {
		args = args[1:]
	}
	os.Exit(runServe(args))
}

func runVersion(args []string) int {
	jsonOutput := false
	flags := flag.NewFlagSet("version", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.BoolVar(&jsonOutput, "json", false, "emit the stable JSON envelope")
	if err := flags.Parse(args); err != nil {
		return writeCLIError(contract.InvalidInput(err.Error()))
	}
	data := map[string]string{
		"version":     version,
		"milestone":   "0",
		"cli_schema":  contract.EnvelopeSchema,
		"api_version": "devroom/v1alpha1",
	}
	if jsonOutput {
		if err := json.NewEncoder(os.Stdout).Encode(contract.Success(data)); err != nil {
			return int(contract.ExitInternal)
		}
		return int(contract.ExitSuccess)
	}
	fmt.Fprintln(os.Stdout, version)
	return int(contract.ExitSuccess)
}

func runServe(args []string) int {
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	listen := flags.String("listen", "127.0.0.1:38471", "loopback address for the local control room")
	home := flags.String("home", defaultHome(), "local data directory")
	if err := flags.Parse(args); err != nil {
		writeCLIError(contract.InvalidInput(err.Error()))
		return int(contract.ExitInvalidInput)
	}

	service, err := app.New(*home, *listen)
	if err != nil {
		return writeCLIError(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go service.RunDoctor(ctx)

	server := &http.Server{
		Addr:              *listen,
		Handler:           service.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	log.Printf("Dev Control Room listening on http://%s", *listen)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return writeCLIError(err)
	}
	return int(contract.ExitSuccess)
}

func writeCLIError(err error) int {
	classified := contract.Classify(err)
	payload := contract.Failure[map[string]any](classified.Code, classified.Message, classified.Details)
	if encodeErr := json.NewEncoder(os.Stderr).Encode(payload); encodeErr != nil {
		fmt.Fprintln(os.Stderr, classified.Message)
	}
	return int(classified.Code.ExitCode())
}

func defaultHome() string {
	if value := os.Getenv("DEV_CONTROL_ROOM_HOME"); value != "" {
		return value
	}
	if value := os.Getenv("LOCALAPPDATA"); value != "" {
		return filepath.Join(value, "DevControlRoom")
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return filepath.Join(".", "data")
	}
	return filepath.Join(dir, "DevControlRoom")
}

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.LUTC)
	log.SetPrefix("dev-control-room: ")
}
