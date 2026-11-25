package install

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/w00zzz/goddns/internal/config"
)

type Options struct {
	ConfigPath string
	Platform   string
	DryRun     bool
	Yes        bool
	Enable     bool
	Start      bool
	Executable string
	Input      io.Reader
	Output     io.Writer
}

type Plan struct {
	Platform      string
	Executable    string
	TargetBinary  string
	TargetConfig  string
	StatePath     string
	ServicePath   string
	ServiceAction string
	Actions       []string
}

func BuildPlan(opts Options) (Plan, error) {
	if opts.ConfigPath == "" {
		return Plan{}, fmt.Errorf("configuration path is required")
	}
	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return Plan{}, err
	}
	platform := opts.Platform
	if platform == "" || platform == "auto" {
		platform = runtime.GOOS
	}
	if platform != "freebsd" && platform != "linux" {
		return Plan{}, fmt.Errorf("unsupported installation platform %q", platform)
	}
	executable := opts.Executable
	if executable == "" {
		executable, err = os.Executable()
		if err != nil {
			return Plan{}, fmt.Errorf("resolve executable: %w", err)
		}
	}
	plan := Plan{
		Platform:     platform,
		Executable:   executable,
		TargetBinary: "/usr/local/bin/goddns",
		TargetConfig: "/usr/local/etc/goddns/config.json",
		StatePath:    cfg.State.Path,
		Actions: []string{
			"create or verify the goddns system user",
			"create required directories with secure permissions",
			"install the GoDDNS binary",
			"backup and install the JSON configuration",
			"install the service definition",
		},
	}
	if platform == "freebsd" {
		plan.ServicePath = "/usr/local/etc/rc.d/goddns"
		plan.ServiceAction = "service goddns"
		if opts.Enable {
			plan.Actions = append(plan.Actions, "enable the FreeBSD rc.d service")
		}
	} else {
		plan.TargetConfig = "/etc/goddns/config.json"
		plan.ServicePath = "/etc/systemd/system/goddns.service"
		plan.ServiceAction = "systemctl"
		if opts.Enable {
			plan.Actions = append(plan.Actions, "enable the systemd service")
		}
	}
	if opts.Start {
		plan.Actions = append(plan.Actions, "start the service")
	}
	return plan, nil
}

func (p Plan) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "GoDDNS installation plan\n\nPlatform: %s\nExecutable: %s\nBinary: %s\nConfig: %s\nState: %s\nService: %s\n\nActions:\n", p.Platform, p.Executable, p.TargetBinary, p.TargetConfig, p.StatePath, p.ServicePath)
	for _, action := range p.Actions {
		fmt.Fprintf(&b, "  - %s\n", action)
	}
	return b.String()
}

func Run(ctx context.Context, opts Options) error {
	if opts.Output == nil {
		opts.Output = os.Stdout
	}
	if opts.Input == nil {
		opts.Input = os.Stdin
	}
	plan, err := BuildPlan(opts)
	if err != nil {
		return err
	}
	fmt.Fprintln(opts.Output, plan.String())
	if opts.DryRun {
		return nil
	}
	if os.Geteuid() != 0 {
		return fmt.Errorf("install requires root privileges; use sudo")
	}
	if !opts.Yes {
		fmt.Fprint(opts.Output, "Continue? [y/N]: ")
		answer, _ := bufio.NewReader(opts.Input).ReadString('\n')
		if strings.ToLower(strings.TrimSpace(answer)) != "y" && strings.ToLower(strings.TrimSpace(answer)) != "yes" {
			return fmt.Errorf("installation cancelled")
		}
	}
	return executePlan(ctx, opts, plan)
}

