//go:build windows

package scheduler

// Install registers (or updates) the per-user Task Scheduler task. No admin
// rights are required for a task in the user's own context.
func Install(cfg Config) (string, error) {
	if err := cfg.Validate(); err != nil {
		return "", err
	}
	if _, err := run("schtasks", SchtasksCreateArgs(cfg)...); err != nil {
		return "", err
	}
	return "Task Scheduler task \"" + TaskName + "\" installed (runs `agentpack sync --scheduled`).", nil
}

// Uninstall removes the task; a missing task is not an error.
func Uninstall() (string, error) {
	if out, err := run("schtasks", "/Delete", "/F", "/TN", TaskName); err != nil {
		if _, qErr := run("schtasks", "/Query", "/TN", TaskName); qErr != nil {
			return "Task \"" + TaskName + "\" is not installed.", nil
		}
		return out, err
	}
	return "Task \"" + TaskName + "\" removed.", nil
}

// Status reports the task as Task Scheduler sees it.
func Status() (string, error) {
	return run("schtasks", "/Query", "/TN", TaskName, "/FO", "LIST")
}
