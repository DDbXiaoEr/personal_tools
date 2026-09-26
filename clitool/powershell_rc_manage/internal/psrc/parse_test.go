package psrc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleAlias = `# ===== Docker =====
Set-Alias -Name dim -Value docker
Set-Alias dps Get-Process
New-Alias -Name ll -Value Get-ChildItem
`

const sampleEnv = `# Go
$env:GOROOT = "C:\Go"
$env:PATH = "$env:PATH;C:\Go\bin"

# tools
${env:FOO_BAR} = 'ok'
`

const sampleFunc = `# ===== net =====
function ip {
    if ($args.Count -eq 0) {
        curl.exe -s ipinfo.io
    } else {
        curl.exe -s "ipinfo.io/$($args[0])"
    }
}

function greet {
    param($name)
    Write-Host "hi $name"
}
`

func TestParseAlias(t *testing.T) {
	f := Parse(sampleAlias, KindAlias)
	if len(f.Items) != 3 {
		t.Fatalf("items = %d, want 3: %#v", len(f.Items), names(f))
	}
	if f.Items[0].Name != "dim" || f.Items[0].Value != "docker" || f.Items[0].Cat() != "Docker" {
		t.Fatalf("item0 = %+v", f.Items[0])
	}
	if f.Items[1].Name != "dps" || f.Items[1].Value != "Get-Process" {
		t.Fatalf("item1 = %+v", f.Items[1])
	}
	if f.Items[2].Name != "ll" || f.Items[2].Value != "Get-ChildItem" {
		t.Fatalf("item2 = %+v", f.Items[2])
	}
}

func TestParseEnv(t *testing.T) {
	f := Parse(sampleEnv, KindEnv)
	if len(f.Items) != 3 {
		t.Fatalf("items = %d, want 3: %#v", len(f.Items), names(f))
	}
	if f.Items[0].Name != "GOROOT" || f.Items[0].Value != `C:\Go` {
		t.Fatalf("item0 = %+v", f.Items[0])
	}
	if f.Items[1].Name != "PATH" || f.Items[1].Value != `$env:PATH;C:\Go\bin` {
		t.Fatalf("item1 = %+v", f.Items[1])
	}
	if f.Items[1].Quote != QuoteDouble {
		t.Fatalf("PATH quote = %q", f.Items[1].Quote)
	}
	if f.Items[2].Name != "FOO_BAR" || f.Items[2].Value != "ok" || f.Items[2].Cat() != "tools" {
		t.Fatalf("item2 = %+v", f.Items[2])
	}
}

func TestParseFunctions(t *testing.T) {
	f := Parse(sampleFunc, KindFunctions)
	if len(f.Items) != 2 {
		t.Fatalf("items = %d, want 2: %#v", len(f.Items), names(f))
	}
	if f.Items[0].Name != "ip" || f.Items[0].Style != StyleFunction {
		t.Fatalf("item0 = %+v", f.Items[0])
	}
	if !strings.Contains(f.Items[0].Value, "ipinfo.io") {
		t.Fatalf("ip body = %q", f.Items[0].Value)
	}
	if f.Items[1].Name != "greet" || !strings.Contains(f.Items[1].Value, "param($name)") {
		t.Fatalf("item1 = %+v", f.Items[1])
	}
	if f.Items[0].Cat() != "net" {
		t.Fatalf("cat = %q", f.Items[0].Cat())
	}
}

func TestParseOneLineFunction(t *testing.T) {
	f := Parse("function foo { Write-Host hi }\n", KindFunctions)
	if len(f.Items) != 1 || f.Items[0].Name != "foo" {
		t.Fatalf("items = %#v", names(f))
	}
	if !strings.Contains(f.Items[0].Value, "Write-Host hi") {
		t.Fatalf("body = %q", f.Items[0].Value)
	}
}

func TestParseHyphenFunction(t *testing.T) {
	f := Parse("function ssh-host {\n  ssh $args[0]\n}\n", KindFunctions)
	if len(f.Items) != 1 || f.Items[0].Name != "ssh-host" {
		t.Fatalf("items = %#v", names(f))
	}
}

func TestParseNamedSnippet(t *testing.T) {
	src := `# ===== tools =====
# name: nvm
$nvm = "$env:APPDATA\nvm"
if (Test-Path $nvm) { . $nvm }
`
	f := Parse(src, KindEnv)
	if len(f.Items) != 1 {
		t.Fatalf("items = %d", len(f.Items))
	}
	if f.Items[0].Name != "nvm" || f.Items[0].Style != StyleSnippet {
		t.Fatalf("item = %+v", f.Items[0])
	}
}

func TestParseSnippetWithoutName(t *testing.T) {
	src := `# NVM
if (Test-Path "$env:NVM_HOME\nvm.exe") {
    $env:PATH = "$env:PATH;$env:NVM_HOME"
}
`
	f := Parse(src, KindEnv)
	if len(f.Items) != 1 || f.Items[0].Style != StyleSnippet {
		t.Fatalf("items = %#v", names(f))
	}
	if f.Items[0].Cat() != "NVM" {
		t.Fatalf("cat = %q", f.Items[0].Cat())
	}
}

