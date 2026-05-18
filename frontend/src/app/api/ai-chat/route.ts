import { NextResponse } from "next/server";

export const runtime = "nodejs";

type ChatMessage = {
  role: "system" | "user" | "assistant";
  content: string;
};

const DEFAULT_BASE_URL = "http://43.156.68.104:20128/v1";
const DEFAULT_MODEL = "MORFOSCHOOLS";
const MAX_AI_OUTPUT_TOKENS = 900;

function getConfig() {
  return {
    baseUrl: (process.env.AI_BASE_URL ?? DEFAULT_BASE_URL).replace(/\/$/, ""),
    apiKey: process.env.AI_API_KEY,
    model: process.env.AI_MODEL ?? DEFAULT_MODEL,
  };
}

const SYSTEM_PROMPT = `Kamu adalah MORFOSCHOOLS AI Agent untuk LMS sekolah Indonesia. Jawab dalam Bahasa Indonesia yang jelas, praktis, dan aman. Bantu guru/admin terkait kelas, siswa, course, exam, grading, jadwal ujian, dan operasional sekolah. Jangan mengklaim sudah membaca data tenant nyata kecuali data itu diberikan eksplisit di chat. Critical path ujian tidak boleh bergantung pada API eksternal.`;

export async function POST(request: Request) {
  const { baseUrl, apiKey, model } = getConfig();

  if (!apiKey) {
    return NextResponse.json({ error: "AI_API_KEY belum dikonfigurasi di server." }, { status: 500 });
  }

  let payload: unknown;
  try {
    payload = await request.json();
  } catch {
    return NextResponse.json({ error: "Payload JSON tidak valid." }, { status: 400 });
  }

  const messages = (payload as { messages?: ChatMessage[] }).messages;
  if (!Array.isArray(messages) || messages.length === 0) {
    return NextResponse.json({ error: "Minimal kirim satu message." }, { status: 400 });
  }

  const systemMessage: ChatMessage = { role: "system", content: SYSTEM_PROMPT };

  try {
    const response = await fetch(`${baseUrl}/chat/completions`, {
      method: "POST",
      headers: {
        Authorization: `Bearer ${apiKey}`,
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        model,
        messages: [systemMessage, ...messages],
        temperature: 0.4,
        max_tokens: MAX_AI_OUTPUT_TOKENS,
        stream: false,
      }),
    });

    const responseText = await response.text();
    let responseJson: unknown = null;
    try { responseJson = JSON.parse(responseText); } catch { /* ignore */ }

    if (!response.ok) {
      return NextResponse.json({ error: `AI error (${response.status})` }, { status: 502 });
    }

    const content = (responseJson as { choices?: Array<{ message?: { content?: string } }> })?.choices?.[0]?.message?.content?.trim();

    if (!content) {
      return NextResponse.json({ error: "Response AI kosong." }, { status: 502 });
    }

    return NextResponse.json({ message: { role: "assistant", content }, model });
  } catch (err) {
    return NextResponse.json({ error: err instanceof Error ? err.message : "Gagal menghubungi AI." }, { status: 502 });
  }
}
