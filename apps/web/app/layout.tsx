import type { Metadata } from "next";
import "./globals.css";
export const metadata: Metadata = {
  title: "KRIEMHILD · Worldbuilding",
  description: "Your worlds. Their histories. Your words.",
};
export default function Layout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
