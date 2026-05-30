package app

import "testing"

func TestExtractBlueprintDraftSlotsKeepsCPLine(t *testing.T) {
	content := `21 · Bhinneka Tunggal Ika · C5 · Esai
Materi: Solusi konflik berbasis kearifan lokal
TP: Peserta didik dapat mengevaluasi efektivitas pendekatan kearifan lokal dengan menyertakan minimal dua kriteria.
Indikator: Disajikan studi kasus konflik, peserta didik dapat mengevaluasi efektivitas pendekatan kearifan lokal.
CP: Peserta didik mampu menganalisis potensi konflik dan bersama-sama memberi solusi yang berkeadilan terhadap permasalahan keberagaman di masyarakat.`
	slots := extractBlueprintDraftSlotsFromText(content)
	if len(slots) != 1 {
		t.Fatalf("expected 1 slot, got %d: %#v", len(slots), slots)
	}
	if slots[0].CapaianPembelajaran == "" {
		t.Fatalf("expected CP to be parsed: %#v", slots[0])
	}
	if slots[0].CapaianPembelajaran != "Peserta didik mampu menganalisis potensi konflik dan bersama-sama memberi solusi yang berkeadilan terhadap permasalahan keberagaman di masyarakat." {
		t.Fatalf("unexpected CP: %q", slots[0].CapaianPembelajaran)
	}
}

func TestExtractBlueprintDraftSlotsFromNarrativeFormat(t *testing.T) {
	content := `**Slot 16** · UUD 1945 · C5 · Esai

Materi: Pelanggaran hak dan pengingkaran kewajiban warga negara

TP: Peserta didik dapat mengevaluasi solusi penanganan kasus pelanggaran hak warga negara berdasarkan Undang-Undang Dasar 1945 dengan memberikan minimal dua kriteria evaluasi.

Indikator: Disajikan studi kasus pelanggaran hak sipil warga negara yang belum terselesaikan, peserta didik dapat mengevaluasi kelemahan solusi yang diajukan pemerintah berdasarkan UUD 1945 dengan menyertakan minimal dua kriteria evaluasi.

17 · NKRI · C3 · Benar/Salah

Materi: Bentuk negara dan bentuk pemerintahan Indonesia

TP: Peserta didik dapat menerapkan konsep bentuk negara kesatuan dan sistem presidensial dalam menentukan benar atau salah pernyataan tentang kewenangan pemerintah pusat dan daerah.

Indikator: Disajikan lima pernyataan tentang pembagian kewenangan pusat-daerah dalam NKRI, peserta didik dapat menentukan benar atau salah setiap pernyataan berdasarkan prinsip negara kesatuan.`
	slots := extractBlueprintDraftSlotsFromText(content)
	if len(slots) != 2 {
		t.Fatalf("expected 2 slots, got %d: %#v", len(slots), slots)
	}
	if slots[0].Position != 16 || slots[0].ElemenCP != "UUD 1945" || slots[0].CognitiveLevel != "C5" || slots[0].QuestionType != "essay" {
		t.Fatalf("slot 16 parsed incorrectly: %#v", slots[0])
	}
	if slots[1].Position != 17 || slots[1].ElemenCP != "NKRI" || slots[1].CognitiveLevel != "C3" || slots[1].QuestionType != "true_false" {
		t.Fatalf("slot 17 parsed incorrectly: %#v", slots[1])
	}
}
