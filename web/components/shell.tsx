"use client";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { ArrowUpRight, BookOpen, Github, Plus, Telescope } from "lucide-react";
import { createContext, useContext } from "react";
import { useResource } from "@/lib/api";
import type { Session } from "@/lib/types";
const SessionContext = createContext<ReturnType<
  typeof useResource<Session>
> | null>(null);
export function useSession() {
  const session = useContext(SessionContext);
  if (!session) throw new Error("Session provider missing");
  return session;
}
export function Brand() {
  return (
    <Link href="/" className="brand" aria-label="Science Ladder home">
      <span className="brand-mark" aria-hidden="true">
        <i />
        <i />
        <i />
      </span>
      <span>
        science<span className="brand-light">ladder</span>
      </span>
    </Link>
  );
}
export function Shell({ children }: { children: React.ReactNode }) {
  const path = usePathname();
  const session = useResource<Session>("/me");
  return (
    <SessionContext.Provider value={session}>
      <a href="#main" className="skip-link">
        Skip to content
      </a>
      <header className="site-header">
        <div className="header-inner">
          <Brand />
          <nav aria-label="Main navigation">
            <Link
              href="/"
              className={
                path === "/" || path.startsWith("/challenges") ? "active" : ""
              }
            >
              <Telescope size={16} />
              Explore
            </Link>
            <Link
              href="/docs"
              className={path.startsWith("/docs") ? "active" : ""}
            >
              <BookOpen size={16} />
              Docs
            </Link>
            {session.data?.capabilities.review && (
              <Link
                href="/review"
                className={path === "/review" ? "active" : ""}
              >
                Review
              </Link>
            )}
          </nav>
          <div className="header-actions">
            <Link href="/create" className="button small ghost">
              <Plus size={15} />
              <span>Create challenge</span>
            </Link>
            <Link
              href="/account"
              className="account-link"
              aria-label={
                session.data?.user
                  ? `Account: ${session.data.user.login}`
                  : "Sign in"
              }
            >
              <Github size={17} />
              <span>{session.data?.user?.login || "Sign in"}</span>
            </Link>
          </div>
        </div>
      </header>
      <main id="main">{children}</main>
      <footer className="site-footer">
        <Brand />
        <div>
          <Link href="/docs#trust">Verification</Link>
          <a
            href="https://github.com/matbalez/science-ladder"
            target="_blank"
            rel="noreferrer"
          >
            GitHub <ArrowUpRight size={13} />
          </a>
        </div>
      </footer>
    </SessionContext.Provider>
  );
}
