"use client";
import { useState } from "react";
import { useAction } from "@/lib/api";
import { dateLabel } from "@/lib/scientific";
import { ErrorMessage } from "./ui";

export type NotificationState = {
  configured: boolean;
  counts: { status: string; count: number }[];
  recent: {
    id: string;
    status: string;
    isTest: boolean;
    createdAt: string;
    lastError?: string;
    versionId?: string;
    candidateId?: string;
  }[];
};

export function ReviewNotifications({
  data,
  refresh,
}: {
  data?: NotificationState;
  refresh: () => void;
}) {
  const action = useAction();
  const [message, setMessage] = useState("");
  const [confirmed, setConfirmed] = useState<Record<string, boolean>>({});
  if (!data) return null;
  async function send(path: string, body = {}) {
    setMessage("");
    if (await action.run(path, body)) {
      setMessage("Email queued. Delivery status will update here.");
      refresh();
    }
  }
  return (
    <section
      className="panel content-section"
      aria-label="Review email notifications"
    >
      <div className="section-title">
        <h2>Review emails</h2>
        <button
          className="button small ghost"
          disabled={!data.configured || action.busy}
          onClick={() => send("/editor/notifications/test")}
        >
          Send test email
        </button>
      </div>
      <p>
        {data.configured
          ? "SendGrid is configured. New requests for human review are emailed automatically."
          : "Email is not configured. Review requests are saved and will be sent when a sender, recipient, and SendGrid key are connected."}
      </p>
      <ErrorMessage error={action.error} />
      {message && <p role="status">{message}</p>}
      <details>
        <summary>
          Email activity
          {data.counts.length > 0
            ? ` · ${data.counts.map((c) => `${c.count} ${c.status}`).join(", ")}`
            : ""}
        </summary>
        <p>
          Accepted means SendGrid accepted the message; it does not confirm
          inbox delivery. Check SendGrid activity for delivery or bounce
          details.
        </p>
        {data.recent.map((item) => (
          <article className="queue-item" key={item.id}>
            <p>
              {item.isTest ? "Test email" : "Review request"} · {item.status} ·{" "}
              {dateLabel(item.createdAt)}
            </p>
            {item.versionId && (
              <a
                href={`/review?version=${encodeURIComponent(item.versionId)}#decision-form`}
              >
                Open review
              </a>
            )}
            {item.candidateId && (
              <a href={`#candidate-${item.candidateId}`}>Open proposal</a>
            )}
            {item.lastError && <p>{item.lastError}</p>}
            {item.status === "uncertain" && (
              <label>
                <input
                  type="checkbox"
                  checked={!!confirmed[item.id]}
                  onChange={(e) =>
                    setConfirmed({ ...confirmed, [item.id]: e.target.checked })
                  }
                />{" "}
                I checked SendGrid activity; retrying may send a duplicate.
              </label>
            )}
            {(item.status === "failed" || item.status === "uncertain") && (
              <button
                className="button small ghost"
                disabled={
                  !data.configured ||
                  action.busy ||
                  (item.status === "uncertain" && !confirmed[item.id])
                }
                onClick={() =>
                  send(`/editor/notifications/${item.id}/retry`, {
                    confirmPossibleDuplicate: !!confirmed[item.id],
                  })
                }
              >
                Retry email
              </button>
            )}
          </article>
        ))}
      </details>
    </section>
  );
}
