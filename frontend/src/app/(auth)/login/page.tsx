"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth-provider";
import { Button } from "@/components/ui/button";
import { InputField } from "@/components/ui/input-field";
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
    <div className="min-h-screen flex items-center justify-center bg-[var(--background)] p-5">
      <div className="w-full max-w-sm space-y-6">
        {/* Header */}
        <div className="text-center">
          <div className="mx-auto mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-[var(--primary)]">
            <span className="text-lg font-bold text-[var(--primary-foreground)]">M</span>
          </div>
          <h1 className="text-xl font-semibold text-[var(--foreground)]">Sign In</h1>
          <p className="mt-1 text-sm text-[var(--muted-foreground)]">
            Masuk ke akun Morfoschools Anda
          </p>
        </div>

        {/* Form Card */}
        <div className="rounded-lg border border-[var(--border)] bg-[var(--card)] p-5 space-y-3">
          {error && (
            <div className="rounded-lg border border-l-4 border-l-[var(--danger)] bg-[var(--danger-soft)] p-3">
              <p className="text-xs font-medium text-[var(--danger)]">{error}</p>
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-3">
            <InputField
              label="Email"
              type="email"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              error={fieldErrors.email}
              icon={<Mail className="h-3.5 w-3.5" />}
              autoComplete="email"
            />

            <InputField
              label="Password"
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              error={fieldErrors.password}
              icon={<Lock className="h-3.5 w-3.5" />}
              autoComplete="current-password"
            />

            <Button type="submit" loading={loading} className="w-full">
              <LogIn className="h-3.5 w-3.5" /> Sign In
            </Button>
          </form>
        </div>

        {/* Dev personas */}
        <div className="rounded-lg border border-[var(--border)] bg-[var(--card)] p-4">
          <p className="mb-2 text-[11px] font-medium text-[var(--muted-foreground)]">Dev quick login:</p>
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
                className="rounded-md bg-[var(--muted)] px-2.5 py-1 text-[11px] font-medium text-[var(--muted-foreground)] hover:bg-[var(--border)] transition-colors"
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
