package shellrc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleAlias = `# ===== Python 虚拟环境 =====
alias adkdev='source ~/pyvenv/agentdev/bin/activate'
alias langchaindev='source ~/pyvenv/langchaingraphdev/bin/activate'

# ===== Docker =====
alias dim='docker images'
alias dps="docker ps"
alias ll=ls
`

const sampleEnv = `# Go
GO_BIN_PATH="/usr/local/go/bin"
export PATH=$PATH:/Users/quanxiaoer/go/bin

# NVM
export NVM_DIR="$HOME/.nvm"
[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"  # This loads nvm
[ -s "$NVM_DIR/bash_completion" ] && \. "$NVM_DIR/bash_completion"

# Terraform
export TF_CLI_CONFIG_FILE=$HOME/.terraform.d/.terraformrc
`

const sampleFunc = `# ===== net =====
ip() {
    if [ $# -eq 0 ]; then
        curl -s ipinfo.io
    else
        curl -s "ipinfo.io/$1"
    fi
    echo
}

function greet() {
    echo "hi $1"
}
`

func TestParseAliasCategories(t *testing.T) {
	f := Parse(sampleAlias, KindAlias)
	if len(f.Items) != 5 {
		t.Fatalf("items = %d, want 5", len(f.Items))
	}
	if f.Items[0].Cat() != "Python 虚拟环境" || f.Items[0].Name != "adkdev" {
		t.Fatalf("item0 = %+v", f.Items[0])
	}
	if f.Items[2].Cat() != "Docker" || f.Items[2].Name != "dim" {
		t.Fatalf("item2 = %+v", f.Items[2])
	}
	if f.Items[3].Quote != QuoteDouble || f.Items[3].Value != "docker ps" {
		t.Fatalf("dps quote/value = %q %q", f.Items[3].Quote, f.Items[3].Value)
	}
	if f.Items[4].Quote != QuoteNone || f.Items[4].Value != "ls" {
		t.Fatalf("ll = %+v", f.Items[4])
	}
}

