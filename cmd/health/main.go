package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	appagent "comp-health/internal/app/agent"
	appserver "comp-health/internal/app/server"
	"comp-health/internal/config"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("health failed: %v", err)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return usageError()
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch os.Args[1] {
	case "server":
		cfgPath, err := parseConfigFlag(os.Args[2:])
		if err != nil {
			return err
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return err
		}
		return appserver.Run(ctx, cfg)
	case "agent":
		cfgPath, err := parseConfigFlag(os.Args[2:])
		if err != nil {
			return err
		}
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return err
		}
		return appagent.Run(ctx, cfg)
	case "init-config":
		mode, out, err := parseInitFlags(os.Args[2:])
		if err != nil {
			return err
		}
		return config.WriteExample(mode, out)
	default:
		return usageError()
	}
}

func parseConfigFlag(args []string) (string, error) {
	fs := flag.NewFlagSet("health", flag.ContinueOnError)
	cfgPath := fs.String("c", "config.yaml", "config file path")
	fs.SetOutput(os.Stdout)
	if err := fs.Parse(args); err != nil {
		return "", err
	}
	return *cfgPath, nil
}

func parseInitFlags(args []string) (string, string, error) {
	fs := flag.NewFlagSet("init-config", flag.ContinueOnError)
	mode := fs.String("mode", "server", "config mode: server or agent")
	out := fs.String("out", "config.yaml", "output path")
	fs.SetOutput(os.Stdout)
	if err := fs.Parse(args); err != nil {
		return "", "", err
	}
	if *mode != "server" && *mode != "agent" {
		return "", "", errors.New("mode must be server or agent")
	}
	return *mode, *out, nil
}

func usageError() error {
	return fmt.Errorf("usage: health <server|agent|init-config> [-c config.yaml]")
}
