package app

import "testing"

func TestClassifyShortReply_Affirm(t *testing.T) {
	cases := []string{"ya", "oke", "Ok", "iya", "lanjut", "lanjutkan", "setuju", "Yes", "OK!", "oke."}
	for _, c := range cases {
		if got := classifyShortReply(c); got != "affirm" {
			t.Errorf("classifyShortReply(%q) = %q, want affirm", c, got)
		}
	}
}

func TestClassifyShortReply_Deny(t *testing.T) {
	cases := []string{"tidak", "batal", "no", "jangan", "cancel", "stop"}
	for _, c := range cases {
		if got := classifyShortReply(c); got != "deny" {
			t.Errorf("classifyShortReply(%q) = %q, want deny", c, got)
		}
	}
}

func TestClassifyShortReply_Neutral(t *testing.T) {
	// Anything that looks like a fresh instruction should NOT be auto-confirmed.
	cases := []string{
		"",
		"buatkan student baru bernama Andi",
		"oke tapi ganti emailnya jadi lain@example.com",
		"saya rasa tidak perlu password segitu panjang",
		"halo",
		"siapa kamu",
		"lanjutkan ke siswa berikutnya dengan email berbeda",
	}
	for _, c := range cases {
		if got := classifyShortReply(c); got != "" {
			t.Errorf("classifyShortReply(%q) = %q, want empty (neutral)", c, got)
		}
	}
}

func TestClassifyShortReply_MultiWord(t *testing.T) {
	// Two-three word replies that lead with a confirmation word: still affirm.
	if got := classifyShortReply("oke lanjutkan"); got != "affirm" {
		t.Errorf("classifyShortReply(oke lanjutkan) = %q, want affirm", got)
	}
	if got := classifyShortReply("ya silakan"); got != "affirm" {
		t.Errorf("classifyShortReply(ya silakan) = %q, want affirm", got)
	}
	if got := classifyShortReply("batal saja"); got != "deny" {
		t.Errorf("classifyShortReply(batal saja) = %q, want deny", got)
	}
}
