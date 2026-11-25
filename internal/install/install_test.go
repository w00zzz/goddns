package install

import (
	"os"
	"strings"
	"testing"
)

func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0600)
}

func TestBuildPlan(t *testing.T) {
	configPath := t.TempDir() + "/config.json"
	content := `{"general":{"interval":"1m","log_level":"info"},"dns":{"provider":"simply","domain":"example.com","host":"home","credentials":{"account":"account","api_key":"key"}},"discovery":{"ipv4":true,"sources":["https://api.ipify.org"],"min_consensus":1},"state":{"path":"/var/db/goddns/state.json"}}`
	if err := writeTestFile(configPath, content); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(Options{ConfigPath: configPath, Platform: "freebsd", Executable: "/tmp/goddns"})
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	if plan.TargetConfig != "/usr/local/etc/goddns/config.json" || plan.ServicePath != "/usr/local/etc/rc.d/goddns" {
		t.Fatalf("unexpected FreeBSD plan: %+v", plan)
	}
	if !strings.Contains(plan.String(), "create or verify the goddns system user") {
		t.Fatal("plan does not describe user creation")
	}
}

func TestBuildLinuxPlan(t *testing.T) {
	configPath := t.TempDir() + "/config.json"
	content := `{"general":{"interval":"1m","log_level":"info"},"dns":{"provider":"simply","domain":"example.com","host":"home","credentials":{"account":"account","api_key":"key"}},"discovery":{"ipv4":true,"sources":["https://api.ipify.org"],"min_consensus":1},"state":{"path":"/var/lib/goddns/state.json"}}`
	if err := writeTestFile(configPath, content); err != nil {
		t.Fatal(err)
	}
	plan, err := BuildPlan(Options{ConfigPath: configPath, Platform: "linux", Executable: "/tmp/goddns"})
	if err != nil {
		t.Fatalf("BuildPlan() error = %v", err)
	}
	if plan.TargetConfig != "/etc/goddns/config.json" || plan.ServicePath != "/etc/systemd/system/goddns.service" {
		t.Fatalf("unexpected Linux plan: %+v", plan)
	}
}