func TestParseEnvMixed(t *testing.T) {
	f := Parse(sampleEnv, KindEnv)
	if len(f.Items) != 5 {
		t.Fatalf("items = %d, want 5: %#v", len(f.Items), names(f))
	}
	if f.Items[0].Style != StyleAssign || f.Items[0].Name != "GO_BIN_PATH" {
		t.Fatalf("item0 = %+v", f.Items[0])
	}
	if f.Items[0].Value != "/usr/local/go/bin" {
		t.Fatalf("GO_BIN_PATH value = %q", f.Items[0].Value)
	}
	if f.Items[1].Style != StyleExport || f.Items[1].Name != "PATH" {
		t.Fatalf("item1 = %+v", f.Items[1])
	}
	if !strings.HasPrefix(f.Items[1].Value, "$PATH:") {
		t.Fatalf("PATH value = %q", f.Items[1].Value)
	}
	if f.Items[2].Name != "NVM_DIR" || f.Items[2].Value != "$HOME/.nvm" {
		t.Fatalf("NVM_DIR = %+v", f.Items[2])
	}
	if f.Items[3].Style != StyleSnippet {
		t.Fatalf("want snippet, got %+v", f.Items[3])
	}
	if !strings.Contains(f.Items[3].Value, "nvm.sh") || !strings.Contains(f.Items[3].Value, "bash_completion") {
		t.Fatalf("snippet = %q", f.Items[3].Value)
	}
	if f.Items[3].Cat() != "NVM" {
		t.Fatalf("snippet cat = %q", f.Items[3].Cat())
	}
	if f.Items[4].Name != "TF_CLI_CONFIG_FILE" || f.Items[4].Cat() != "Terraform" {
		t.Fatalf("item4 = %+v", f.Items[4])
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
	if f.Items[1].Name != "greet" {
		t.Fatalf("item1 name = %q", f.Items[1].Name)
	}
	if f.Items[0].Cat() != "net" {
		t.Fatalf("cat = %q", f.Items[0].Cat())
	}
}

func TestParseOneLineFunction(t *testing.T) {
	f := Parse("foo() { echo hi; }\n", KindFunctions)
	if len(f.Items) != 1 || f.Items[0].Name != "foo" {
		t.Fatalf("items = %#v", names(f))
	}
	if !strings.Contains(f.Items[0].Value, "echo hi") {
		t.Fatalf("body = %q", f.Items[0].Value)
	}
}

func TestParseHyphenFunction(t *testing.T) {
	f := Parse("ssh-host() {\n  ssh \"$1\"\n}\n", KindFunctions)
	if len(f.Items) != 1 || f.Items[0].Name != "ssh-host" {
		t.Fatalf("items = %#v", names(f))
	}
}

func TestExportWithoutEqualsIsSnippet(t *testing.T) {
	f := Parse("export PATH\nexport FOO=bar\n", KindEnv)
	if len(f.Items) != 2 {
		t.Fatalf("items = %#v", names(f))
	}
	if f.Items[0].Style != StyleSnippet || !strings.Contains(f.Items[0].Value, "export PATH") {
		t.Fatalf("item0 = %+v", f.Items[0])
	}
	if f.Items[1].Name != "FOO" || f.Items[1].Value != "bar" {
		t.Fatalf("item1 = %+v", f.Items[1])
	}
}

func TestParseNamedSnippet(t *testing.T) {
	src := `# ===== tools =====
# name: nvm
[ -s "$NVM_DIR/nvm.sh" ] && . "$NVM_DIR/nvm.sh"
`
	f := Parse(src, KindEnv)
	if len(f.Items) != 1 {
		t.Fatalf("items = %d", len(f.Items))
	}
	if f.Items[0].Name != "nvm" || f.Items[0].Style != StyleSnippet {
		t.Fatalf("item = %+v", f.Items[0])
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
	if !strings.Contains(out, `export NVM_DIR="$HOME/.nvm"`) && !strings.Contains(out, `export NVM_DIR='$HOME/.nvm'`) {
		// double quotes should keep $HOME expandable
		if !strings.Contains(out, `$HOME/.nvm`) {
			t.Fatalf("NVM_DIR lost expansion:\n%s", out)
		}
	}
	if strings.Contains(out, `\$HOME`) {
		t.Fatalf("escaped $HOME would break expansion:\n%s", out)
	}
	f2 := Parse(out, KindEnv)
	if len(f2.Items) != len(f.Items) {
		t.Fatalf("roundtrip count %d -> %d\n%s", len(f.Items), len(f2.Items), out)
	}
	if f2.Items[2].Value != "$HOME/.nvm" {
		t.Fatalf("NVM_DIR = %q", f2.Items[2].Value)
	}
	if f2.Items[3].Style != StyleSnippet {
		t.Fatalf("snippet lost: %+v", f2.Items[3])
	}
}

func TestFunctionRoundTrip(t *testing.T) {
	f := Parse(sampleFunc, KindFunctions)
	f2 := Parse(f.String(), KindFunctions)
	if len(f2.Items) != 2 {
		t.Fatalf("count = %d\n%s", len(f2.Items), f.String())
	}
	if f2.Items[0].Name != "ip" || !strings.Contains(f2.Items[0].Value, `curl -s "ipinfo.io/$1"`) {
		t.Fatalf("ip = %+v", f2.Items[0])
	}
	if f2.Items[1].Name != "greet" {
		t.Fatalf("greet = %+v", f2.Items[1])
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
	path := filepath.Join(dir, ".zsh_alias")
	if err := os.WriteFile(path, []byte("alias a='1'\n"), 0644); err != nil {
		t.Fatal(err)
	}
	f := Parse("alias b='2'\n", KindAlias)
	f.Path = path
	if err := f.Save(); err != nil {
		t.Fatal(err)
	}
	bak, err := os.ReadFile(path + ".bak")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(bak), "alias a=") {
		t.Fatalf("bak = %q", bak)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "alias b=") {
		t.Fatalf("saved = %q", got)
	}
}

func TestLoadStore(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".zsh_alias"), []byte(sampleAlias), 0644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(dir, "zsh")
	if err != nil {
		t.Fatal(err)
	}
	if s.File(KindAlias).Items[0].Name != "adkdev" {
		t.Fatalf("alias not loaded")
	}
	if s.File(KindEnv).Path != filepath.Join(dir, ".zsh_env") {
		t.Fatalf("env path = %s", s.File(KindEnv).Path)
	}
}

func TestNormalizeShell(t *testing.T) {
	if got := NormalizeShell("/bin/zsh"); got != "zsh" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeShell("/usr/local/bin/bash"); got != "bash" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeShell(""); got != "zsh" {
		t.Fatalf("got %q", got)
	}
}

func TestSingleQuoteEscape(t *testing.T) {
	f := Parse(`alias x='it'\''s ok'`+"\n", KindAlias)
	if len(f.Items) != 1 || f.Items[0].Value != "it's ok" {
		t.Fatalf("got %+v", f.Items)
	}
	out := f.String()
	f2 := Parse(out, KindAlias)
	if f2.Items[0].Value != "it's ok" {
		t.Fatalf("roundtrip = %q\n%s", f2.Items[0].Value, out)
	}
}

func names(f *File) []string {
	var out []string
	for _, it := range f.Items {
		out = append(out, it.Style.String()+"/"+it.Name)
	}
	return out
}

func (s Style) String() string { return string(s) }
