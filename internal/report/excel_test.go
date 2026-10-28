package report_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Segners/PachinGo/internal/executor"
	"github.com/Segners/PachinGo/internal/inventory"
	"github.com/Segners/PachinGo/internal/patch"
	"github.com/Segners/PachinGo/internal/report"
)

func TestExport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.xlsx")
	inv := []inventory.HostInventory{
		{
			Name: "web-01", Kind: "vm", Host: "10.0.0.1", OS: "Ubuntu",
			Techs: []inventory.Tech{{Name: "React", Version: "18.2.0", Source: "package.json", Path: "/opt/a/package.json"}},
		},
	}
	checks := []executor.HostResult{
		{
			Name: "web-01", Kind: "vm", Host: "10.0.0.1",
			Updates: []patch.Update{{Package: "curl", CurrentVer: "1", AvailableVer: "2"}},
		},
	}
	if err := report.Export(path, inv, checks); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Size() < 100 {
		t.Fatalf("file too small: %d", fi.Size())
	}
}
