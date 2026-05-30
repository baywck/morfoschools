package app

import "testing"

func TestIsBlueprintDraftSaveRequest(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{"simpan dulu slot 6-10", true},
		{"buatkan proposal kisi-kisi ini", true},
		{"simpan pengaturan", false},
	}
	for _, c := range cases {
		if got := isBlueprintDraftSaveRequest(c.msg); got != c.want {
			t.Fatalf("isBlueprintDraftSaveRequest(%q) = %v, want %v", c.msg, got, c.want)
		}
	}
}

func TestBlueprintAffirmativeActionWords(t *testing.T) {
	for _, msg := range []string{"setuju", "lakukan", "jalankan"} {
		if got := classifyShortReply(msg); got != "affirm" {
			t.Fatalf("classifyShortReply(%q) = %q, want affirm", msg, got)
		}
	}
}

func TestIsExplicitBlueprintProposalFallbackCommand(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{"buatkan proposal 10 slot kisi-kisi", true},
		{"langsung buatkan 10 slot sekarang", true},
		{"Ya, bantu kau menambahkan slot kisi-kisi, kita akan membuat 40 soal, dan kita sudah membuat 5 saat ini, masih kurang 35 lagi", false},
		{"generate 5 slot kisi-kisi", false},
		{"susun 8 kisi-kisi", false},
		{"aku ingin membuat kisi-kisi", false},
		{"buatkan 10 soal pilihan ganda", false},
	}
	for _, c := range cases {
		if got := isExplicitBlueprintProposalFallbackCommand(c.msg); got != c.want {
			t.Errorf("isExplicitBlueprintProposalFallbackCommand(%q) = %v, want %v", c.msg, got, c.want)
		}
	}
}
