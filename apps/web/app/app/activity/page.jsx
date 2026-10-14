"use client";

import Link from "next/link";
import { useState } from "react";
import {
  api,
  date,
  errorMessage,
  notificationDestination,
} from "../../../lib/api";
import {
  Loading,
  Notice,
  PageHeader,
  useResource,
} from "../../../components/ui";

export default function Activity() {
  const [page, setPage] = useState(1);
  const [key, refresh] = useState(0);
  const [notice, setNotice] = useState("");
  const result = useResource(`/notifications?page=${page}`, key);
  async function markRead(id) {
    try {
      await api(`/notifications/${id}/read`, { method: "POST" });
      window.dispatchEvent(new Event("linkpay:notifications-changed"));
      refresh((value) => value + 1);
    } catch (error) {
      setNotice(errorMessage(error));
    }
  }
  return (
    <>
      <PageHeader
        eyebrow="YOUR WORKSPACE"
        title="Activity"
        description="Enquiries, quote decisions, and customer payment reports in one place."
      />
      <Notice message={notice} />
      <section className="panel">
        {result.loading ? (
          <Loading />
        ) : result.error ? (
          <Notice message={result.error} />
        ) : result.data.items.length ? (
          <div className="stack" style={{ gap: 12 }}>
            {result.data.items.map((item) => (
              <div className="preview-card" key={item.id}>
                <strong>
                  {item.read ? "" : "● "}
                  {item.title}
                </strong>
                <p className="helper">
                  {date(item.created_at)} · Email alert: {item.email_status}
                </p>
                <div className="form-actions">
                  <Link href={notificationDestination(item.kind)}>
                    Review →
                  </Link>
                  {!item.read && (
                    <button onClick={() => markRead(item.id)}>Mark read</button>
                  )}
                </div>
              </div>
            ))}
          </div>
        ) : (
          <p>No activity yet.</p>
        )}
        {result.data && (
          <div className="pagination">
            <button
              disabled={!result.data.has_prev}
              onClick={() => setPage(page - 1)}
            >
              ← Previous
            </button>
            <span>Page {page}</span>
            <button
              disabled={!result.data.has_next}
              onClick={() => setPage(page + 1)}
            >
              Next →
            </button>
          </div>
        )}
      </section>
    </>
  );
}
