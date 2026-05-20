package app

import (
	"strings"
	"testing"
)

func TestInferCognitiveLevel(t *testing.T) {
	cases := []struct {
		text  string
		want  string
		why   string
	}{
		{"Sebutkan ibukota Indonesia.", "C1", "recall verb"},
		{"Jelaskan teori relativitas.", "C2", "explain verb"},
		{"Hitung luas segitiga jika alas 6 dan tinggi 4.", "C3", "calculate"},
		{"Analisis dampak inflasi pada UKM.", "C4", "analyze"},
		{"Evaluasi kebijakan ekonomi 2024.", "C5", "evaluate"},
		{"Buatlah desain rangkaian listrik paralel.", "C6", "create"},
		{"What is photosynthesis?", "", "no Indonesian/English verb keyword for C2 only matches with explain"},
	}
	for _, c := range cases {
		got, _ := inferCognitiveLevel(strings.ToLower(c.text))
		if got != c.want {
			t.Errorf("text=%q: want=%s got=%s (%s)", c.text, c.want, got, c.why)
		}
	}
}

func TestInferDifficulty_EssayAlwaysSulit(t *testing.T) {
	got, _ := inferDifficulty("short essay prompt", "essay")
	if got != "sulit" {
		t.Errorf("essay should be sulit, got %s", got)
	}
}

func TestInferDifficulty_LengthBasedForMCQ(t *testing.T) {
	short := "Apa ibukota?"
	medium := "Jelaskan secara singkat proses fotosintesis pada tumbuhan tingkat tinggi yang dipengaruhi cahaya matahari."
	long := strings.Repeat("Teks sangat panjang dengan banyak konteks. ", 8)

	if got, _ := inferDifficulty(short, "multiple_choice"); got != "mudah" {
		t.Errorf("short MCQ should be mudah, got %s", got)
	}
	if got, _ := inferDifficulty(medium, "multiple_choice"); got != "sedang" {
		t.Errorf("medium MCQ should be sedang, got %s", got)
	}
	if got, _ := inferDifficulty(long, "multiple_choice"); got != "sulit" {
		t.Errorf("long MCQ should be sulit, got %s", got)
	}
}

func TestInferDifficulty_MultiStepRaisesDifficulty(t *testing.T) {
	multiStep := "Kemudian hitung volume tabung. Lalu tentukan luas permukaan. Berikutnya bandingkan dengan kubus."
	got, _ := inferDifficulty(multiStep, "multiple_choice")
	if got != "sulit" {
		t.Errorf("multi-step text should be sulit, got %s", got)
	}
}

func TestDetectDomains_BlueprintKeywordsMapToBlueprintsDomain(t *testing.T) {
	cases := []struct {
		msg          string
		mustContain  string
	}{
		{"buatkan kisi-kisi dari soal yang sudah ada", "blueprints"},
		{"saya butuh blueprint untuk UTS matematika", "blueprints"},
		{"AKM literasi level 3 untuk kelas 10", "blueprints"},
		{"slot kompetensi untuk kurikulum merdeka", "blueprints"},
		{"buat exam baru kuis matematika", "exams"},
	}
	for _, c := range cases {
		domains := DetectDomains(c.msg)
		found := false
		for _, d := range domains {
			if d == c.mustContain {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("msg=%q: expected %q in domains, got %v", c.msg, c.mustContain, domains)
		}
	}
}
