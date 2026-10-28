package inventory

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"github.com/Segners/PachinGo/internal/config"
	"github.com/Segners/PachinGo/internal/patch"
	"github.com/Segners/PachinGo/internal/sshx"
)

// Tech is a detected technology and version.
type Tech struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Source  string `json:"source"` // global | package.json | requirements.txt | pyproject.toml
	Path    string `json:"path,omitempty"`
}

// HostInventory is the scan result for one host.
type HostInventory struct {
	Name  string `json:"name"`
	Kind  string `json:"kind"`
	Host  string `json:"host"`
	OS    string `json:"os,omitempty"`
	Techs []Tech `json:"techs"`
	Error string `json:"error,omitempty"`
}

// ScanHost connects and detects Angular, React, Flask, Django, Blazor/.NET.
func ScanHost(ctx context.Context, cfg *config.Config, t config.Target) HostInventory {
	inv := HostInventory{
		Name:  t.Name,
		Kind:  string(t.Kind),
		Host:  t.Host,
		Techs: []Tech{},
	}
	client, err := sshx.Dial(ctx, cfg, t)
	if err != nil {
		inv.Error = err.Error()
		return inv
	}
	defer func() { _ = client.Close() }()

	if mgr, err := patch.New(ctx, client); err == nil {
		inv.OS = mgr.OSPrettyName(ctx)
	}

	techs := scanGlobal(ctx, client)
	techs = append(techs, scanProjects(ctx, client, cfg.ScanRoots)...)
	inv.Techs = dedupeTechs(techs)
	return inv
}

// ScanAll inventorie les technos des cibles demandées.
// Une liste de noms vide couvre toutes les cibles activées.
func ScanAll(ctx context.Context, cfg *config.Config, names []string) ([]HostInventory, error) {
	targets, err := cfg.SelectTargets(names)
	if err != nil {
		return nil, err
	}
	out := make([]HostInventory, len(targets))
	limit := cfg.Concurrency
	if limit <= 0 {
		limit = 1
	}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i, t := range targets {
		wg.Add(1)
		go func(i int, t config.Target) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			out[i] = ScanHost(ctx, cfg, t)
		}(i, t)
	}
	wg.Wait()
	return out, nil
}

func scanGlobal(ctx context.Context, client *sshx.Client) []Tech {
	var techs []Tech

	// Angular CLI
	if out, err := client.Run(ctx, `ng version 2>/dev/null | head -40`); err == nil {
		if v := matchFirst(out, `(?i)Angular CLI:\s*([0-9][^\s]*)`); v != "" {
			techs = append(techs, Tech{Name: "Angular", Version: v, Source: "global", Path: "ng"})
		} else if v := matchFirst(out, `(?i)@angular/core\s+([0-9][^\s]*)`); v != "" {
			techs = append(techs, Tech{Name: "Angular", Version: v, Source: "global", Path: "ng"})
		}
	}

	// React via npm root -g / npm ls -g
	if out, err := client.Run(ctx, `npm ls -g react --depth=0 2>/dev/null`); err == nil {
		if v := matchFirst(out, `react@([0-9][^\s]*)`); v != "" {
			techs = append(techs, Tech{Name: "React", Version: v, Source: "global", Path: "npm -g"})
		}
	}

	// Flask
	if out, err := client.Run(ctx, `python3 -c "import flask; print(flask.__version__)" 2>/dev/null || python -c "import flask; print(getattr(flask,'__version__','unknown'))" 2>/dev/null`); err == nil {
		v := strings.TrimSpace(out)
		if v != "" && !strings.Contains(v, "Error") {
			techs = append(techs, Tech{Name: "Flask", Version: firstLine(v), Source: "global", Path: "python"})
		}
	}
	if !hasTech(techs, "Flask") {
		if out, err := client.Run(ctx, `pip3 show flask 2>/dev/null || pip show flask 2>/dev/null`); err == nil {
			if v := matchFirst(out, `(?i)^Version:\s*(\S+)`); v != "" {
				techs = append(techs, Tech{Name: "Flask", Version: v, Source: "global", Path: "pip"})
			}
		}
	}

	// Django
	if out, err := client.Run(ctx, `django-admin --version 2>/dev/null`); err == nil {
		v := strings.TrimSpace(out)
		if v != "" && !strings.Contains(strings.ToLower(v), "not found") {
			techs = append(techs, Tech{Name: "Django", Version: firstLine(v), Source: "global", Path: "django-admin"})
		}
	}
	if !hasTech(techs, "Django") {
		if out, err := client.Run(ctx, `pip3 show django 2>/dev/null || pip show django 2>/dev/null`); err == nil {
			if v := matchFirst(out, `(?i)^Version:\s*(\S+)`); v != "" {
				techs = append(techs, Tech{Name: "Django", Version: v, Source: "global", Path: "pip"})
			}
		}
	}

	// .NET SDK / runtime (Blazor host)
	if out, err := client.Run(ctx, `dotnet --version 2>/dev/null`); err == nil {
		v := firstLine(strings.TrimSpace(strings.ReplaceAll(out, "\r", "")))
		if v != "" && regexp.MustCompile(`^[0-9]`).MatchString(v) {
			techs = append(techs, Tech{Name: ".NET", Version: v, Source: "global", Path: "dotnet"})
		}
	}
	if out, err := client.Run(ctx, `dotnet --list-runtimes 2>/dev/null | head -20`); err == nil {
		out = strings.ReplaceAll(out, "\r", "")
		if v := matchFirst(out, `(?i)Microsoft\.AspNetCore\.App\s+([0-9][^\s]*)`); v != "" && !hasTech(techs, "ASP.NET Core") {
			techs = append(techs, Tech{Name: "ASP.NET Core", Version: v, Source: "global", Path: "dotnet --list-runtimes"})
		}
	}

	return techs
}

