package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/w00zzz/goddns/internal/config"
	"github.com/w00zzz/goddns/internal/discovery"
	"github.com/w00zzz/goddns/internal/install"
	"github.com/w00zzz/goddns/internal/logger"
	"github.com/w00zzz/goddns/internal/monitor"
	"github.com/w00zzz/goddns/internal/provider"
	"github.com/w00zzz/goddns/internal/provider/simply"
	"github.com/w00zzz/goddns/internal/storage"
)

const (
	version = "0.1.0-dev"
	creator = "lumiar.info"
)

func main() {
	if err := execute(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func execute(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return printHelp(stdout, "")
	}
	if args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		command := ""
		if args[0] == "help" && len(args) > 1 {
			command = args[1]
		}
		return printHelp(stdout, command)
	}
	switch args[0] {
	case "run":
		return runCommand(args[1:], stdout, stderr)
	case "check":
		return checkCommand(args[1:], stdout, stderr)
	case "sync":
		return syncCommand(args[1:], stdout, stderr)
	case "install":
		return installCommand(args[1:], stdout, stderr)
	case "status":
		return statusCommand(args[1:], stdout, stderr)
	case "version":
		if len(args) != 1 {
			return fmt.Errorf("version does not accept arguments")
		}
		_, err := fmt.Fprintln(stdout, version)
		return err
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runCommand(args []string, stdout, stderr io.Writer) error {
	if hasHelpFlag(args) {
		return printHelp(stdout, "run")
	}
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "config.json", "path to JSON configuration")
	verbose := fs.Bool("verbose", false, "enable detailed logs")
	logFile := fs.String("log-file", "", "write logs to this file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, dnsProvider, sources, err := loadRuntime(*configPath)
	if err != nil {
		return err
	}
	log, closeLog, err := newLogger(cfg.General.LogLevel, cfg.General.LogFile, *logFile, *verbose, stderr)
	if err != nil {
		return err
	}
	defer closeLog()
	log.Info("starting GoDDNS", "version", version)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	m := monitor.NewWithDependencies(cfg, dnsProvider, func(ctx context.Context) (net.IP, error) {
		return discovery.DiscoverWithLogger(ctx, sources, cfg.Discovery.MinConsensus, log)
	})
	m.SetLogger(log)
	if err := m.Run(ctx); err != nil {
		return err
	}
	log.Info("GoDDNS stopped")
	return nil
}

func checkCommand(args []string, stdout, stderr io.Writer) error {
	if hasHelpFlag(args) {
		return printHelp(stdout, "check")
	}
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "config.json", "path to JSON configuration")
	verbose := fs.Bool("verbose", false, "enable detailed logs")
	logFile := fs.String("log-file", "", "write logs to this file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, _, sources, err := loadRuntime(*configPath)
	if err != nil {
		return err
	}
	var log *slog.Logger
	var closeLog func()
	if *verbose || *logFile != "" || cfg.General.LogFile != "" {
		log, closeLog, err = newLogger(cfg.General.LogLevel, cfg.General.LogFile, *logFile, *verbose, stderr)
		if err != nil {
			return err
		}
		defer closeLog()
	}
	ctx := context.Background()
	ip, err := discovery.DiscoverWithLogger(ctx, sources, cfg.Discovery.MinConsensus, log)
	if err != nil {
		return fmt.Errorf("discover public IP: %w", err)
	}
	_, err = fmt.Fprintln(stdout, ip.String())
	return err
}

func installCommand(args []string, stdout, stderr io.Writer) error {
	if hasHelpFlag(args) {
		return printHelp(stdout, "install")
	}
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "config.json", "path to JSON configuration")
	platform := fs.String("platform", "auto", "installation platform: auto, freebsd, or linux")
	dryRun := fs.Bool("dry-run", false, "show the plan without changing the system")
	yes := fs.Bool("yes", false, "skip confirmation")
	enable := fs.Bool("enable", false, "enable the service at boot")
	start := fs.Bool("start", false, "start the service after installation")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return install.Run(context.Background(), install.Options{ConfigPath: *configPath, Platform: *platform, DryRun: *dryRun, Yes: *yes, Enable: *enable, Start: *start, Output: stdout})
}

func syncCommand(args []string, stdout, stderr io.Writer) error {
	if hasHelpFlag(args) {
		return printHelp(stdout, "sync")
	}
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "config.json", "path to JSON configuration")
	verbose := fs.Bool("verbose", false, "enable detailed logs")
	logFile := fs.String("log-file", "", "write logs to this file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, dnsProvider, sources, err := loadRuntime(*configPath)
	if err != nil {
		return err
	}
	log, closeLog, err := newLogger(cfg.General.LogLevel, cfg.General.LogFile, *logFile, *verbose, stderr)
	if err != nil {
		return err
	}
	defer closeLog()
	m := monitor.NewWithDependencies(cfg, dnsProvider, func(ctx context.Context) (net.IP, error) {
		return discovery.DiscoverWithLogger(ctx, sources, cfg.Discovery.MinConsensus, log)
	})
	m.SetLogger(log)
	result, err := m.Check(context.Background())
	if err != nil {
		return fmt.Errorf("synchronization failed: %w", err)
	}
	if result.Updated {
		_, err = fmt.Fprintf(stdout, "synchronization successful: %s\\n", result.CurrentIP)
	} else {
		_, err = fmt.Fprintf(stdout, "synchronization complete: IP unchanged (%s)\\n", result.CurrentIP)
	}
	return err
}

func statusCommand(args []string, stdout, stderr io.Writer) error {
	if hasHelpFlag(args) {
		return printHelp(stdout, "status")
	}
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	configPath := fs.String("config", "config.json", "path to JSON configuration")
	verbose := fs.Bool("verbose", false, "enable detailed logs")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(*configPath)
	if err != nil {
		return err
	}
	state, err := storage.Load(cfg.State.Path)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			state = storage.State{}
		} else {
			return err
		}
	}
	if *verbose {
		fmt.Fprintln(stderr, "INFO persisted state loaded")
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(state)
}

