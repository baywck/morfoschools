# Question Quality Standard — Kurikulum Merdeka

## 7 Kriteria Soal Berkualitas

1. Valid
   - Soal benar-benar mengukur kompetensi yang ingin diukur.
   - Level kognitif, KKO pada TP, dan indikator soal harus selaras.
   - Soal C4 wajib menuntut analisis nyata, bukan hafalan.
   - Rekomendasi validitas manusia: guru lain menjawab soal dan membandingkan hasilnya.

2. HOTS dan Kontekstual
   - Kurikulum Merdeka menekankan Higher Order Thinking Skills untuk C4-C6.
   - Stimulus berkaitan dengan kehidupan nyata dan kasus autentik.
   - Menuntut berpikir kritis, bukan sekadar mengingat.

3. Memiliki Stimulus
   - Setiap soal HOTS wajib memiliki stimulus.
   - Stimulus dianjurkan berupa wacana/teks berita, kasus nyata/fenomena sosial, data statistik/tabel, grafik/diagram, kutipan UU/peraturan, gambar/ilustrasi, infografis, atau dialog/percakapan.
   - Soal tanpa stimulus cenderung hafalan dan tidak sesuai untuk HOTS Kurikulum Merdeka.

4. Adil dan Tidak Bias
   - Tidak mengandung unsur SARA.
   - Tidak memihak kelompok gender, ekonomi, budaya, atau kelompok sosial tertentu.
   - Tidak membutuhkan pengalaman khusus di luar kurikulum.
   - Konteks soal dapat dipahami siswa dari berbagai latar belakang.

5. Satu Jawaban Benar untuk PG
   - Pilihan ganda harus memiliki tepat satu jawaban paling tepat dan tidak terbantahkan.
   - Distraktor harus masuk akal namun jelas salah.
   - Distraktor absurd memperlemah kualitas soal.

6. Bahasa Baku dan Jelas
   - Menggunakan Bahasa Indonesia baku sesuai PUEBI.
   - Tidak ambigu atau bermakna ganda.
   - Hindari kalimat terlalu panjang dan berbelit.
   - Hindari negasi ganda seperti “yang bukan bukan...”.

7. Mandiri / Independent
   - Jawaban satu soal tidak memberi clue untuk soal lain.
   - Setiap soal dapat dijawab tanpa bergantung pada soal lain.
   - Jika urutan soal diacak, semua soal tetap bisa dijawab.

## Distribusi Ideal Kelas XI

- LOTS C1-C2: sekitar 20%.
- MOTS C3: sekitar 30%.
- HOTS C4-C6: sekitar 50%.

## Agent Enforcement Notes

- Quality criteria are not optional prompt flavor; they are validation/review criteria for AI question authoring.
- Backend should block hard structural violations such as duplicate MCQ options, no correct answer, multiple correct answers when mode requires one, empty stimulus for HOTS slot, and invalid slot/question mismatch.
- Backend should warn for softer quality risks such as overly long wording, weak distractors, possible bias, or unclear context.
- Proposals should show quality warnings before confirmation.
