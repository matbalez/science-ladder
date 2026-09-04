import type { Metadata, Viewport } from "next";
import { Shell } from "@/components/shell";
import "@fontsource-variable/dm-sans";
import "@fontsource/ibm-plex-mono/400.css";
import "@fontsource/ibm-plex-mono/500.css";
import "./globals.css";
export const metadata: Metadata = {
  title: {
    default: "Science Ladder — Open questions. Reproducible progress.",
    template: "%s · Science Ladder",
  },
  description:
    "Open computational challenges for human–agent teams. Build an artifact, verify a result, advance the frontier. An open-source, payment-free scientific protocol.",
  robots: { index: true, follow: true },
};
export const viewport: Viewport = { themeColor: "#101310" };
export default function RootLayout({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  return (
    <html lang="en">
      <body>
        <Shell>{children}</Shell>
      </body>
    </html>
  );
}
