package search

import "testing"

type item struct {
	id   int
	name string
}

func itemName(i item) string { return i.name }

func TestModeMatch(t *testing.T) {
	cases := []struct {
		mode              Mode
		candidate, target string
		want              bool
	}{
		{Exact, "cs2.exe", "cs2.exe", true},
		{Exact, "CS2.exe", "cs2.exe", false},
		{Fold, "CLIENT.DLL", "client.dll", true},
		{Fold, "client.dll.bak", "client.dll", false},
		{Contains, "libClient.so", "client", true},
		{Contains, "server.so", "client", false},
	}
	for _, c := range cases {
		if got := c.mode.Match(c.candidate, c.target); got != c.want {
			t.Errorf("%v.Match(%q, %q) = %v, want %v", c.mode, c.candidate, c.target, got, c.want)
		}
	}
}

func TestParseMode(t *testing.T) {
	for in, want := range map[string]Mode{"exact": Exact, "FOLD": Fold, " i ": Fold, "contains": Contains} {
		got, err := ParseMode(in)
		if err != nil || got != want {
			t.Errorf("ParseMode(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	if _, err := ParseMode("regex"); err == nil {
		t.Error("ParseMode(regex) should fail")
	}
}

func TestAll(t *testing.T) {
	items := []item{{1, "a.dll"}, {2, "B.dll"}, {3, "a.DLL"}}
	if got := All(items, itemName, "a.dll", Fold); len(got) != 2 || got[0].id != 1 || got[1].id != 3 {
		t.Errorf("All fold = %v", got)
	}
	if got := All(items, itemName, "", Exact); len(got) != 3 {
		t.Errorf("All with empty target should return everything, got %v", got)
	}
}

func TestBest(t *testing.T) {
	items := []item{{1, "FOO.DLL"}, {2, "Foo.dll"}, {3, "foo.dll"}}

	if got, ok := Best(items, itemName, "Foo.dll", Fold); !ok || got.id != 2 {
		t.Errorf("exact-case hit should win, got %v %v", got, ok)
	}
	if got, ok := Best(items, itemName, "fOO.dll", Fold); !ok || got.id != 1 {
		t.Errorf("first fold hit should win, got %v %v", got, ok)
	}
	if _, ok := Best(items, itemName, "fOO.dll", Exact); ok {
		t.Error("exact mode must not fall back to fold")
	}
	if _, ok := Best(items, itemName, "", Fold); ok {
		t.Error("empty target must not match")
	}
}

// The original only called Process32Next, so the first snapshot entry could never match.
// Best must consider index 0.
func TestBestConsidersFirstEntry(t *testing.T) {
	items := []item{{0, "first.exe"}, {1, "second.exe"}}
	if got, ok := Best(items, itemName, "first.exe", Exact); !ok || got.id != 0 {
		t.Errorf("first entry skipped: %v %v", got, ok)
	}
}
