package app

import (
	"strings"
	"testing"
)

func TestArchivedEmailFormat(t *testing.T) {
	uid := "11111111-2222-3333-4444-555555555555"
	got := archivedEmailFor(uid)
	want := "archived+11111111-2222-3333-4444-555555555555@archived.morfoschools.local"
	if got != want {
		t.Errorf("archivedEmailFor(%q) = %q, want %q", uid, got, want)
	}
	if !strings.HasPrefix(got, "archived+") {
		t.Errorf("synthetic email missing 'archived+' prefix: %q", got)
	}
}

func TestArchivedEmailUniquePerUser(t *testing.T) {
	a := archivedEmailFor("uid-aaa")
	b := archivedEmailFor("uid-bbb")
	if a == b {
		t.Errorf("expected different synthetic emails per user, got %q for both", a)
	}
}

func TestIsArchivedEmail(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"archived+abc@archived.morfoschools.local", true},
		{"archived+11111111-2222-3333-4444-555555555555@archived.morfoschools.local", true},
		{"user@example.com", false},
		{"archived+x@example.com", false},
		{"foo@archived.morfoschools.local", false},
		{"", false},
	}
	for _, c := range cases {
		if got := isArchivedEmail(c.in); got != c.want {
			t.Errorf("isArchivedEmail(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}
