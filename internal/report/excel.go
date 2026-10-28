package report

import (
	"fmt"
	"time"

	"github.com/Segners/PachinGo/internal/executor"
	"github.com/Segners/PachinGo/internal/inventory"
	"github.com/xuri/excelize/v2"
)

// Export writes Hosts, Technologies and Updates sheets to path.
func Export(path string, inv []inventory.HostInventory, checks []executor.HostResult) error {
	f := excelize.NewFile()
	defer f.Close()

	hostsSheet := "Hosts"
	techSheet := "Technologies"
	updSheet := "Updates"

	_ = f.SetSheetName("Sheet1", hostsSheet)
	_, _ = f.NewSheet(techSheet)
	_, _ = f.NewSheet(updSheet)

	writeHosts(f, hostsSheet, inv, checks)
	writeTechs(f, techSheet, inv)
	writeUpdates(f, updSheet, checks)

	return f.SaveAs(path)
}

func writeHosts(f *excelize.File, sheet string, inv []inventory.HostInventory, checks []executor.HostResult) {
	headers := []string{"name", "kind", "host", "os", "last_scan", "reboot_required", "error"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}

	checkByName := map[string]executor.HostResult{}
	for _, c := range checks {
		checkByName[c.Name] = c
	}

	now := time.Now().UTC().Format(time.RFC3339)
	// Prefer inventory list; fall back to checks-only hosts
	rows := make([]inventory.HostInventory, 0, len(inv)+len(checks))
	seen := map[string]struct{}{}
	for _, h := range inv {
		rows = append(rows, h)
		seen[h.Name] = struct{}{}
	}
	for _, c := range checks {
		if _, ok := seen[c.Name]; !ok {
			rows = append(rows, inventory.HostInventory{
				Name: c.Name, Kind: c.Kind, Host: c.Host, OS: c.OS, Error: c.Error,
			})
		}
	}

	for i, h := range rows {
		reboot := false
		if c, ok := checkByName[h.Name]; ok {
			reboot = c.RebootRequired
			if h.OS == "" {
				h.OS = c.OS
			}
			if h.Error == "" {
				h.Error = c.Error
			}
		}
		vals := []any{h.Name, h.Kind, h.Host, h.OS, now, reboot, h.Error}
		for j, v := range vals {
			cell, _ := excelize.CoordinatesToCellName(j+1, i+2)
			_ = f.SetCellValue(sheet, cell, v)
		}
	}
}

func writeTechs(f *excelize.File, sheet string, inv []inventory.HostInventory) {
	headers := []string{"host", "kind", "tech", "version", "source", "path"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}
	row := 2
	for _, h := range inv {
		for _, t := range h.Techs {
			vals := []any{h.Name, h.Kind, t.Name, t.Version, t.Source, t.Path}
			for j, v := range vals {
				cell, _ := excelize.CoordinatesToCellName(j+1, row)
				_ = f.SetCellValue(sheet, cell, v)
			}
			row++
		}
	}
}

func writeUpdates(f *excelize.File, sheet string, checks []executor.HostResult) {
	headers := []string{"host", "kind", "package", "current", "available", "severity"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(sheet, cell, h)
	}
	row := 2
	for _, h := range checks {
		for _, u := range h.Updates {
			vals := []any{h.Name, h.Kind, u.Package, u.CurrentVer, u.AvailableVer, u.Severity}
			for j, v := range vals {
				cell, _ := excelize.CoordinatesToCellName(j+1, row)
				_ = f.SetCellValue(sheet, cell, v)
			}
			row++
		}
	}
	if row == 2 {
		_ = f.SetCellValue(sheet, "A2", fmt.Sprintf("no updates recorded at %s", time.Now().UTC().Format(time.RFC3339)))
	}
}