func TestAliasRoundTrip(t *testing.T) {
	f := Parse(sampleAlias, KindAlias)
	f2 := Parse(f.String(), KindAlias)
	if len(f2.Items) != len(f.Items) {
		t.Fatalf("roundtrip count %d -> %d\n%s", len(f.Items), len(f2.Items), f.String())
	}
	for i := range f.Items {
		if f2.Items[i].Name != f.Items[i].Name || f2.Items[i].Value != f.Items[i].Value {
			t.Fatalf("item %d: %+v vs %+v", i, f.Items[i], f2.Items[i])
		}
		if f2.Items[i].Cat() != f.Items[i].Cat() {
			t.Fatalf("cat %d: %q vs %q", i, f2.Items[i].Cat(), f.Items[i].Cat())
		}
	}
}

func TestEnvRoundTripKeepsExpansion(t *testing.T) {
	f := Parse(sampleEnv, KindEnv)
	out := f.String()
	if !strings.Contains(out, `$env:PATH = "$env:PATH;C:\Go\bin"`) {
		t.Fatalf("PATH lost expansion:\n%s", out)
	}
	f2 := Parse(out, KindEnv)
	if len(f2.Items) != len(f.Items) {
		t.Fatalf("roundtrip count %d -> %d\n%s", len(f.Items), len(f2.Items), out)
	}
	if f2.Items[1].Value != `$env:PATH;C:\Go\bin` {
		t.Fatalf("PATH = %q", f2.Items[1].Value)
	}
}

func TestFunctionRoundTrip(t *testing.T) {
	f := Parse(sampleFunc, KindFunctions)
	f2 := Parse(f.String(), KindFunctions)
	if len(f2.Items) != 2 {
		t.Fatalf("count = %d\n%s", len(f2.Items), f.String())
	}
	if f2.Items[0].Name != "ip" || !strings.Contains(f2.Items[0].Value, "ipinfo.io") {
		t.Fatalf("ip = %+v", f2.Items[0])
	}
	if f2.Items[1].Name != "greet" {
		t.Fatalf("greet = %+v", f2.Items[1])
	}
}

func TestParseCRLF(t *testing.T) {
	src := "Set-Alias -Name ll -Value Get-ChildItem\r\n$env:FOO = 'bar'\r\n"
	f := Parse(src, KindAlias)
	if len(f.Items) != 2 {
		t.Fatalf("items = %#v", names(f))
	}
}

func TestParseMissingFile(t *testing.T) {
	f, err := ParseFile(filepath.Join(t.TempDir(), "nope"), KindAlias)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Items) != 0 {
		t.Fatalf("want empty, got %d", len(f.Items))
	}
}

func TestSaveBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".ps_alias.ps1")
	if err := os.WriteFile(path, []byte("Set-Alias -Name a -Value 1\r\n"), 0644); err != nil {
		t.Fatal(err)
	}
	f := Parse("Set-Alias -Name b -Value 2\n", KindAlias)
	f.Path = path
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	bak, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(bak), "Set-Alias -Name a") {
		t.Fatalf("bak = %q", bak)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "Set-Alias -Name b") {
		t.Fatalf("saved = %q", got)
	}
	if !strings.Contains(string(got), "\r\n") {
		t.Fatalf("want CRLF:\n%q", got)
	}
}

func TestLoadStore(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".ps_alias.ps1"), []byte(sampleAlias), 0644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if s.File(KindAlias).Items[0].Name != "dim" {
		t.Fatalf("alias not loaded")
	}
	if s.File(KindEnv).Path != filepath.Join(dir, ".ps_env.ps1") {
		t.Fatalf("env path = %s", s.File(KindEnv).Path)
	}
}

func TestSingleQuoteEscape(t *testing.T) {
	f := Parse("Set-Alias -Name x -Value 'it''s ok'\n", KindAlias)
	if len(f.Items) != 1 || f.Items[0].Value != "it's ok" {
		t.Fatalf("got %+v", f.Items)
	}
	out := f.String()
	f2 := Parse(out, KindAlias)
	if f2.Items[0].Value != "it's ok" {
		t.Fatalf("roundtrip = %q\n%s", f2.Items[0].Value, out)
	}
}

func TestFunctionHereString(t *testing.T) {
	src := "function say {\n  $t = @\"\nhello {world}\n\"@\n  $t\n}\n"
	f := Parse(src, KindFunctions)
	if len(f.Items) != 1 || f.Items[0].Name != "say" {
		t.Fatalf("items = %#v", names(f))
	}
	if !strings.Contains(f.Items[0].Value, "hello {world}") {
		t.Fatalf("body = %q", f.Items[0].Value)
	}
}

func names(f *File) []string {
	var out []string
	for _, it := range f.Items {
		out = append(out, string(it.Style)+"/"+it.Name)
	}
	return out
}