func scanProjects(ctx context.Context, client *sshx.Client, roots []string) []Tech {
	if len(roots) == 0 {
		roots = []string{"/var/www", "/opt", "/srv", "/app", "$HOME"}
	}
	script := `
export PATH="/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin:$PATH"
set --` + " " + strings.Join(quoteAll(roots), " ") + `
for r in "$@"; do
  r=$(eval echo "$r")
  [ -d "$r" ] || continue
  find "$r" -maxdepth 5 \( \
    -name package.json -o \
    -name requirements.txt -o \
    -name pyproject.toml -o \
    -name '*.csproj' -o \
    -name '*.fsproj' -o \
    -name 'blazor.webassembly.js' -o \
    -name 'blazor.server.js' -o \
    -name '*.deps.json' \
  \) 2>/dev/null
done | head -120
`
	out, err := client.Shell(ctx, script)
	if err != nil || strings.TrimSpace(out) == "" {
		return nil
	}

	var techs []Tech
	for _, path := range strings.Split(out, "\n") {
		path = strings.TrimSpace(strings.ReplaceAll(path, "\r", ""))
		if path == "" {
			continue
		}
		switch {
		case strings.HasSuffix(path, "package.json"):
			content, err := client.Run(ctx, fmt.Sprintf(`cat %s 2>/dev/null`, shellQuote(path)))
			if err != nil {
				continue
			}
			techs = append(techs, ParsePackageJSON(content, path)...)
		case strings.HasSuffix(path, "requirements.txt"):
			content, err := client.Run(ctx, fmt.Sprintf(`cat %s 2>/dev/null`, shellQuote(path)))
			if err != nil {
				continue
			}
			techs = append(techs, ParseRequirements(content, path)...)
		case strings.HasSuffix(path, "pyproject.toml"):
			content, err := client.Run(ctx, fmt.Sprintf(`cat %s 2>/dev/null`, shellQuote(path)))
			if err != nil {
				continue
			}
			techs = append(techs, ParsePyProject(content, path)...)
		case strings.HasSuffix(strings.ToLower(path), ".csproj"), strings.HasSuffix(strings.ToLower(path), ".fsproj"):
			content, err := client.Run(ctx, fmt.Sprintf(`cat %s 2>/dev/null`, shellQuote(path)))
			if err != nil {
				continue
			}
			techs = append(techs, ParseDotNetProject(content, path)...)
		case strings.HasSuffix(path, "blazor.webassembly.js"):
			techs = append(techs, Tech{Name: "Blazor", Version: "WebAssembly", Source: "deployed", Path: path})
		case strings.HasSuffix(path, "blazor.server.js"):
			techs = append(techs, Tech{Name: "Blazor", Version: "Server", Source: "deployed", Path: path})
		case strings.HasSuffix(path, ".deps.json"):
			content, err := client.Run(ctx, fmt.Sprintf(`cat %s 2>/dev/null`, shellQuote(path)))
			if err != nil {
				continue
			}
			techs = append(techs, ParseDepsJSON(content, path)...)
		}
	}
	return techs
}

