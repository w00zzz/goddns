package install

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type UninstallOptions struct {
	Platform   string
	DryRun     bool
	Yes        bool
	Purge      bool
	RemoveUser bool
	Input      io.Reader
	Output     io.Writer
}

type UninstallPlan struct {
	Platform string
	Paths    []string
	Actions  []string
}

func BuildUninstallPlan(platform string, purge bool, removeUser bool) (UninstallPlan, error) {
	if platform == "" || platform == "auto" {
		platform = runtime.GOOS
	}
	if platform != "freebsd" && platform != "linux" {
		return UninstallPlan{}, fmt.Errorf("unsupported uninstallation platform %q", platform)
	}
	service := "/usr/local/etc/rc.d/goddns"
	configDir := "/usr/local/etc/goddns"
	stateDir := "/var/db/goddns"
	if platform == "linux" {
		service = "/etc/systemd/system/goddns.service"
		configDir = "/etc/goddns"
		stateDir = "/var/lib/goddns"
	}
	plan := UninstallPlan{
		Platform: platform,
		Paths:    []string{"/usr/local/bin/goddns", service},
		Actions:  []string{"stop the GoDDNS service", "disable the GoDDNS service", "remove the GoDDNS service definition", "remove the GoDDNS binary"},
	}
	if purge {
		plan.Paths = append(plan.Paths, configDir, stateDir, "/var/log/goddns.log")
		plan.Actions = append(plan.Actions, "remove GoDDNS configuration, state and logs")
	} else {
		plan.Actions = append(plan.Actions, "preserve GoDDNS configuration, state and logs")
	}
	if removeUser {
		plan.Actions = append(plan.Actions, "remove the goddns system user")
	}
	return plan, nil
}

func (p UninstallPlan) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "GoDDNS uninstallation plan\n\nPlatform: %s\n\nActions:\n", p.Platform)
	for _, action := range p.Actions {
		fmt.Fprintf(&b, "  - %s\n", action)
	}
	b.WriteString("\nPaths:\n")
	for _, path := range p.Paths {
		fmt.Fprintf(&b, "  - %s\n", path)
	}
	return b.String()
}

func Uninstall(ctx context.Context, opts UninstallOptions) error {
	if opts.Output == nil {
		opts.Output = os.Stdout
	}
	if opts.Input == nil {
		opts.Input = os.Stdin
	}
	plan, err := BuildUninstallPlan(opts.Platform, opts.Purge, opts.RemoveUser)
	if err != nil {
		return err
	}
	fmt.Fprintln(opts.Output, plan.String())
	if opts.DryRun {
		return nil
	}
	if os.Geteuid() != 0 {
		return fmt.Errorf("uninstall requires root privileges; use sudo")
	}
	if !opts.Yes {
		fmt.Fprint(opts.Output, "Continue? [y/N]: ")
		answer, _ := bufio.NewReader(opts.Input).ReadString('\n')
		answer = strings.ToLower(strings.TrimSpace(answer))
		if answer != "y" && answer != "yes" {
			return fmt.Errorf("uninstallation cancelled")
		}
	}
	if plan.Platform == "freebsd" {
		_ = command(ctx, opts.Output, "service", "goddns", "stop")
		_ = command(ctx, opts.Output, "sysrc", "-x", "goddns_enable")
	} else {
		_ = command(ctx, opts.Output, "systemctl", "disable", "--now", "goddns")
	}
	for _, path := range plan.Paths {
		if err := os.RemoveAll(path); err != nil {
			return fmt.Errorf("remove %s: %w", path, err)
		}
	}
	if opts.RemoveUser {
		if plan.Platform == "freebsd" {
			_ = command(ctx, opts.Output, "pw", "userdel", "goddns")
		} else {
			_ = command(ctx, opts.Output, "userdel", "goddns")
		}
	}
	if plan.Platform == "linux" {
		_ = command(ctx, opts.Output, "systemctl", "daemon-reload")
	}
	return nil
}

func ensureDirectory(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0755)
}
