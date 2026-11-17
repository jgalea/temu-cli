package browser

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestAccountConfigDir(t *testing.T) {
	base := t.TempDir()
	t.Setenv("TEMU_CONFIG_DIR", base)
	defer SetAccount("")

	if err := SetAccount(""); err != nil {
		t.Fatal(err)
	}
	if got := ConfigDir(); got != base {
		t.Fatalf("default dir = %q, want %q", got, base)
	}
	if err := SetAccount("work"); err != nil {
		t.Fatal(err)
	}
	if got, want := ConfigDir(), filepath.Join(base, "accounts", "work"); got != want {
		t.Fatalf("account dir = %q, want %q", got, want)
	}
}

func TestSetAccountRejectsPathTricks(t *testing.T) {
	defer SetAccount("")
	for _, bad := range []string{"../x", "a/b", "a b", ".", "..", "x\x00y"} {
		if err := SetAccount(bad); err == nil {
			t.Errorf("SetAccount(%q) accepted", bad)
		}
	}
}

func TestAccountsLists(t *testing.T) {
	base := t.TempDir()
	t.Setenv("TEMU_CONFIG_DIR", base)
	if got := Accounts(); len(got) != 0 {
		t.Fatalf("empty dir listed %v", got)
	}
	os.MkdirAll(filepath.Join(base, "profile"), 0o700)
	os.MkdirAll(filepath.Join(base, "accounts", "work"), 0o700)
	if got, want := Accounts(), []string{"default", "work"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Accounts() = %v, want %v", got, want)
	}
}
