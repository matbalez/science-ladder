"use client";
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { ArrowUp, Square, MessageCircle } from "lucide-react";
import type { Challenge } from "@/lib/types";
import { challengeEducation } from "./challenge-education";

type Views = Record<string, unknown>;
const LearningContext = createContext<{
  views: Views;
  update: (kind: string, value?: unknown) => void;
}>({ views: {}, update: () => {} });
export function ChallengeLearningProvider({
  children,
}: {
  children: ReactNode;
}) {
  const [views, setViews] = useState<Views>({});
  const update = useCallback(
    (kind: string, value?: unknown) =>
      setViews((previous) => {
        const next = { ...previous };
        if (value === undefined) delete next[kind];
        else next[kind] = value;
        return next;
      }),
    [],
  );
  const value = useMemo(() => ({ views, update }), [views, update]);
  return (
    <LearningContext.Provider value={value}>
      {children}
    </LearningContext.Provider>
  );
}

/** Publish only explicit visualization data, never arbitrary page/DOM contents. */
export function useLearningView(kind: string, value: unknown) {
  const { update } = useContext(LearningContext);
  const encoded = JSON.stringify(value);
  useEffect(() => {
    update(kind, JSON.parse(encoded));
    return () => update(kind);
  }, [kind, encoded, update]);
}

