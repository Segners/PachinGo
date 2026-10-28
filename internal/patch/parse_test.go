package patch_test

import (
	"testing"

	"github.com/Segners/PachinGo/internal/patch"
)

func TestParseAptUpgradable(t *testing.T) {
	out := `Listing...
curl/jammy-updates 7.81.0-1ubuntu1.16 amd64 [upgradable from: 7.81.0-1ubuntu1.15]
openssl/jammy-updates 3.0.2-0ubuntu1.18 amd64 [upgradable from: 3.0.2-0ubuntu1.12]
`
	updates := patch.ParseAptUpgradable(out)
	if len(updates) != 2 {
		t.Fatalf("len=%d", len(updates))
	}
	if updates[0].Package != "curl" || updates[0].CurrentVer != "7.81.0-1ubuntu1.15" {
		t.Fatalf("%+v", updates[0])
	}
	if updates[1].AvailableVer != "3.0.2-0ubuntu1.18" {
		t.Fatalf("%+v", updates[1])
	}
}

func TestParseDnfCheckUpdate(t *testing.T) {
	out := `Last metadata expiration check: 1:00:00 ago
bash.x86_64                  5.2.15-1.fc38               updates
vim-enhanced.x86_64          2:9.0.1500-1.fc38           updates
`
	updates := patch.ParseDnfCheckUpdate(out)
	if len(updates) != 2 {
		t.Fatalf("len=%d %+v", len(updates), updates)
	}
	if updates[0].Package != "bash" {
		t.Fatalf("%+v", updates[0])
	}
}

func TestParseApkVersion(t *testing.T) {
	out := `busybox-1.36.1-r0 < busybox-1.36.1-r15
musl-1.2.4-r2 < musl-1.2.5-r0
`
	updates := patch.ParseApkVersion(out)
	if len(updates) != 2 {
		t.Fatalf("len=%d %+v", len(updates), updates)
	}
	if updates[0].Package != "busybox" || updates[0].CurrentVer != "1.36.1-r0" || updates[0].AvailableVer != "1.36.1-r15" {
		t.Fatalf("%+v", updates[0])
	}
}
