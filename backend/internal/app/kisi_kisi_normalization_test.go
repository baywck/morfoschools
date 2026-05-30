package app

import "testing"

func TestNormalizeKisiKisiFieldsStripsKDLabelFromMateri(t *testing.T) {
	_, desc, materi, indikator := normalizeKisiKisiFields(
		"3.7",
		"Memahami dan menerapkan konsep KD 3.2 Sistem dan Dinamika Demokrasi Pancasila sesuai kompetensi yang diukur.",
		"KD 3.2 Sistem dan Dinamika Demokrasi Pancasila",
		"Menganalisis penerapan demokrasi Pancasila dalam pengambilan keputusan bersama",
	)

	if materi != "Sistem dan Dinamika Demokrasi Pancasila" {
		t.Fatalf("expected KD label stripped from materi, got %q", materi)
	}
	if desc == materi {
		t.Fatalf("description must not duplicate materi")
	}
	if desc != indikator {
		t.Fatalf("expected generic description replaced by indikator-derived competency, got %q", desc)
	}
}

func TestNormalizeKisiKisiFieldsKeepsDistinctDescription(t *testing.T) {
	_, desc, materi, _ := normalizeKisiKisiFields(
		"3.6",
		"Menganalisis upaya preventif pelanggaran HAM di lingkungan sekolah",
		"Harmonisasi Hak dan Kewajiban Asasi Manusia",
		"Menentukan strategi pencegahan pelanggaran HAM berdasarkan kasus kontekstual",
	)

	if materi != "Harmonisasi Hak dan Kewajiban Asasi Manusia" {
		t.Fatalf("unexpected materi: %q", materi)
	}
	if desc != "Menganalisis upaya preventif pelanggaran HAM di lingkungan sekolah" {
		t.Fatalf("expected distinct description preserved, got %q", desc)
	}
}