func quoteAll(roots []string) []string {
	out := make([]string, len(roots))
	for i, r := range roots {
		out[i] = shellQuote(r)
	}
	return out
}

func shellQuote(s string) string {
	return `'` + strings.ReplaceAll(s, `'`, `'\''`) + `'`
}

type packageJSON struct {
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// ParsePackageJSON extracts Angular/React versions from package.json content.
func ParsePackageJSON(content, path string) []Tech {
	var pkg packageJSON
	if err := json.Unmarshal([]byte(content), &pkg); err != nil {
		return nil
	}
	var techs []Tech
	lookup := func(name, dep string) {
		if v, ok := pkg.Dependencies[dep]; ok {
			techs = append(techs, Tech{Name: name, Version: cleanSemver(v), Source: "package.json", Path: path})
			return
		}
		if v, ok := pkg.DevDependencies[dep]; ok {
			techs = append(techs, Tech{Name: name, Version: cleanSemver(v), Source: "package.json", Path: path})
		}
	}
	lookup("Angular", "@angular/core")
	lookup("React", "react")
	return techs
}

var reqLine = regexp.MustCompile(`(?i)^(flask|django)\s*([=<>!~]+)\s*([^\s;#]+)`)

// ParseRequirements extracts Flask/Django from requirements.txt.
func ParseRequirements(content, path string) []Tech {
	var techs []Tech
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m := reqLine.FindStringSubmatch(line)
		if m != nil {
			name := "Flask"
			if strings.EqualFold(m[1], "django") {
				name = "Django"
			}
			techs = append(techs, Tech{Name: name, Version: cleanSemver(m[3]), Source: "requirements.txt", Path: path})
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "flask") && !containsWord(techs, "Flask") {
			techs = append(techs, Tech{Name: "Flask", Version: extractBareVersion(line), Source: "requirements.txt", Path: path})
		}
		if strings.HasPrefix(lower, "django") && !containsWord(techs, "Django") {
			techs = append(techs, Tech{Name: "Django", Version: extractBareVersion(line), Source: "requirements.txt", Path: path})
		}
	}
	return techs
}

// ParsePyProject extracts Flask/Django from pyproject.toml (simple regex).
func ParsePyProject(content, path string) []Tech {
	var techs []Tech
	patterns := []struct {
		name string
		re   *regexp.Regexp
	}{
		{"Flask", regexp.MustCompile(`(?i)["']flask(?:\[.*?\])?([=<>!~]+)([^"'\s,]+)["']`)},
		{"Django", regexp.MustCompile(`(?i)["']django(?:\[.*?\])?([=<>!~]+)([^"'\s,]+)["']`)},
		{"Flask", regexp.MustCompile(`(?i)^\s*flask\s*=\s*["']?([^"'\s]+)`)},
		{"Django", regexp.MustCompile(`(?i)^\s*django\s*=\s*["']?([^"'\s]+)`)},
	}
	for _, p := range patterns {
		for _, m := range p.re.FindAllStringSubmatch(content, -1) {
			ver := ""
			if len(m) >= 3 {
				ver = m[2]
			} else if len(m) == 2 {
				ver = m[1]
			}
			if ver != "" && !containsWord(techs, p.name) {
				techs = append(techs, Tech{Name: p.name, Version: cleanSemver(ver), Source: "pyproject.toml", Path: path})
			}
		}
	}
	return techs
}

