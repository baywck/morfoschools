import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = {
  title: "Morfoschools",
  description: "LMS SaaS untuk sekolah Indonesia",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="id" suppressHydrationWarning>
      <body className="antialiased">{children}</body>
    </html>
  );
}
