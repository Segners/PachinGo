package patch

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/Segners/PachinGo/internal/crypto"
	"github.com/Segners/PachinGo/internal/sshx"
)

// Update describes a pending package upgrade.
type Update struct {
	Package      string `json:"package"`
	CurrentVer   string `json:"current_version"`
	AvailableVer string `json:"available_version"`
	Severity     string `json:"severity"`
}

type privMode int

const (
	privRoot privMode = iota
	privSudoN
	privSudoS
)

// Manager runs package update checks/applies over SSH.
type Manager struct {
	client *sshx.Client
	pm     string // apt | dnf | yum | apk

	privOnce sync.Once
	mode     privMode
	sudoPass string
	privErr  error
}

// New creates a Manager and detects the remote package manager.
func New(ctx context.Context, client *sshx.Client) (*Manager, error) {
	m := &Manager{client: client}
	pm, err := m.detect(ctx)
	if err != nil {
		return nil, err
	}
	m.pm = pm
	return m, nil
}

func (m *Manager) detect(ctx context.Context) (string, error) {
	if override := strings.ToLower(strings.TrimSpace(m.client.Target().PackageManager)); override != "" {
		switch override {
		case "apt", "dnf", "yum", "apk":
			return override, nil
		default:
			return "", fmt.Errorf("invalid package_manager %q on %s (use apt, dnf, yum or apk)", override, m.client.Target().Name)
		}
	}

	script := `
export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH"
echo PACHINGO_PROBE:ok
pm=""
for c in /usr/bin/apt-get /bin/apt-get /usr/bin/dnf /bin/dnf /usr/bin/yum /bin/yum /sbin/apk /usr/bin/apk; do
  if [ -x "$c" ]; then
    case "$c" in
      *apt-get) pm=apt ;;
      *dnf) pm=dnf ;;
      *yum) pm=yum ;;
      *apk) pm=apk ;;
    esac
    break
  fi
done
if [ -z "$pm" ]; then
  command -v apt-get >/dev/null 2>&1 && pm=apt
  command -v dnf >/dev/null 2>&1 && pm=dnf
  command -v yum >/dev/null 2>&1 && pm=yum
  command -v apk >/dev/null 2>&1 && pm=apk
fi
echo PACHINGO_PM:${pm:-unknown}
echo PACHINGO_UNAME:$(uname -s 2>/dev/null || echo unknown)
echo PACHINGO_ID:$(grep -E '^ID=' /etc/os-release 2>/dev/null | head -1 || true)
ls -l /usr/bin/apt-get /usr/bin/dnf /usr/bin/yum /sbin/apk 2>/dev/null | head -5 || true
`

	out, err := m.client.Shell(ctx, script)
	out = strings.ReplaceAll(out, "\r", "")
	if err != nil {
		return "", fmt.Errorf("detect package manager on %s: %w\n%s", m.client.Target().Name, err, truncate(out, 1200))
	}
	if strings.TrimSpace(out) == "" {
		// Last-resort tiny probe to distinguish SSH I/O vs script failure.
		probe, probeErr := m.client.Run(ctx, "echo PACHINGO_ECHO_OK")
		return "", fmt.Errorf(
			"empty SSH output while detecting package manager on %s (shell script produced no stdout). echo probe: %q err=%v — set package_manager: apt (or dnf/yum/apk) on the target as workaround",
			m.client.Target().Name, strings.TrimSpace(probe), probeErr,
		)
	}
	pm := ""
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "PACHINGO_PM:") {
			pm = strings.TrimPrefix(line, "PACHINGO_PM:")
			break
		}
	}
	switch pm {
	case "apt", "dnf", "yum", "apk":
		return pm, nil
	default:
		return "", fmt.Errorf(
			"unsupported package manager on %s (detected %q; need apt, dnf, yum or apk). Set package_manager on the target to force it.\nremote output:\n%s",
			m.client.Target().Name, pm, truncate(strings.TrimSpace(out), 1200),
		)
	}
}

