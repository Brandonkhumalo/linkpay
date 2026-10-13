"use client";

import Link from "next/link";
import { useState } from "react";
import { api, errorMessage } from "../lib/api";
import {
  Badge,
  Empty,
  Loading,
  Notice,
  PageHeader,
  Pagination,
  useResource,
} from "./ui";

export default function RemindersClient({ page }) {
  const [key, refresh] = useState(0),
    [notice, setNotice] = useState(""),
    [busy, setBusy] = useState(false);
  const settings = useResource("/reminder-rules", key),
    history = useResource(`/reminder-deliveries?page=${page}`, key),
    subscription = useResource("/subscription");
  async function save(e) {
    e.preventDefault();
    setBusy(true);
    setNotice("");
    const data = new FormData(e.currentTarget);
    const offsets = String(data.get("offsets") || "")
      .split(",")
      .map((x) => x.trim())
      .filter(Boolean)
      .map(Number);
    if (offsets.some((x) => !Number.isInteger(x))) {
      setNotice("Use whole numbers separated by commas.");
      setBusy(false);
      return;
    }
    try {
      await api("/reminder-rules", {
        method: "PUT",
        body: JSON.stringify({ enabled: data.has("enabled"), offsets }),
      });
      refresh((v) => v + 1);
      setNotice("Reminder schedule saved.");
    } catch (error) {
      setNotice(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }
  return (
    <>
      <PageHeader
        eyebrow="STAY ON TOP OF DUES"
        title="Payment reminders"
        description="Follow up with customers while balances are outstanding."
      />
      <Notice
        message={notice}
        kind={notice.includes("saved") ? "success" : "error"}
      />
      <div className="two-col">
        <section className="panel">
          <h2>Automated email</h2>
          <p>
            Available on Starter and Business. Messages are sent only while the
            invoice is outstanding.
          </p>
          {settings.loading || subscription.loading ? (
            <Loading />
          ) : settings.error || subscription.error ? (
            <Notice message={settings.error || subscription.error} />
          ) : (
            <form key={key} onSubmit={save}>
              <label className="field checkbox-field">
                <input
                  type="checkbox"
                  name="enabled"
                  defaultChecked={settings.data.enabled}
                  disabled={subscription.data.plan === "free"}
                />{" "}
                Enable reminders
              </label>
              <label className="field">
                Days relative to due date
                <input
                  name="offsets"
                  defaultValue={settings.data.offsets?.join(", ")}
                  placeholder="-3, 0, 3, 7"
                  disabled={subscription.data.plan === "free"}
                />
                <span className="helper">
                  Negative numbers send before the due date. Up to 8 unique days
                  from −30 to +60.
                </span>
              </label>
              <button
                className="button button-dark"
                disabled={busy || subscription.data.plan === "free"}
              >
                {busy ? "Saving…" : "Save schedule"}
              </button>
              {subscription.data.plan === "free" && (
                <p className="helper" style={{ marginTop: 12 }}>
                  Upgrade to a paid plan to enable automated reminders.{" "}
                  <Link href="/app/billing">See plans →</Link>
                </p>
              )}
            </form>
          )}
        </section>
        <section className="panel" style={{ background: "#eaf2e2" }}>
          <span className="eyebrow">PERSONAL FOLLOW-UP</span>
          <h2>Send a WhatsApp reminder</h2>
          <p>
            Open an issued invoice to copy its link or contact your customer
            directly. Messages are always sent by you.
          </p>
          <Link className="button button-light" href="/app/invoices">
            View invoices →
          </Link>
        </section>
      </div>
      <section className="panel" style={{ marginTop: 16 }}>
        <h2>Delivery history</h2>
        {history.loading ? (
          <Loading />
        ) : history.error ? (
          <Notice message={history.error} />
        ) : history.data.items.length ? (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Invoice</th>
                  <th>Scheduled day</th>
                  <th>Status</th>
                  <th>Attempts</th>
                </tr>
              </thead>
              <tbody>
                {history.data.items.map((d, index) => (
                  <tr key={`${d.invoice_id}-${d.offset_days}-${index}`}>
                    <td>
                      <Link
                        className="table-action"
                        href={`/app/invoices/${d.invoice_id}`}
                      >
                        #{d.invoice_id.slice(0, 8)}…
                      </Link>
                    </td>
                    <td>{d.offset_days}</td>
                    <td>
                      <Badge>{d.status}</Badge>
                    </td>
                    <td>{d.attempts}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <Empty
            title={
              page > 1 ? "No deliveries on this page" : "No reminders sent yet"
            }
            description="Delivery attempts will appear here."
          />
        )}
        <Pagination base="/app/reminders" data={history.data} />
      </section>
    </>
  );
}
