package app

import "testing"

func TestIsBlueprintSlotCreateCommand(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		// Imperative creation commands → true
		{"buatkan 10 slot", true},
		{"buat 10 kisi-kisi", true},
		{"langsung buatkan 10 sekaligus", true},
		{"tidak usah preview, buat 10 slot sekaligus", true},
		{"generate 5 slot kisi-kisi", true},
		{"susun 8 kisi-kisi", true},
		// Planning / intention / discussion → false
		{"aku ingin membuat kisi-kisi", false},
		{"aku berencana membuat 50 soal", false},
		{"bantu aku membuat kisi-kisi", false},
		{"bagaimana cara membuat kisi-kisi yang baik?", false},
		{"ayo diskusi kisi-kisi dulu", false},
		// Missing target or verb → false
		{"buatkan 10 soal pilihan ganda", false},
		{"jelaskan 10 slot", false},
		{"buat kisi-kisi", false},
	}
	for _, c := range cases {
		if got := isBlueprintSlotCreateCommand(c.msg); got != c.want {
			t.Errorf("isBlueprintSlotCreateCommand(%q) = %v, want %v", c.msg, got, c.want)
		}
	}
}
