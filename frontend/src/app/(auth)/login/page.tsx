"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth-provider";
import { Button } from "@/components/ui/button";
import { InputField } from "@/components/ui/input-field";
import { Mail, Lock } from "lucide-react";

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
    <div className="flex min-h-screen items-center justify-center bg-[color:var(--background)] px-4">
      <div className="w-full max-w-sm">
        {/* Logo */}
        <div className="mb-8 text-center">
          <div className="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-[color:var(--brand)] shadow-lg">
            <span className="text-xl font-bold text-white">M</span>
          </div>
          <h1 className="text-xl font-bold text-[color:var(--foreground)]">Morfoschools</h1>
          <p className="mt-1 text-sm text-[color:var(--foreground-muted)]">
            Masuk ke akun Anda
          </p>
        </div>

        {/* Form */}
        <form onSubmit={handleSubmit} className="space-y-4">
          {error && (
            <div className="rounded-xl border border-[color:var(--danger)]/20 bg-[color:var(--danger)]/5 px-4 py-3" role="alert">
              <p className="text-sm text-[color:var(--danger)]">{error}</p>
            </div>
          )}

          <InputField
            label="Email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            error={fieldErrors.email}
            icon={<Mail className="h-4 w-4" />}
            autoComplete="email"
          />

          <InputField
            label="Password"
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            error={fieldErrors.password}
            icon={<Lock className="h-4 w-4" />}
            autoComplete="current-password"
          />

          <Button type="submit" loading={loading} className="w-full">
            Masuk
          </Button>
        </form>

        {/* Dev personas */}
        {process.env.NODE_ENV === "development" && (
          <div className="mt-6 border-t border-[color:var(--border)] pt-4">
            <p className="mb-2 text-xs text-[color:var(--foreground-muted)]">Dev quick login:</p>
            <div className="flex flex-wrap gap-1">
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
                  className="rounded-lg bg-[color:var(--surface-subtle)] px-2 py-1 text-xs text-[color:var(--foreground-muted)] hover:bg-[color:var(--border)] transition-colors"
                >
                  {p.label}
                </button>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
