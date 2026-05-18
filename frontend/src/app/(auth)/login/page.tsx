"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth-provider";
import { Button } from "@/components/ui/button";
import { TextField } from "@/components/ui/text-field";
import { Mail, Lock, LogIn } from "lucide-react";

export default function LoginPage() {
  const router = useRouter();
  const { login } = useAuth();
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError(null);
    setFieldErrors({});
    setLoading(true);

    const res = await login(email, password);

    if (res.error) {
      if (res.error.fields) {
        setFieldErrors(res.error.fields);
      } else {
        setError(res.error.message);
      }
      setLoading(false);
      return;
    }

    router.replace("/app");
  }

  return (
    <div className="min-h-screen flex items-center justify-center bg-[var(--shell)] p-5">
      <div className="w-full max-w-sm space-y-6">
        {/* Header */}
        <div className="text-center">
          <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-white/15">
            <span className="text-lg font-bold text-white">M</span>
          </div>
          <h1 className="text-xl font-semibold text-[var(--shell-foreground)]">Morfoschools</h1>
          <p className="mt-1 text-[13px] text-[var(--shell-muted)]">
            Masuk ke akun Anda
          </p>
        </div>

        {/* Form Card */}
        <div className="rounded-2xl border border-[var(--border)] bg-[var(--card)] p-6 shadow-[0_20px_60px_rgba(0,0,0,0.12)] space-y-4">
          {error && (
            <div className="rounded-xl border-2 border-[var(--danger)] bg-[var(--danger-soft)] px-4 py-3">
              <p className="text-[11px] font-medium text-[var(--danger)]">{error}</p>
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-3">
            <TextField
              label="Email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              error={fieldErrors.email}
              prefix={<Mail size={15} />}
              autoComplete="email"
            />

            <TextField
              label="Password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              error={fieldErrors.password}
              prefix={<Lock size={15} />}
              autoComplete="current-password"
            />

            <Button type="submit" loading={loading} size="lg" className="w-full mt-2">
              <LogIn size={14} /> Sign In
            </Button>
          </form>
        </div>

        {/* Dev personas */}
        <div className="rounded-xl border border-white/10 bg-white/[0.04] p-4">
          <p className="mb-2 text-[10px] font-semibold text-[var(--shell-muted)] uppercase tracking-wider">Dev quick login</p>
          <div className="flex flex-wrap gap-1.5">
            {[
              { label: "Master", email: "master@morfoschools.local", pw: "master123" },
              { label: "Admin", email: "admin@morfoschools.local", pw: "admin123" },
              { label: "Teacher", email: "teacher@morfoschools.local", pw: "teacher123" },
              { label: "Student", email: "student@morfoschools.local", pw: "student123" },
            ].map((p) => (
              <button
                key={p.email}
                type="button"
                onClick={() => { setEmail(p.email); setPassword(p.pw); }}
                className="h-7 px-2.5 rounded-md text-[11px] font-medium border border-white/10 bg-white/[0.06] text-[var(--shell-muted)] hover:bg-white/[0.1] hover:text-[var(--shell-foreground)] transition-all"
              >
                {p.label}
              </button>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
