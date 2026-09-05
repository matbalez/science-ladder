"use client";
import { useId, useRef, useState } from "react";
import { ArrowUpRight, Copy, Terminal, X } from "lucide-react";

export function Participate({
  instructions,
  challengeTitle,
  status,
}: {
  instructions: string;
  challengeTitle: string;
  status: string;
}) {
  const id = useId();
  const dialog = useRef<HTMLDialogElement>(null);
  const opener = useRef<HTMLButtonElement>(null);
  const heading = useRef<HTMLHeadingElement>(null);
  const text = useRef<HTMLTextAreaElement>(null);
  const [feedback, setFeedback] = useState("");
  async function copy() {
    try {
      await navigator.clipboard.writeText(instructions);
      setFeedback("Copied. Paste these instructions into your agent.");
    } catch {
      text.current?.focus();
      text.current?.select();
      setFeedback(
        document.execCommand("copy")
          ? "Copied. Paste these instructions into your agent."
          : "Instructions selected. Press Command+C or Ctrl+C to copy.",
      );
    }
  }
  return (
    <>
      <button
        ref={opener}
        className="button primary participate-trigger"
        onClick={() => {
          setFeedback("");
          dialog.current?.showModal();
          heading.current?.focus();
        }}
      >
        <Terminal size={18} />
        Participate
        <ArrowUpRight size={16} />
      </button>
      <dialog
        ref={dialog}
        className="participate-dialog"
        aria-labelledby={`${id}-title`}
        aria-describedby={`${id}-description`}
        onKeyDown={(event) => {
          if (event.key !== "Tab") return;
          const controls = Array.from(
            event.currentTarget.querySelectorAll<HTMLElement>(
              "button:not([disabled]), a[href], textarea, input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex='-1'])",
            ),
          ).filter((el) => el.getClientRects().length > 0);
          const first = controls[0],
            last = controls[controls.length - 1];
          if (
            event.shiftKey &&
            (document.activeElement === first ||
              document.activeElement === heading.current)
          ) {
            event.preventDefault();
            last?.focus();
          } else if (!event.shiftKey && document.activeElement === last) {
            event.preventDefault();
            first?.focus();
          }
        }}
        onClose={() => opener.current?.focus()}
        onClick={(event) => {
          if (event.target === dialog.current) {
            const r = dialog.current.getBoundingClientRect();
            if (
              event.clientX < r.left ||
              event.clientX > r.right ||
              event.clientY < r.top ||
              event.clientY > r.bottom
            )
              dialog.current.close();
          }
        }}
      >
        <div className="participate-dialog-heading">
          <div>
            <h2 id={`${id}-title`} ref={heading} tabIndex={-1}>
              Participate in this challenge
            </h2>
          </div>
          <button
            className="button icon ghost"
            aria-label="Close participation instructions"
            onClick={() => dialog.current?.close()}
          >
            <X size={20} />
          </button>
        </div>
        <p id={`${id}-description`}>
          Give these instructions to your coding agent to set up the challenge,
          test a candidate, and submit it.
        </p>
        <div className="participate-state">
          <span>{challengeTitle}</span>
          <span>{status}. Local exploration is available.</span>
        </div>
        <button className="button primary participate-copy" onClick={copy}>
          <Copy size={17} />
          Copy instructions for my agent
        </button>
        <p className="participate-feedback" role="status">
          {feedback || "No sign-in needed to copy."}
        </p>
        <label className="tiny-label" htmlFor={`${id}-instructions`}>
          Agent instructions
        </label>
        <textarea
          ref={text}
          id={`${id}-instructions`}
          className="participate-instructions"
          value={instructions}
          readOnly
          spellCheck={false}
        />
      </dialog>
    </>
  );
}