func (m *Manager) resolvePriv(ctx context.Context) error {
	m.privOnce.Do(func() {
		out, err := m.client.Run(ctx, `id -u`)
		if err != nil {
			m.privErr = fmt.Errorf("detect uid: %w", err)
			return
		}
		if strings.TrimSpace(out) == "0" {
			m.mode = privRoot
			return
		}

		check, err := m.client.Run(ctx, `sudo -n true >/dev/null 2>&1; echo EXIT:$?`)
		if err == nil && strings.Contains(check, "EXIT:0") {
			m.mode = privSudoN
			return
		}

		// Classic sudoer with password: use encrypted SSH/sudo password via sudo -S.
		t := m.client.Target()
		if t.PasswordEnc == "" {
			m.privErr = fmt.Errorf(
				"user %q is sudoer but needs a password — set the target password in the UI/config (password_enc), or configure NOPASSWD, or connect as root",
				t.User,
			)
			return
		}
		pass, err := crypto.Decrypt(t.PasswordEnc)
		if err != nil {
			m.privErr = fmt.Errorf("decrypt sudo password: %w", err)
			return
		}
		probe, err := m.client.RunWithStdin(ctx, `sudo -S -p '' true >/dev/null 2>&1; echo EXIT:$?`, []byte(pass+"\n"))
		if err != nil || !strings.Contains(probe, "EXIT:0") {
			m.privErr = fmt.Errorf(
				"sudo password rejected for user %q — update the password in the target config (used for sudo -S)",
				t.User,
			)
			return
		}
		m.mode = privSudoS
		m.sudoPass = pass
	})
	return m.privErr
}

func (m *Manager) runPrivileged(ctx context.Context, script string) (string, error) {
	if err := m.resolvePriv(ctx); err != nil {
		return "", err
	}
	quoted := sshx.ShellQuote(script)
	switch m.mode {
	case privRoot:
		return m.client.Run(ctx, `/bin/sh -c `+quoted)
	case privSudoN:
		return m.client.Run(ctx, `sudo -n /bin/sh -c `+quoted)
	case privSudoS:
		return m.client.RunWithStdin(ctx, `sudo -S -p '' /bin/sh -c `+quoted, []byte(m.sudoPass+"\n"))
	default:
		return "", fmt.Errorf("unknown privilege mode")
	}
}

// PackageManager returns apt, dnf, yum or apk.
func (m *Manager) PackageManager() string {
	return m.pm
}

// OSPrettyName reads /etc/os-release PRETTY_NAME.
func (m *Manager) OSPrettyName(ctx context.Context) string {
	out, err := m.client.Run(ctx, `grep -E '^PRETTY_NAME=' /etc/os-release 2>/dev/null | head -1 | cut -d= -f2- | tr -d '"'`)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.ReplaceAll(out, "\r", ""))
}

// CheckUpdates lists pending package updates.
func (m *Manager) CheckUpdates(ctx context.Context) ([]Update, error) {
	switch m.pm {
	case "apt":
		_, _ = m.runPrivileged(ctx, `export DEBIAN_FRONTEND=noninteractive; apt-get update -qq`)
		out, err := m.client.Run(ctx, `apt list --upgradable 2>/dev/null`)
		if err != nil {
			return nil, err
		}
		return ParseAptUpgradable(out), nil
	case "dnf":
		out, err := m.runPrivileged(ctx, `dnf check-update -q || true`)
		return ParseDnfCheckUpdate(out), errIfFatal(err, out)
	case "yum":
		out, err := m.runPrivileged(ctx, `yum check-update -q || true`)
		return ParseDnfCheckUpdate(out), errIfFatal(err, out)
	case "apk":
		_, _ = m.runPrivileged(ctx, `apk update -q`)
		out, err := m.runPrivileged(ctx, `apk version -l '<' 2>/dev/null || true`)
		if err != nil {
			return nil, err
		}
		return ParseApkVersion(out), nil
	default:
		return nil, fmt.Errorf("unknown package manager: %s", m.pm)
	}
}

func errIfFatal(err error, out string) error {
	if err == nil {
		return nil
	}
	if strings.TrimSpace(out) != "" {
		return nil
	}
	return err
}

