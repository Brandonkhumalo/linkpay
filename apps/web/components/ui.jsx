"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { api, errorMessage } from "../lib/api";

export function Brand({ compact = false }) {
  return (
    <Link className="brand" href="/" aria-label="LinkPay home">
      <span className="brand-symbol">↗</span>
      {!compact && (
        <span>
          linkpay<span className="brand-dot">.</span>
        </span>
      )}
    </Link>
  );
}

export function Icon({ name, size = 20 }) {
  const paths = {
    grid: (
      <>
        <rect x="3" y="3" width="7" height="7" rx="1" />
        <rect x="14" y="3" width="7" height="7" rx="1" />
        <rect x="3" y="14" width="7" height="7" rx="1" />
        <rect x="14" y="14" width="7" height="7" rx="1" />
      </>
    ),
    shop: (
      <>
        <path d="M3 10h18l-1.5-6h-15L3 10Z" />
        <path d="M5 10v10h14V10M9 20v-7h6v7" />
      </>
    ),
    box: (
      <>
        <path d="m3 7 9-4 9 4v10l-9 4-9-4V7Z" />
        <path d="m3 7 9 4 9-4M12 11v10" />
      </>
    ),
    chat: (
      <path d="M20 11.5a8 8 0 0 1-8 8 8.7 8.7 0 0 1-3-.6L4 20l1.1-4.8a8 8 0 1 1 14.9-3.7Z" />
    ),
    users: (
      <>
        <path d="M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2M16 3.3a4 4 0 0 1 0 7.4M22 21v-2a4 4 0 0 0-3-3.9" />
        <circle cx="9" cy="7" r="4" />
      </>
    ),
    file: (
      <>
        <path d="M6 2h8l5 5v15H6a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2Z" />
        <path d="M14 2v6h5M8 13h8M8 17h8" />
      </>
    ),
    bell: (
      <>
        <path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9ZM10 21h4" />
      </>
    ),
    card: (
      <>
        <rect x="2" y="5" width="20" height="14" rx="2" />
        <path d="M2 10h20" />
      </>
    ),
    gear: (
      <>
        <circle cx="12" cy="12" r="3" />
        <path d="M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1-2.8 2.8-.1-.1A1.7 1.7 0 0 0 15 19.4a1.7 1.7 0 0 0-1 1.6V21h-4v-.1a1.7 1.7 0 0 0-1-1.5 1.7 1.7 0 0 0-1.9.3l-.1.1-2.8-2.8.1-.1A1.7 1.7 0 0 0 4.6 15 1.7 1.7 0 0 0 3 14h-.1v-4H3a1.7 1.7 0 0 0 1.6-1 1.7 1.7 0 0 0-.3-1.9l-.1-.1 2.8-2.8.1.1A1.7 1.7 0 0 0 9 4.6a1.7 1.7 0 0 0 1-1.6V3h4v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.9-.3l.1-.1 2.8 2.8-.1.1A1.7 1.7 0 0 0 19.4 9 1.7 1.7 0 0 0 21 10h.1v4H21a1.7 1.7 0 0 0-1.6 1Z" />
      </>
    ),
    arrow: <path d="M5 12h14m-6-6 6 6-6 6" />,
    external: (
      <>
        <path d="M13 5h6v6M19 5 9 15" />
        <path d="M19 13v6H5V5h6" />
      </>
    ),
    plus: <path d="M12 5v14M5 12h14" />,
  };
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {paths[name]}
    </svg>
  );
}

export function Badge({ children }) {
  const status = String(children || "")
    .toLowerCase()
    .replace(/[^a-z_]/g, "");
  return (
    <span className={`badge badge-${status}`}>
      {String(children || "").replaceAll("_", " ")}
    </span>
  );
}

export function PageHeader({ eyebrow, title, description, action }) {
  return (
    <div className="page-heading">
      <div>
        <span className="eyebrow">{eyebrow}</span>
        <h1>{title}</h1>
        {description && <p>{description}</p>}
      </div>
      {action && <div className="page-action">{action}</div>}
    </div>
  );
}

export function Empty({ title, description, action }) {
  return (
    <div className="empty">
      <div className="empty-icon">↗</div>
      <h3>{title}</h3>
      <p>{description}</p>
      {action}
    </div>
  );
}

export function Notice({ message, kind = "error" }) {
  return (
    message && (
      <div className={`notice notice-${kind}`} role="alert">
        {message}
      </div>
    )
  );
}

export function Loading() {
  return (
    <div className="loading-block">
      <span className="spinner" /> Loading your workspace…
    </div>
  );
}

export function useResource(path, refreshKey = 0) {
  const [state, setState] = useState({
    path: "",
    refreshKey: -1,
    data: null,
    loading: true,
    error: "",
  });
  useEffect(() => {
    let active = true;
    api(path)
      .then(
        (data) =>
          active &&
          setState({ path, refreshKey, data, loading: false, error: "" }),
      )
      .catch(
        (error) =>
          active &&
          setState({
            path,
            refreshKey,
            data: null,
            loading: false,
            error: errorMessage(error),
          }),
      );
    return () => {
      active = false;
    };
  }, [path, refreshKey]);
  return state.path === path && state.refreshKey === refreshKey
    ? state
    : { data: null, loading: true, error: "" };
}

export function Pagination({ base, data }) {
  if (!data) return null;
  const url = (page) => `${base}?page=${page}`;
  return (
    <nav className="pagination" aria-label="Pagination">
      <Link
        aria-disabled={!data.has_prev}
        tabIndex={!data.has_prev ? -1 : undefined}
        className={`button button-light ${!data.has_prev ? "disabled" : ""}`}
        href={data.has_prev ? url(data.page - 1) : "#"}
      >
        ← Previous
      </Link>
      <span>
        Page <strong>{data.page}</strong>
      </span>
      <Link
        aria-disabled={!data.has_next}
        tabIndex={!data.has_next ? -1 : undefined}
        className={`button button-light ${!data.has_next ? "disabled" : ""}`}
        href={data.has_next ? url(data.page + 1) : "#"}
      >
        Next →
      </Link>
    </nav>
  );
}
