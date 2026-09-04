"use client";
import { useState } from "react";
import {
  ArrowLeft,
  ArrowRight,
  Download,
  Fingerprint,
  ShieldCheck,
} from "lucide-react";
import { useResource } from "@/lib/api";
import { dateLabel, humanize, shortHash } from "@/lib/scientific";
import {
  Badge,
  DownloadButton,
  Empty,
  ErrorMessage,
  JsonViewer,
  Loading,
} from "./ui";
interface Checkpoint {
  id: string;
  digest: string;
  bundle: { checkpoint: unknown; witnesses: unknown[]; events: unknown[] };
  quorumVerified: boolean;
  issuedAt: string;
}
export function AuditStatus() {
  const [cursors, setCursors] = useState(["0"]);
  const after = cursors[cursors.length - 1];
  const resource = useResource<{
    checkpoints: Checkpoint[];
    deploymentMode: string;
  }>(`/audit/checkpoints?after=${encodeURIComponent(after)}`, 30000);
  const data = resource.data;
  return (
    <section className="audit-status">
      <div className="section-title">
        <div>
          <div className="section-kicker">LIVE HOST EVIDENCE</div>
          <h3>
            <Fingerprint size={18} />
            Public audit checkpoints
          </h3>
        </div>
        {data && <Badge>{humanize(data.deploymentMode)}</Badge>}
      </div>
      <p>
        Inspect published checkpoint bundles and their witness receipts. Quorum
        status below is reported by this host; verify the exported signatures
        and trusted keys independently.
      </p>
      <ErrorMessage error={resource.error} retry={resource.refresh} />
      {resource.loading && !data ? (
        <Loading label="Reading public checkpoint evidence" />
      ) : data?.checkpoints.length ? (
        <>
          <div className="checkpoint-list">
            {data.checkpoints.map((c) => (
              <article key={c.id}>
                <div className="checkpoint-heading">
                  <span className="mono">
                    #{c.id} · {shortHash(c.digest)}
                  </span>
                  <Badge tone={c.quorumVerified ? "lime" : "amber"}>
                    {c.quorumVerified
                      ? "Host reports witness quorum"
                      : "Witness quorum not established"}
                  </Badge>
                </div>
                <div className="checkpoint-facts">
                  <span>{dateLabel(c.issuedAt)}</span>
                  <span>
                    {c.bundle.witnesses?.length || 0} witness receipts
                  </span>
                  <span>{c.bundle.events?.length || 0} public events</span>
                  <DownloadButton
                    text={JSON.stringify(c.bundle, null, 2)}
                    filename={`checkpoint-${c.id}.json`}
                  >
                    Export bundle
                  </DownloadButton>
                </div>
                <JsonViewer
                  value={c.bundle}
                  label="Inspect signed checkpoint and witness evidence"
                />
              </article>
            ))}
          </div>
          <div className="form-actions">
            {cursors.length > 1 && (
              <button
                className="button small ghost"
                onClick={() => setCursors((v) => v.slice(0, -1))}
              >
                <ArrowLeft size={13} />
                Earlier page
              </button>
            )}
            {data.checkpoints.length === 20 && (
              <button
                className="button small ghost"
                onClick={() =>
                  setCursors((v) => [
                    ...v,
                    data.checkpoints[data.checkpoints.length - 1].id,
                  ])
                }
              >
                Later checkpoints
                <ArrowRight size={13} />
              </button>
            )}
          </div>
        </>
      ) : data ? (
        <Empty
          title="No signed checkpoints have been published."
          icon={<ShieldCheck size={24} />}
        >
          This host has no public checkpoint evidence to display yet. An empty
          record is not evidence of independent witness quorum.
        </Empty>
      ) : null}
      <a
        className="button small ghost"
        href="/.well-known/science-ladder-keys.json"
        download
      >
        <Download size={14} />
        Download published signing keys
      </a>
    </section>
  );
}
