/** Exact decimal display: scientific scores stay integer strings throughout the UI. */
export function formatTicks(ticks: string | undefined, quantum = "1"): string {
  if (ticks === undefined || !/^-?\d+$/.test(ticks)) return "—";
  const match = /^\+?(\d+)(?:\.(\d+))?$/.exec(quantum);
  if (!match) return `${ticks} ticks`;
  const decimals = match[2]?.length || 0;
  const coefficient = BigInt(match[1] + (match[2] || ""));
  const product = BigInt(ticks) * coefficient;
  const negative = product < 0n;
  const digits = (negative ? -product : product)
    .toString()
    .padStart(decimals + 1, "0");
  const integer = decimals ? digits.slice(0, -decimals) : digits;
  const fraction = decimals ? digits.slice(-decimals).replace(/0+$/, "") : "";
  return `${negative ? "-" : ""}${integer.replace(/\B(?=(\d{3})+(?!\d))/g, ",")}${fraction ? "." + fraction : ""}`;
}
export function plotRatio(ticks: string, min: bigint, max: bigint): number {
  const range = max - min;
  return range === 0n
    ? 0.5
    : Number(((BigInt(ticks) - min) * 1000000n) / range) / 1000000;
}
export function shortHash(value?: string) {
  return value ? `${value.replace(/^sha256:/, "").slice(0, 10)}…` : "Pending";
}
export function humanize(value?: string) {
  return value
    ? value.replace(/[_-]/g, " ").replace(/^./, (x) => x.toUpperCase())
    : "Pending";
}
export function dateLabel(value?: string) {
  if (!value) return "Not set";
  const date = new Date(value);
  return Number.isNaN(date.getTime())
    ? value
    : date.toLocaleDateString("en", {
        month: "short",
        day: "numeric",
        year: "numeric",
      });
}
export function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}
export function asText(value: unknown, fallback = ""): string {
  return typeof value === "string"
    ? value
    : typeof value === "number"
      ? String(value)
      : fallback;
}
export function asList(value: unknown): unknown[] {
  return Array.isArray(value) ? value : [];
}
export function safeWebUrl(value: unknown): string | undefined {
  if (typeof value !== "string") return undefined;
  try {
    const u = new URL(value);
    return ["https:", "http:"].includes(u.protocol) ? u.href : undefined;
  } catch {
    return undefined;
  }
}