// ApplyUpdates runs a full upgrade.
func (m *Manager) ApplyUpdates(ctx context.Context) error {
	var script string
	switch m.pm {
	case "apt":
		script = `set -e; export DEBIAN_FRONTEND=noninteractive; ` +
			`apt-get update -y; ` +
			`apt-get -y -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold upgrade`
	case "dnf":
		script = `dnf upgrade -y`
	case "yum":
		script = `yum update -y`
	case "apk":
		script = `apk update && apk upgrade`
	default:
		return fmt.Errorf("unknown package manager: %s", m.pm)
	}

	out, err := m.runPrivileged(ctx, script)
	if err != nil {
		return fmt.Errorf("upgrade failed: %w\n%s%s", err, truncate(out, 4000), hintForApt(out, err))
	}
	return nil
}

func hintForApt(out string, err error) string {
	lower := strings.ToLower(out + " " + err.Error())
	switch {
	case strings.Contains(lower, "lock") || strings.Contains(lower, "could not get lock"):
		return "\nhint: package manager is locked — retry later"
	case strings.Contains(lower, "permission denied") || strings.Contains(lower, "are you root"):
		return "\nhint: need root or sudo — set the target password for sudo -S"
	case strings.Contains(lower, "status 100"):
		return "\nhint: package manager exit 100 — check privileges, repos, or lock on the host"
	default:
		return ""
	}
}

// IsRebootRequired reports whether a reboot is needed.
func (m *Manager) IsRebootRequired(ctx context.Context) (bool, error) {
	switch m.pm {
	case "apt":
		out, err := m.client.Run(ctx, `test -f /var/run/reboot-required && echo yes || echo no`)
		if err != nil {
			return false, err
		}
		return strings.TrimSpace(strings.ReplaceAll(out, "\r", "")) == "yes", nil
	case "dnf", "yum":
		out, err := m.runPrivileged(ctx, `needs-restarting -r >/dev/null 2>&1; echo $?`)
		if err != nil {
			return false, nil
		}
		return strings.TrimSpace(strings.ReplaceAll(out, "\r", "")) == "1", nil
	default:
		return false, nil
	}
}

var aptLine = regexp.MustCompile(`^([^\s/]+)/[^\s]+\s+(\S+)\s+\S+\s+\[upgradable from:\s*([^\]]+)\]`)

// ParseAptUpgradable parses `apt list --upgradable` output.
func ParseAptUpgradable(out string) []Update {
	var updates []Update
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Listing") {
			continue
		}
		m := aptLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		updates = append(updates, Update{
			Package:      m[1],
			AvailableVer: m[2],
			CurrentVer:   m[3],
			Severity:     "unknown",
		})
	}
	return updates
}

var dnfLine = regexp.MustCompile(`^(\S+)\s+(\S+)\s+(\S+)`)

// ParseDnfCheckUpdate parses `dnf check-update` / `yum check-update` output.
func ParseDnfCheckUpdate(out string) []Update {
	var updates []Update
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(strings.ReplaceAll(line, "\r", ""))
		if line == "" || strings.HasPrefix(line, "Last metadata") || strings.HasPrefix(line, "Security:") {
			continue
		}
		m := dnfLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		pkg := m[1]
		if i := strings.LastIndex(pkg, "."); i > 0 {
			pkg = pkg[:i]
		}
		updates = append(updates, Update{
			Package:      pkg,
			AvailableVer: m[2],
			CurrentVer:   "",
			Severity:     "unknown",
		})
	}
	return updates
}

// ParseApkVersion parses `apk version -l '<'` output (name-ver < name-ver).
func ParseApkVersion(out string) []Update {
	var updates []Update
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(strings.ReplaceAll(line, "\r", ""))
		if line == "" || !strings.Contains(line, "<") {
			continue
		}
		parts := strings.SplitN(line, "<", 2)
		if len(parts) != 2 {
			continue
		}
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		if left == "" || right == "" {
			continue
		}
		name := apkPkgName(left)
		updates = append(updates, Update{
			Package:      name,
			CurrentVer:   strings.TrimPrefix(left, name+"-"),
			AvailableVer: strings.TrimPrefix(right, name+"-"),
			Severity:     "unknown",
		})
	}
	return updates
}

func apkPkgName(pkgVer string) string {
	// Strip trailing -X.Y.Z-rN style version; keep name (may contain dashes).
	re := regexp.MustCompile(`^(.*)-([0-9].*)$`)
	if m := re.FindStringSubmatch(pkgVer); m != nil {
		return m[1]
	}
	return pkgVer
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
