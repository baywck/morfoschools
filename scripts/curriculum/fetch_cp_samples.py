#!/usr/bin/env python3
import json, re, sys, time
from pathlib import Path
from urllib.request import Request, urlopen
from urllib.error import HTTPError, URLError

BASE = "https://guru.kemendikdasmen.go.id/kurikulum/referensi-penerapan/capaian-pembelajaran"
OUT = Path(".ai/curriculum-samples")

SAMPLES = [
    {"group": "sd-sma-sederajat", "levels": ["sd-sma", "sd-sma-sederajat"], "subject": "Pendidikan Pancasila", "phases": ["c", "f"]},
    {"group": "sd-sma-sederajat", "levels": ["sd-sma", "sd-sma-sederajat"], "subject": "Bahasa Indonesia", "phases": ["a"]},
    {"group": "sd-sma-sederajat", "levels": ["sd-sma", "sd-sma-sederajat"], "subject": "Matematika", "phases": ["d"]},
    {"group": "sd-sma-sederajat", "levels": ["sd-sma", "sd-sma-sederajat"], "subject": "Informatika", "phases": ["e"]},
    {"group": "sd-sma-sederajat", "levels": ["sd-sma", "sd-sma-sederajat"], "subject": "Biologi", "phases": ["f"]},
    {"group": "sd-sma-sederajat", "levels": ["sd-sma", "sd-sma-sederajat"], "subject": "Ilmu Pengetahuan Alam dan Sosial (IPAS)", "phases": ["b"]},
    {"group": "sd-sma-sederajat", "levels": ["sd-sma", "sd-sma-sederajat"], "subject": "Bahasa Inggris Tingkat Lanjut", "phases": ["f"]},
    {"group": "smk-sederajat", "levels": ["smk", "smk-sederajat"], "subject": "Dasar-dasar Pengembangan Perangkat Lunak dan Gim", "phases": ["e"]},
    {"group": "smk-sederajat", "levels": ["smk", "smk-sederajat"], "subject": "Rekayasa Perangkat Lunak", "phases": ["f"]},
]

SPECIAL_SLUGS = {
    "Ilmu Pengetahuan Alam dan Sosial (IPAS)": "ilmu-pengetahuan-alam-dan-sosial-ipas",
    "Pendidikan Jasmani, Olahraga, dan Kesehatan (PJOK)": "pendidikan-jasmani-olahraga-dan-kesehatan-pjok",
}

def slugify(name: str) -> str:
    if name in SPECIAL_SLUGS:
        return SPECIAL_SLUGS[name]
    s = name.lower().strip()
    s = re.sub(r"\((.*?)\)", r" \1 ", s)
    s = s.replace("&", " dan ")
    s = re.sub(r"[^a-z0-9]+", "-", s)
    return re.sub(r"-+", "-", s).strip("-")

def fetch(url: str):
    req = Request(url, headers={"User-Agent": "Morfoschools CP sampler/1.0"})
    with urlopen(req, timeout=25) as r:
        return r.status, r.read()

def try_sample(sample, phase):
    subject = sample["subject"]
    slug = slugify(subject)
    attempts = []
    for level in sample["levels"]:
        url = f"{BASE}/{level}/subject/{slug}/fase-{phase}.json"
        try:
            status, body = fetch(url)
            data = json.loads(body.decode("utf-8"))
            path = OUT / level / slug / f"fase-{phase}.json"
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n")
            return {"subject": subject, "slug": slug, "group": sample["group"], "level": level, "phase": phase, "url": url, "status": status, "path": str(path), "data": data, "attempts": attempts}
        except HTTPError as e:
            attempts.append(f"{level}: HTTP {e.code}")
        except (URLError, TimeoutError, json.JSONDecodeError, Exception) as e:
            attempts.append(f"{level}: {type(e).__name__}: {e}")
        time.sleep(0.2)
    return {"subject": subject, "slug": slug, "group": sample["group"], "level": "", "phase": phase, "url": "", "status": "FAIL", "path": "", "data": None, "attempts": attempts}

def summarize(result):
    data = result.get("data") or {}
    compiled = data.get("compiled") if isinstance(data, dict) else None
    params = compiled.get("params", {}) if isinstance(compiled, dict) else {}
    elements = compiled.get("elements", []) if isinstance(compiled, dict) else []
    elem_names = [e.get("name", "") for e in elements if isinstance(e, dict)]
    general = compiled.get("generalInfo", "") if isinstance(compiled, dict) else ""
    return {
        "subject": result["subject"],
        "slug": result["slug"],
        "group": result["group"],
        "level": result["level"],
        "phase": result["phase"],
        "status": result["status"],
        "has_compiled": isinstance(compiled, dict),
        "params": params,
        "general_chars": len(general or ""),
        "elements_count": len(elements),
        "element_names": elem_names,
        "path": result["path"],
        "attempts": result.get("attempts", []),
    }

def main():
    OUT.mkdir(parents=True, exist_ok=True)
    summaries = []
    for sample in SAMPLES:
        for phase in sample["phases"]:
            res = try_sample(sample, phase)
            summaries.append(summarize(res))
            print(f"{res['status']} {sample['subject']} fase-{phase} level={res['level'] or '-'}")
    (OUT / "summary.json").write_text(json.dumps(summaries, ensure_ascii=False, indent=2) + "\n")
    lines = ["# Curriculum CP sample fetch summary", "", "| Subject | Group | Level | Phase | Status | Elements | Element names | Path / Attempts |", "|---|---|---|---|---:|---:|---|---|"]
    for s in summaries:
        names = ", ".join(s["element_names"][:6])
        if len(s["element_names"]) > 6:
            names += ", …"
        detail = s["path"] or "<br>".join(s["attempts"])
        lines.append(f"| {s['subject']} | {s['group']} | {s['level'] or '-'} | {s['phase'].upper()} | {s['status']} | {s['elements_count']} | {names} | {detail} |")
    (OUT / "summary.md").write_text("\n".join(lines) + "\n")

if __name__ == "__main__":
    main()
