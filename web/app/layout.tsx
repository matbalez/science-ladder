import type { Metadata, Viewport } from "next";
import { Shell } from "@/components/shell";
import "@fontsource-variable/dm-sans";
import "@fontsource/ibm-plex-mono/400.css";
import "@fontsource/ibm-plex-mono/500.css";
import "./globals.css";
export const metadata: Metadata = {
  title: {
    default: "Science Ladder",
    template: "%s · Science Ladder",
  },
  description:
    "Explore computational science challenges, submit solutions, and compare verified results.",
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
