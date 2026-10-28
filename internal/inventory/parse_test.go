package inventory_test

import (
	"testing"

	"github.com/Segners/PachinGo/internal/inventory"
)

func TestParsePackageJSON(t *testing.T) {
	content := `{
  "dependencies": {
    "@angular/core": "^17.3.0",
    "react": "~18.2.0"
  }
}`
	techs := inventory.ParsePackageJSON(content, "/opt/app/package.json")
	if len(techs) != 2 {
		t.Fatalf("len=%d %+v", len(techs), techs)
	}
	byName := map[string]string{}
	for _, tech := range techs {
		byName[tech.Name] = tech.Version
	}
	if byName["Angular"] != "17.3.0" || byName["React"] != "18.2.0" {
		t.Fatalf("%v", byName)
	}
}

func TestParseRequirements(t *testing.T) {
	content := `# deps
Flask==3.0.2
Django>=4.2.0
requests==2.31.0
`
	techs := inventory.ParseRequirements(content, "/opt/api/requirements.txt")
	if len(techs) != 2 {
		t.Fatalf("len=%d %+v", len(techs), techs)
	}
}

func TestParsePyProject(t *testing.T) {
	content := `
[project]
dependencies = [
  "flask>=3.0.0",
  "django==4.2.11",
]
`
	techs := inventory.ParsePyProject(content, "/opt/api/pyproject.toml")
	if len(techs) < 1 {
		t.Fatalf("expected techs, got %+v", techs)
	}
}

func TestParseDotNetProjectBlazor(t *testing.T) {
	content := `
<Project Sdk="Microsoft.NET.Sdk.BlazorWebAssembly">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Microsoft.AspNetCore.Components.Web" Version="8.0.4" />
  </ItemGroup>
</Project>
`
	techs := inventory.ParseDotNetProject(content, "/opt/blazor/App.csproj")
	byName := map[string]string{}
	for _, tech := range techs {
		byName[tech.Name] = tech.Version
	}
	if byName["Blazor"] != "8.0.4" {
		t.Fatalf("Blazor=%q techs=%+v", byName["Blazor"], techs)
	}
	if byName[".NET"] != "net8.0" {
		t.Fatalf(".NET=%q", byName[".NET"])
	}
}

func TestParseDepsJSON(t *testing.T) {
	content := `{
  "libraries": {
    "Microsoft.AspNetCore.Components.Web/8.0.2": { "type": "package" }
  }
}`
	techs := inventory.ParseDepsJSON(content, "/var/www/site/App.deps.json")
	if len(techs) != 1 || techs[0].Name != "Blazor" || techs[0].Version != "8.0.2" {
		t.Fatalf("%+v", techs)
	}
}