func newLogger(level, configuredPath, overridePath string, verbose bool, stderr io.Writer) (*slog.Logger, func(), error) {
	path := overridePath
	if path == "" {
		path = configuredPath
	}
	output := stderr
	var file *os.File
	if path != "" {
		var err error
		file, err = os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return nil, func() {}, fmt.Errorf("open log file: %w", err)
		}
		output = file
	}
	if verbose {
		level = "debug"
	}
	log, err := logger.New(level, output)
	if err != nil {
		if file != nil {
			_ = file.Close()
		}
		return nil, func() {}, err
	}
	return log, func() {
		if file != nil {
			_ = file.Close()
		}
	}, nil
}

func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

func printHelp(w io.Writer, command string) error {
	var text string
	switch command {
	case "", "help":
		text = fmt.Sprintf(`GoDDNS - dynamic DNS updater

Creator: %s
Version: %s

Usage:
  goddns <command> [options]

Commands:
  run       Run the monitor in the foreground
  check     Check the public IP without changing DNS
  sync      Check and update DNS once, then exit
  install   Install GoDDNS as a system service
  status    Show the persisted state
  version   Show the version
  help      Show this help or command-specific help

Common options:
  -h, --help    Show help

Examples:
  goddns check --config ./config.json
  goddns run --config /etc/goddns/config.json
  goddns sync --config ./config.json
  goddns status --config /var/lib/goddns/config.json
  goddns version
  goddns help run
`, creator, version)
	case "install":
		text = `GoDDNS install - install the service

Usage:
  sudo goddns install --config PATH [options]

Options:
  --platform NAME  auto, freebsd, or linux (default: auto)
  --dry-run        Show the plan without changing the system
  --yes            Skip confirmation
  --enable         Enable the service at boot
  --start          Start the service after installation
  -h, --help       Show help

Examples:
  goddns install --dry-run --config ./config.json
  sudo goddns install --config ./config.json --enable --start

The installer displays its plan and asks for confirmation unless --yes is used.
`
	case "sync":
		text = `GoDDNS sync - update DNS once and exit

Usage:
  goddns sync [--config PATH] [--verbose] [--log-file PATH]

Options:
  --config PATH    Path to the JSON file (default: config.json)
  --verbose        Enable detailed source and update logs
  --log-file PATH  Write logs to this file
  -h, --help       Show help

Example:
  goddns sync --config ./config.json

This command performs one synchronization and exits.
`
	case "run":
		text = `GoDDNS run - run the monitor in the foreground

Usage:
  goddns run [--config PATH] [--verbose] [--log-file PATH]

Options:
  --config PATH    Path to the JSON file (default: config.json)
  --verbose        Enable detailed source and monitor logs
  --log-file PATH  Write logs to this file
  -h, --help       Show help

Examples:
  goddns run
  goddns run --config /etc/goddns/config.json

The process stays in the foreground. FreeBSD/systemd should manage its lifecycle.
`
	case "check":
		text = `GoDDNS check - check the public IP

Usage:
  goddns check [--config PATH] [--verbose] [--log-file PATH]

Options:
  --config PATH    Path to the JSON file (default: config.json)
  --verbose        Enable detailed source logs
  --log-file PATH  Write logs to this file
  -h, --help       Show help

Example:
  goddns check --config ./config.json

This command does not change DNS.
`
	case "status":
		text = `GoDDNS status - show the persisted state

Usage:
  goddns status [--config PATH] [--verbose]

Options:
  --config PATH    Path to the JSON file (default: config.json)
  --verbose        Show status log messages
  -h, --help       Show help

Example:
  goddns status --config /etc/goddns/config.json
`
	case "version":
		text = fmt.Sprintf("GoDDNS version %s\\nCreator: %s\\n", version, creator)
	default:
		return fmt.Errorf("unknown help topic %q", command)
	}
	_, err := io.WriteString(w, text)
	return err
}

func loadRuntime(path string) (config.Config, provider.DNSProvider, []discovery.IPSource, error) {
	cfg, err := config.Load(path)
	if err != nil {
		return config.Config{}, nil, nil, err
	}
	credentials := cfg.DNS.Credentials
	dnsProvider := simply.NewConfigured(simply.Config{
		Mode:         simply.Mode(cfg.DNS.Mode),
		Account:      credentials["account"],
		APIKey:       credentials["api_key"],
		Product:      cfg.DNS.Product,
		RecordID:     cfg.DNS.RecordID,
		TTL:          cfg.DNS.TTL,
		DDNSEndpoint: cfg.DNS.DDNSEndpoint,
		APIEndpoint:  cfg.DNS.APIEndpoint,
	})
	registry := provider.NewRegistry()
	if err := registry.Register(dnsProvider); err != nil {
		return config.Config{}, nil, nil, err
	}
	selected, err := registry.Get(cfg.DNS.Provider)
	if err != nil {
		return config.Config{}, nil, nil, err
	}
	return *cfg, selected, discovery.NewSources(cfg.Discovery.Sources, nil), nil
}
