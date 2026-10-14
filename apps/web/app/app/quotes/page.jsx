"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";
import { api, date, errorMessage, money } from "../../../lib/api";
import {
  Badge,
  Loading,
  Notice,
  PageHeader,
  useResource,
} from "../../../components/ui";
import { useWorkspace } from "../layout";

export default function Quotes() {
  const router = useRouter();
  const { business } = useWorkspace();
  const [page, setPage] = useState(1);
  const [notice, setNotice] = useState("");
  const result = useResource(`/quotes?page=${page}`);
  async function convert(id) {
    try {
      const invoice = await api(`/quotes/${id}/convert`, { method: "POST" });
      window.dispatchEvent(new Event("linkpay:notifications-changed"));
      router.push(`/app/invoices/${invoice.id}`);
    } catch (error) {
      setNotice(errorMessage(error));
    }
  }
  async function copy(url) {
    try {
      await navigator.clipboard.writeText(url);
      setNotice("Quote link copied.");
    } catch {
      setNotice(`Copy this quote link: ${url}`);
    }
  }
  return (
    <>
      <PageHeader
        eyebrow="AGREED WORK"
        title="Quotes"
        description="Share scope and price for customer approval before invoicing."
        action={
          <Link className="button button-light" href="/app/enquiries">
            View enquiries →
          </Link>
        }
      />
      <Notice
        message={notice}
        kind={notice.includes("copied") ? "success" : "error"}
      />
      <section className="panel">
        {result.loading ? (
          <Loading />
        ) : result.error ? (
          <Notice message={result.error} />
        ) : result.data.items.length ? (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Customer</th>
                  <th>Scope</th>
                  <th>Amount</th>
                  <th>Expires</th>
                  <th>Status</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {result.data.items.map((q) => (
                  <tr key={q.id}>
                    <td>{q.customer_name}</td>
                    <td>{q.description}</td>
                    <td>{money(q.amount_minor, business.currency)}</td>
                    <td>{date(q.expires_at)}</td>
                    <td>
                      <Badge>{q.status}</Badge>
                    </td>
                    <td>
                      <div className="list-actions">
                        <button onClick={() => copy(q.url)}>Copy link</button>
                        {q.status === "accepted" && (
                          <button onClick={() => convert(q.id)}>
                            Create invoice
                          </button>
                        )}
                        {q.invoice_id && (
                          <Link href={`/app/invoices/${q.invoice_id}`}>
                            Open invoice
                          </Link>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <p>No quotes yet. Create one from an enquiry.</p>
        )}
        {result.data && (
          <div className="pagination">
            <button
              className="button button-light small"
              disabled={!result.data.has_prev}
              onClick={() => setPage(page - 1)}
            >
              ← Previous
            </button>
            <span>Page {page}</span>
            <button
              className="button button-light small"
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
