"use client";

import { useState } from "react";
import { api, date, errorMessage } from "../../../lib/api";
import {
  Loading,
  Notice,
  PageHeader,
  useResource,
} from "../../../components/ui";

export default function Followups() {
  const [key, refresh] = useState(0);
  const [customerPage, setCustomerPage] = useState(1);
  const [page, setPage] = useState(1);
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const followups = useResource(`/followups?page=${page}`, key);
  const customers = useResource(`/customers?page=${customerPage}`);
  async function create(event) {
    event.preventDefault();
    setBusy(true);
    setNotice("");
    try {
      await api("/followups", {
        method: "POST",
        body: JSON.stringify(
          Object.fromEntries(new FormData(event.currentTarget)),
        ),
      });
      event.currentTarget.reset();
      setNotice("Follow-up scheduled.");
      refresh((value) => value + 1);
    } catch (error) {
      setNotice(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }
  async function complete(id) {
    try {
      await api(`/followups/${id}/complete`, { method: "POST" });
      refresh((value) => value + 1);
    } catch (error) {
      setNotice(errorMessage(error));
    }
  }
  return (
    <>
      <PageHeader
        eyebrow="KEEP IN TOUCH"
        title="Customer follow-ups"
        description="A private list of people to contact. LinkPay does not send these messages automatically."
      />
      <Notice
        message={notice}
        kind={notice.includes("scheduled") ? "success" : "error"}
      />
      <div className="page-columns">
        <section className="panel">
          <h2>Upcoming and overdue</h2>
          {followups.loading ? (
            <Loading />
          ) : followups.error ? (
            <Notice message={followups.error} />
          ) : followups.data.items.length ? (
            <div className="table-wrap">
              <table className="data-table">
                <thead>
                  <tr>
                    <th>Customer</th>
                    <th>Due</th>
                    <th>Reason</th>
                    <th>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {followups.data.items.map((item) => (
                    <tr key={item.id}>
                      <td>{item.customer_name}</td>
                      <td>{date(item.due_on)}</td>
                      <td>{item.note}</td>
                      <td>
                        {item.completed_at ? (
                          "Done"
                        ) : (
                          <button
                            className="button button-light small"
                            onClick={() => complete(item.id)}
                          >
                            Mark done
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : (
            <p>No follow-ups due.</p>
          )}
          {followups.data && (
            <div className="pagination">
              <button
                disabled={!followups.data.has_prev}
                onClick={() => setPage(page - 1)}
              >
                ← Previous
              </button>
              <span>Page {page}</span>
              <button
                disabled={!followups.data.has_next}
                onClick={() => setPage(page + 1)}
              >
                Next →
              </button>
            </div>
          )}
        </section>
        <aside className="panel">
          <h2>Schedule a follow-up</h2>
          <form onSubmit={create}>
            <label className="field">
              Customer
              {customers.loading ? (
                <Loading />
              ) : customers.error ? (
                <Notice message={customers.error} />
              ) : (
                <select name="customer_id" required defaultValue="">
                  <option value="" disabled>
                    Choose a customer
                  </option>
                  {customers.data.items.map((customer) => (
                    <option key={customer.id} value={customer.id}>
                      {customer.name}
                    </option>
                  ))}
                </select>
              )}
            </label>
            {customers.data && (
              <div className="pagination">
                <button
                  type="button"
                  disabled={!customers.data.has_prev}
                  onClick={() => setCustomerPage(customerPage - 1)}
                >
                  ← Previous
                </button>
                <span>Customer page {customerPage}</span>
                <button
                  type="button"
                  disabled={!customers.data.has_next}
                  onClick={() => setCustomerPage(customerPage + 1)}
                >
                  Next →
                </button>
              </div>
            )}
            <label className="field">
              Due date
              <input name="due_on" type="date" required />
            </label>
            <label className="field">
              Reason
              <textarea name="note" minLength={2} maxLength={500} required />
            </label>
            <button
              className="button button-dark"
              disabled={busy || customers.loading}
            >
              {busy ? "Saving…" : "Schedule follow-up"}
            </button>
          </form>
        </aside>
      </div>
    </>
  );
}