func executePlan(ctx context.Context, opts Options, plan Plan) error {
	if _, err := user.Lookup("goddns"); err != nil {
		var unknown user.UnknownUserError
		if !errors.As(err, &unknown) {
			return fmt.Errorf("look up goddns user: %w", err)
		}
		if plan.Platform == "freebsd" {
			if err := command(ctx, opts.Output, "pw", "useradd", "goddns", "-d", "/nonexistent", "-s", "/usr/sbin/nologin"); err != nil {
				return err
			}
		} else if err := command(ctx, opts.Output, "useradd", "--system", "--home-dir", "/var/lib/goddns", "--shell", "/usr/sbin/nologin", "goddns"); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(plan.TargetConfig), 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(plan.StatePath), 0750); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	if err := os.Chown(filepath.Dir(plan.StatePath), lookupUID("goddns"), lookupGID("goddns")); err != nil {
		return fmt.Errorf("set state directory owner: %w", err)
	}
	if err := copyFile(plan.Executable, plan.TargetBinary, 0755); err != nil {
		return err
	}
	if _, err := os.Stat(plan.TargetConfig); err == nil {
		if err := os.Rename(plan.TargetConfig, plan.TargetConfig+".bak"); err != nil {
			return fmt.Errorf("backup existing configuration: %w", err)
		}
	}
	if err := copyFile(opts.ConfigPath, plan.TargetConfig, 0640); err != nil {
		return err
	}
	if err := os.Chown(plan.TargetConfig, 0, lookupGID("goddns")); err != nil {
		return fmt.Errorf("set configuration owner: %w", err)
	}
	if err := os.WriteFile(plan.ServicePath, []byte(serviceDefinition(plan.Platform)), 0555); err != nil {
		return fmt.Errorf("write service definition: %w", err)
	}
	if plan.Platform == "linux" {
		if err := command(ctx, opts.Output, "systemctl", "daemon-reload"); err != nil {
			return err
		}
		if opts.Enable {
			if err := command(ctx, opts.Output, "systemctl", "enable", "goddns"); err != nil {
				return err
			}
		}
		if opts.Start {
			return command(ctx, opts.Output, "systemctl", "start", "goddns")
		}
	} else {
		if opts.Enable && command(ctx, opts.Output, "sysrc", "goddns_enable=YES") != nil {
			return fmt.Errorf("enable FreeBSD service")
		}
		if opts.Start {
			return command(ctx, opts.Output, "service", "goddns", "start")
		}
	}
	return nil
}

func command(ctx context.Context, output io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout, cmd.Stderr = output, output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run %s: %w", name, err)
	}
	return nil
}

func copyFile(source, target string, mode os.FileMode) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("read %s: %w", source, err)
	}
	if err := os.WriteFile(target, data, mode); err != nil {
		return fmt.Errorf("write %s: %w", target, err)
	}
	return os.Chmod(target, mode)
}

func lookupUID(name string) int {
	u, _ := user.Lookup(name)
	if u == nil {
		return -1
	}
	var id int
	fmt.Sscanf(u.Uid, "%d", &id)
	return id
}
func lookupGID(name string) int {
	u, _ := user.Lookup(name)
	if u == nil {
		return -1
	}
	var id int
	fmt.Sscanf(u.Gid, "%d", &id)
	return id
}

func serviceDefinition(platform string) string {
	if platform == "freebsd" {
		return `#!/bin/sh
# PROVIDE: goddns
# REQUIRE: NETWORKING
# KEYWORD: shutdown
. /etc/rc.subr
name="goddns"
rcvar="goddns_enable"
load_rc_config "$name"
: "${goddns_enable:="NO"}"
command="/usr/sbin/daemon"
pidfile="/var/run/goddns.pid"
procname="/usr/local/bin/goddns"
command_args="-p ${pidfile} -u goddns -o /var/log/goddns.log ${procname} run --config /usr/local/etc/goddns/config.json"
required_files="/usr/local/etc/goddns/config.json"
run_rc_command "$1"
`
	}
	return `[Unit]
Description=GoDDNS dynamic DNS updater
Wants=network-online.target
After=network-online.target

[Service]
Type=simple
User=goddns
Group=goddns
ExecStart=/usr/local/bin/goddns run --config /etc/goddns/config.json
Restart=on-failure
RestartSec=10s
NoNewPrivileges=true
PrivateTmp=true
ProtectHome=true
ProtectSystem=strict
ReadWritePaths=/var/lib/goddns
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
`
}
