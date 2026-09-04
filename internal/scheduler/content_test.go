package scheduler_test

import (
	"strings"
	"testing"
	"time"

	"github.com/doriansobacki/agentpack/internal/scheduler"
)

func cfg(interval time.Duration) scheduler.Config {
	return scheduler.Config{
		Executable: `/usr/local/bin/agentpack`,
		Interval:   interval,
		LogDir:     `/home/dev/.agentpack/logs`,
	}
}

func TestSchtasksArgsRoundsToMinutes(t *testing.T) {
	args := scheduler.SchtasksCreateArgs(scheduler.Config{Executable: `C:\bin\agentpack.exe`, Interval: 5 * time.Minute})
	joined := strings.Join(args, " ")
	for _, want := range []string{"/SC MINUTE /MO 5", "/TN " + scheduler.TaskName, `"C:\bin\agentpack.exe" sync --scheduled`} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args missing %q: %v", want, args)
		}
	}

	// Sub-minute intervals must clamp to Task Scheduler's floor of 1 minute.
	args = scheduler.SchtasksCreateArgs(scheduler.Config{Executable: "x", Interval: time.Second})
	if !strings.Contains(strings.Join(args, " "), "/MO 1") {
		t.Fatalf("sub-minute interval not clamped: %v", args)
	}
}

func TestLaunchdPlist(t *testing.T) {
	plist := scheduler.LaunchdPlist(cfg(90 * time.Second))
	for _, want := range []string{
		"<string>" + scheduler.LaunchdLabel + "</string>",
		"<string>/usr/local/bin/agentpack</string>",
		"<string>--scheduled</string>",
		"<integer>90</integer>",
	} {
		if !strings.Contains(plist, want) {
			t.Fatalf("plist missing %q:\n%s", want, plist)
		}
	}
}

func TestSystemdUnits(t *testing.T) {
	service := scheduler.SystemdService(cfg(5 * time.Minute))
	if !strings.Contains(service, "ExecStart=/usr/local/bin/agentpack sync --scheduled") {
		t.Fatalf("service unit wrong:\n%s", service)
	}
	timer := scheduler.SystemdTimer(cfg(5 * time.Minute))
	for _, want := range []string{"OnUnitActiveSec=300s", "Persistent=true", "WantedBy=timers.target"} {
		if !strings.Contains(timer, want) {
			t.Fatalf("timer unit missing %q:\n%s", want, timer)
		}
	}
}

func TestValidate(t *testing.T) {
	if err := cfg(time.Minute).Validate(); err != nil {
		t.Fatal(err)
	}
	if err := cfg(30 * time.Second).Validate(); err == nil {
		t.Fatal("expected sub-minute interval to fail validation")
	}
	if err := (scheduler.Config{Interval: time.Minute}).Validate(); err == nil {
		t.Fatal("expected empty executable to fail validation")
	}
}
