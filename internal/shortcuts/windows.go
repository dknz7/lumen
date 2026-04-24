// Package shortcuts creates Windows desktop shortcuts (.lnk files).
package shortcuts

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// CreateDesktop drops a Lumen.lnk on the user's Desktop, targeting the given
// executable with the given arguments. Uses PowerShell's WScript.Shell COM
// object — it's available on every Windows install since XP.
func CreateDesktop(exePath, args string) (string, error) {
	exePath, err := filepath.Abs(exePath)
	if err != nil {
		return "", fmt.Errorf("abs path: %w", err)
	}
	workDir := filepath.Dir(exePath)

	desktop := filepath.Join(os.Getenv("USERPROFILE"), "Desktop")
	if info, err := os.Stat(desktop); err != nil || !info.IsDir() {
		return "", fmt.Errorf("desktop not found at %s", desktop)
	}
	lnkPath := filepath.Join(desktop, "Lumen.lnk")

	script := fmt.Sprintf(`
$wsh = New-Object -ComObject WScript.Shell
$sc = $wsh.CreateShortcut(%q)
$sc.TargetPath = %q
$sc.Arguments = %q
$sc.WorkingDirectory = %q
$sc.IconLocation = %q
$sc.Description = "Launch Lumen"
$sc.Save()
`, lnkPath, exePath, args, workDir, exePath)

	cmd := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("powershell: %w — %s", err, out)
	}
	return lnkPath, nil
}