// ParseDotNetProject detects Blazor / ASP.NET Core from a .csproj/.fsproj.
func ParseDotNetProject(content, path string) []Tech {
	content = strings.ReplaceAll(content, "\r", "")
	var techs []Tech

	tfm := matchFirst(content, `(?i)<TargetFramework>\s*([^<]+)\s*</TargetFramework>`)
	if tfm == "" {
		tfm = matchFirst(content, `(?i)<TargetFrameworks>\s*([^<]+)\s*</TargetFrameworks>`)
		if tfm != "" {
			tfm = strings.Split(tfm, ";")[0]
		}
	}
	tfm = strings.TrimSpace(tfm)

	isBlazor := strings.Contains(content, "Microsoft.NET.Sdk.BlazorWebAssembly") ||
		regexp.MustCompile(`(?i)Microsoft\.AspNetCore\.Components(\.Web(Assembly)?)?`).MatchString(content) ||
		regexp.MustCompile(`(?i)<PackageReference[^>]*Include\s*=\s*["']Microsoft\.AspNetCore\.Components`).MatchString(content)

	isAspNet := isBlazor ||
		strings.Contains(content, "Microsoft.NET.Sdk.Web") ||
		regexp.MustCompile(`(?i)Microsoft\.AspNetCore\.`).MatchString(content)

	if isBlazor {
		ver := blazorVersion(content)
		if ver == "" {
			ver = tfm
		}
		if ver == "" {
			ver = "present"
		}
		techs = append(techs, Tech{Name: "Blazor", Version: cleanSemver(ver), Source: "csproj", Path: path})
	}
	if isAspNet && !isBlazor {
		ver := tfm
		if ver == "" {
			ver = "present"
		}
		techs = append(techs, Tech{Name: "ASP.NET Core", Version: cleanSemver(ver), Source: "csproj", Path: path})
	}
	if tfm != "" && (isBlazor || isAspNet) {
		techs = append(techs, Tech{Name: ".NET", Version: cleanSemver(tfm), Source: "csproj", Path: path})
	}
	return techs
}

func blazorVersion(content string) string {
	patterns := []string{
		`(?i)<PackageReference[^>]*Include\s*=\s*["']Microsoft\.AspNetCore\.Components\.WebAssembly["'][^>]*Version\s*=\s*["']([^"']+)["']`,
		`(?i)<PackageReference[^>]*Include\s*=\s*["']Microsoft\.AspNetCore\.Components\.Web["'][^>]*Version\s*=\s*["']([^"']+)["']`,
		`(?i)<PackageReference[^>]*Include\s*=\s*["']Microsoft\.AspNetCore\.Components\.WebAssembly["'][^>]*>\s*<Version>\s*([^<]+)\s*</Version>`,
		`(?i)<PackageReference[^>]*Include\s*=\s*["']Microsoft\.AspNetCore\.Components\.Web["'][^>]*>\s*<Version>\s*([^<]+)\s*</Version>`,
	}
	for _, p := range patterns {
		if v := matchFirst(content, p); v != "" {
			return v
		}
	}
	return ""
}

// ParseDepsJSON detects Blazor from a published *.deps.json.
func ParseDepsJSON(content, path string) []Tech {
	lower := strings.ToLower(content)
	if !strings.Contains(lower, "microsoft.aspnetcore.components") {
		return nil
	}
	ver := matchFirst(content, `(?i)"Microsoft\.AspNetCore\.Components\.Web(?:Assembly)?/([0-9][^"]*)"`)
	if ver == "" {
		ver = matchFirst(content, `(?i)"Microsoft\.AspNetCore\.App/([0-9][^"]*)"`)
	}
	if ver == "" {
		ver = "present"
	}
	return []Tech{{
		Name:    "Blazor",
		Version: cleanSemver(ver),
		Source:  "deps.json",
		Path:    path,
	}}
}

func cleanSemver(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "^")
	v = strings.TrimPrefix(v, "~")
	v = strings.TrimPrefix(v, ">=")
	v = strings.TrimPrefix(v, "<=")
	v = strings.TrimPrefix(v, "==")
	v = strings.TrimPrefix(v, "~=")
	v = strings.TrimPrefix(v, ">")
	v = strings.TrimPrefix(v, "<")
	return strings.TrimSpace(v)
}

func extractBareVersion(line string) string {
	re := regexp.MustCompile(`[=<>!~]+\s*([0-9][^\s;#]*)`)
	if m := re.FindStringSubmatch(line); m != nil {
		return cleanSemver(m[1])
	}
	return "present"
}

func matchFirst(s, pattern string) string {
	re := regexp.MustCompile(pattern)
	m := re.FindStringSubmatch(s)
	if len(m) >= 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

func hasTech(techs []Tech, name string) bool {
	return containsWord(techs, name)
}

func containsWord(techs []Tech, name string) bool {
	for _, t := range techs {
		if t.Name == name {
			return true
		}
	}
	return false
}

func dedupeTechs(in []Tech) []Tech {
	type key struct{ name, path, source string }
	seen := map[key]struct{}{}
	out := make([]Tech, 0, len(in))
	for _, t := range in {
		k := key{t.Name, t.Path, t.Source}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, t)
	}
	return out
}