type Turn = { question: string; answer: string; complete: boolean };
function Answer({ text }: { text: string }) {
  // Tiny safe prose renderer: no HTML, images, embeds or executable URL schemes.
  const tokens = text.split(
    /(\[[^\]\n]+\]\(https?:\/\/[^\s)]+\)|\*\*[^*]+\*\*|`[^`]+`)/g,
  );
  return (
    <div className="learning-answer">
      {tokens.map((token, i) => {
        const link = token.match(/^\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)$/);
        if (link)
          return (
            <a key={i} href={link[2]} target="_blank" rel="noopener noreferrer">
              {link[1]}
            </a>
          );
        if (token.startsWith("**") && token.endsWith("**"))
          return <strong key={i}>{token.slice(2, -2)}</strong>;
        if (token.startsWith("`") && token.endsWith("`"))
          return <code key={i}>{token.slice(1, -1)}</code>;
        return token;
      })}
    </div>
  );
}

export function ChallengeLearning({
  challenge,
  section,
}: {
  challenge: Challenge;
  section: string;
}) {
  const { views } = useContext(LearningContext);
  const [turns, setTurns] = useState<Turn[]>([]);
  const [question, setQuestion] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [ready, setReady] = useState(false);
  const controller = useRef<AbortController | null>(null);
  const log = useRef<HTMLDivElement>(null);
  const storageKey = `science-ladder-learning:${challenge.versionId}`;
  useEffect(() => {
    try {
      const saved = JSON.parse(sessionStorage.getItem(storageKey) || "[]");
      if (Array.isArray(saved))
        setTurns(
          saved
            .filter(
              (t) =>
                typeof t.question === "string" &&
                t.question.length <= 2000 &&
                typeof t.answer === "string" &&
                t.answer.length <= 16000 &&
                t.complete === true,
            )
            .slice(-20),
        );
    } catch {
      /* Tab storage is optional. */
    }
    setReady(true);
    return () => {
      controller.current?.abort();
      controller.current = null;
    };
  }, [storageKey]);
  useEffect(() => {
    if (ready && !busy)
      try {
        sessionStorage.setItem(
          storageKey,
          JSON.stringify(turns.filter((t) => t.complete).slice(-20)),
        );
      } catch {
        /* No persistence requirement. */
      }
  }, [turns, storageKey, ready, busy]);
  useEffect(() => {
    const element = log.current;
    if (
      element &&
      element.scrollHeight - element.scrollTop - element.clientHeight < 180
    )
      element.scrollTop = element.scrollHeight;
  }, [turns]);

  async function ask(value: string) {
    const text = value.trim();
    if (!text || busy || !ready) return;
    const active = new AbortController();
    controller.current = active;
    setQuestion("");
    setError("");
    setBusy(true);
    let history = turns
      .filter((t) => t.complete)
      .slice(-7)
      .flatMap((t) => [
        { role: "user", content: t.question },
        { role: "assistant", content: t.answer.slice(0, 8000) },
      ]);
    while (history.reduce((n, m) => n + m.content.length, text.length) > 23000)
      history = history.slice(2);
    const next = [
      ...turns,
      { question: text, answer: "", complete: false },
    ].slice(-20);
    setTurns(next);
    let answer = "",
      completed = false;
    try {
      const response = await fetch(
        `/v1/challenges/${encodeURIComponent(challenge.slug)}/ask`,
        {
          method: "POST",
          credentials: "same-origin",
          signal: active.signal,
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({
            versionId: challenge.versionId,
            messages: [...history, { role: "user", content: text }],
            view: {
              section,
              education: (() => {
                const e = challengeEducation(challenge);
                return e
                  ? {
                      frontier: e.frontier.join("\n\n").slice(0, 3500),
                      significance: e.significance.join("\n\n").slice(0, 3500),
                      sources: e.sources.slice(0, 5),
                    }
                  : null;
              })(),
              visualizations: views,
            },
          }),
        },
      );
      if (!response.ok) {
        const body = await response.json().catch(() => null);
        throw new Error(
          body?.error?.message || "The guide is unavailable. Please try again.",
        );
      }
      if (!response.body)
        throw new Error("The guide returned an empty response.");
      const reader = response.body.getReader();
      const decoder = new TextDecoder();
      let pending = "";
      const receive = (block: string) => {
        const line = block
          .split("\n")
          .find((line) => line.startsWith("data: "));
        if (!line) return;
        const event = JSON.parse(line.slice(6));
        if (event.type === "delta") {
          answer += event.text;
          if (controller.current === active)
            setTurns([
              ...next.slice(0, -1),
              { question: text, answer, complete: false },
            ]);
        } else if (event.type === "done") completed = true;
        else if (event.type === "error") throw new Error(event.message);
      };
      while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        pending += decoder.decode(value, { stream: true });
        let end: number;
        while ((end = pending.indexOf("\n\n")) >= 0) {
          receive(pending.slice(0, end));
          pending = pending.slice(end + 2);
        }
      }
      if (!completed || !answer.trim())
        throw new Error(
          "The answer was interrupted. Please retry your question.",
        );
      if (controller.current === active)
        setTurns([
          ...next.slice(0, -1),
          { question: text, answer, complete: true },
        ]);
    } catch (e) {
      if (controller.current === active) {
        setError(
          (e as Error).name === "AbortError"
            ? "Answer stopped."
            : (e as Error).message,
        );
        setQuestion(text);
      }
    } finally {
      active.abort();
      if (controller.current === active) {
        setBusy(false);
        controller.current = null;
      }
    }
  }

  return (
    <section
      className="challenge-learning"
      aria-label="Ask about this challenge"
    >
      <div className="learning-heading">
        <h2>
          <MessageCircle size={19} /> Ask about this challenge
        </h2>
        {turns.length > 0 && (
          <button
            className="text-button"
            onClick={() => {
              controller.current?.abort();
              controller.current = null;
              setBusy(false);
              setTurns([]);
              setError("");
              setQuestion("");
            }}
          >
            Clear conversation
          </button>
        )}
      </div>
      {!turns.length && (
        <div className="learning-suggestions">
          {[
            "Explain this challenge simply",
            "What am I seeing in the visualization?",
            "Why would an improvement matter?",
          ].map((prompt) => (
            <button
              key={prompt}
              type="button"
              disabled={!ready}
              onClick={() => ask(prompt)}
            >
              {prompt}
            </button>
          ))}
        </div>
      )}
      {turns.length > 0 && (
        <div
          className="learning-log"
          ref={log}
          role="log"
          aria-label="Challenge conversation"
          aria-live="polite"
          aria-relevant="additions"
        >
          {turns.map((turn, i) => (
            <div className="learning-turn" key={i}>
              <p className="learning-question">
                <span>You</span>
                {turn.question}
              </p>
              <div className="learning-reply">
                <span>AI guide</span>
                {turn.answer ? (
                  <Answer text={turn.answer} />
                ) : busy && i === turns.length - 1 ? (
                  <p className="learning-thinking">Thinking…</p>
                ) : (
                  <p className="subtle">No answer received.</p>
                )}
                {!turn.complete && turn.answer && !busy && (
                  <small>Partial answer</small>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
      <form
        className="learning-form"
        onSubmit={(e) => {
          e.preventDefault();
          void ask(question);
        }}
      >
        <label className="sr-only" htmlFor="challenge-question">
          Your question about this challenge
        </label>
        <textarea
          id="challenge-question"
          value={question}
          onChange={(e) => setQuestion(e.target.value)}
          maxLength={2000}
          rows={2}
          placeholder="Ask a question…"
          disabled={busy}
          onKeyDown={(e) => {
            if (
              e.key === "Enter" &&
              !e.shiftKey &&
              !e.nativeEvent.isComposing
            ) {
              e.preventDefault();
              void ask(question);
            }
          }}
        />
        {busy ? (
          <button
            type="button"
            className="learning-send"
            aria-label="Stop answer"
            onClick={() => controller.current?.abort()}
          >
            <Square size={17} />
          </button>
        ) : (
          <button
            className="learning-send"
            type="submit"
            disabled={!question.trim() || !ready}
            aria-label="Ask question"
          >
            <ArrowUp size={20} />
          </button>
        )}
      </form>
      {error && (
        <p className="learning-error" role="alert">
          {error}
        </p>
      )}
      <p className="learning-note">
        AI explanations, grounded in this challenge and your current view.
        Conversation saved in this tab; questions and context are sent to
        OpenAI.
      </p>
    </section>
  );
}
