"use client";
import {
  useState,
  useId,
  cloneElement,
  isValidElement,
  type ReactElement,
} from "react";
import {
  AlertCircle,
  ArrowUpRight,
  Check,
  Copy,
  Download,
  Loader2,
  RefreshCw,
} from "lucide-react";
import { ApiError } from "@/lib/api";
import { humanize } from "@/lib/scientific";
import type { Finding } from "@/lib/types";
export function Badge({
  children,
  tone = "",
}: {
  children: React.ReactNode;
  tone?: string;
}) {
  return <span className={`badge ${tone}`}>{children}</span>;
}
export function Status({ value }: { value: string }) {
  return (
    <Badge
      tone={
        /published|valid$|ready|claimed|finalized|public_frontier/.test(value)
          ? "lime"
          : /fail|reject|compromis|invalid/.test(value)
            ? "red"
            : /pending|queued|running|draft|paused/.test(value)
              ? "amber"
              : ""
      }
    >
      {humanize(value)}
    </Badge>
  );
}
export function Loading({
  label = "Reading the scientific record",
}: {
  label?: string;
}) {
  return (
    <div className="loading" role="status">
      <Loader2 size={20} className="spin" />
      {label}
      <span className="loading-line" />
    </div>
  );
}
export function ErrorMessage({
  error,
  retry,
}: {
  error?: Error;
  retry?: () => void;
}) {
  if (!error) return null;
  return (
    <div className="error" role="alert">
      <AlertCircle size={18} />
      <div>
        <strong>
          {error instanceof ApiError
            ? humanize(error.code)
            : "Could not complete this step"}
        </strong>
        <p>{error.message}</p>
        {error instanceof ApiError && error.details ? (
          <details>
            <summary>View details</summary>
            <pre>{JSON.stringify(error.details, null, 2)}</pre>
          </details>
        ) : null}
      </div>
      {retry && (
        <button className="button small ghost" onClick={retry}>
          <RefreshCw size={14} /> Retry
        </button>
      )}
    </div>
  );
}
export function CopyButton({
  text,
  children = "Copy",
  className = "",
}: {
  text: string;
  children?: React.ReactNode;
  className?: string;
}) {
  const [copied, setCopied] = useState(false);
  const [failed, setFailed] = useState(false);
  return (
    <button
      className={`button small ghost ${className}`}
      onClick={async () => {
        try {
          await navigator.clipboard.writeText(text);
          setCopied(true);
          setTimeout(() => setCopied(false), 1800);
        } catch {
          setFailed(true);
        }
      }}
    >
      <span aria-live="polite">
        {failed ? (
          "Select text to copy"
        ) : copied ? (
          <>
            <Check size={14} /> Copied
          </>
        ) : (
          <>
            <Copy size={14} />
            {children}
          </>
        )}
      </span>
    </button>
  );
}
export function CodeBlock({ code, label }: { code: string; label?: string }) {
  return (
    <div className="code-block">
      <div className="code-head">
        <span>{label || "TERMINAL"}</span>
        <CopyButton text={code} />
      </div>
      <pre>{code}</pre>
    </div>
  );
}
export function JsonViewer({
  value,
  label = "View signed record",
}: {
  value: unknown;
  label?: string;
}) {
  return (
    <details className="json-view">
      <summary>{label}</summary>
      <pre>{JSON.stringify(value, null, 2)}</pre>
    </details>
  );
}
export function ExternalLink({
  href,
  children,
  className = "",
}: {
  href: string;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <a className={className} href={href} target="_blank" rel="noreferrer">
      {children}
      <ArrowUpRight size={14} />
    </a>
  );
}
export function DownloadButton({
  text,
  filename,
  children = "Download YAML",
}: {
  text: string;
  filename: string;
  children?: React.ReactNode;
}) {
  return (
    <button
      className="button small ghost"
      onClick={() => {
        const url = URL.createObjectURL(
          new Blob([text], { type: "text/plain;charset=utf-8" }),
        );
        const a = document.createElement("a");
        a.href = url;
        a.download = filename;
        a.click();
        setTimeout(() => URL.revokeObjectURL(url), 1000);
      }}
    >
      <Download size={14} />
      {children}
    </button>
  );
}
export function Findings({ findings }: { findings?: Finding[] }) {
  return findings?.length ? (
    <ul className="findings">
      {findings.map((f, i) => (
        <li key={i}>
          <Status value={f.severity || "finding"} />
          <div>
            <strong>{f.message || f.code || "Review finding"}</strong>
            {f.path && <code>{f.path}</code>}
            {f.detail && <p>{f.detail}</p>}
          </div>
        </li>
      ))}
    </ul>
  ) : null;
}
export function Empty({
  title,
  children,
  icon,
}: {
  title: string;
  children: React.ReactNode;
  icon?: React.ReactNode;
}) {
  return (
    <div className="empty-state">
      {icon}
      <h3>{title}</h3>
      <p>{children}</p>
    </div>
  );
}
export function Field({
  label,
  help,
  children,
}: {
  label: string;
  help?: string;
  children: React.ReactNode;
}) {
  const id = useId();
  const control = isValidElement(children)
    ? cloneElement(
        children as ReactElement<{ id?: string; "aria-describedby"?: string }>,
        { id, "aria-describedby": help ? `${id}-help` : undefined },
      )
    : children;
  return (
    <div className="field">
      <label htmlFor={id}>{label}</label>
      {control}
      {help && <small id={`${id}-help`}>{help}</small>}
    </div>
  );
}
