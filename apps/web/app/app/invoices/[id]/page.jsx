"use client";

import Link from "next/link";
import Image from "next/image";
import { useParams, useRouter } from "next/navigation";
import { useState } from "react";
import { api, date, errorMessage, money } from "../../../../lib/api";
import { invoiceTemplates } from "../../../../lib/invoice-design";
import {
  Badge,
  Loading,
  Notice,
  PageHeader,
  useResource,
} from "../../../../components/ui";

function WhatsappInvoiceActions({ id, invoice }) {
  const link = useResource(`/invoices/${encodeURIComponent(id)}/share-link`);
  if (link.loading || link.error || !link.data?.url) return null;
  const number = invoice.customer_phone.replace(/\D/g, "");
  if (!number) return null;
  const numberLabel = String(invoice.number).padStart(6, "0");
  const share = `Hello ${invoice.customer_name}, here is invoice #${numberLabel} for ${money(invoice.total_minor, invoice.currency)}. View it here: ${link.data.url}`;
  const reminder = `Hello ${invoice.customer_name}, invoice #${numberLabel} has ${money(invoice.balance_minor, invoice.currency)} outstanding. ${link.data.url}`;
  return (
    <>
      <a
        className="button button-light"
        href={`https://wa.me/${number}?text=${encodeURIComponent(share)}`}
        target="_blank"
        rel="noopener noreferrer"
      >
        Share on WhatsApp ↗
      </a>
      {invoice.balance_minor > 0 && (
        <a
          className="button button-light"
          href={`https://wa.me/${number}?text=${encodeURIComponent(reminder)}`}
          target="_blank"
          rel="noopener noreferrer"
        >
          WhatsApp reminder ↗
        </a>
      )}
    </>
  );
}

export default function InvoiceDetail() {
  const router = useRouter();
  const { id } = useParams();
  const [key, refresh] = useState(0),
    [notice, setNotice] = useState(""),
    [payment, setPayment] = useState(false),
    [selectedClaim, setSelectedClaim] = useState(null),
    [claimPage, setClaimPage] = useState(1),
    [busy, setBusy] = useState(false),
    [paymentKey, setPaymentKey] = useState(null);
  const result = useResource(`/invoices/${encodeURIComponent(id)}`, key),
    i = result.data;
  const claims = useResource(
    `/invoices/${encodeURIComponent(id)}/payment-claims?page=${claimPage}`,
    key,
  );
  const scheduleResource = useResource(
    `/invoices/${encodeURIComponent(id)}/schedule`,
    key,
  );
  const schedule = scheduleResource.data;
  async function duplicate(event) {
    event.preventDefault();
    setBusy(true);
    try {
      const form = new FormData(event.currentTarget);
      const created = await api(`/invoices/${id}/duplicate`, {
        method: "POST",
        body: JSON.stringify({ due_at: form.get("due_at") }),
      });
      router.push(`/app/invoices/${created.id}`);
    } catch (error) {
      setNotice(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }
  async function saveSchedule(event) {
    event.preventDefault();
    setBusy(true);
    try {
      const form = new FormData(event.currentTarget);
      await api(`/invoices/${id}/schedule`, {
        method: "POST",
        body: JSON.stringify({
          interval_months: Number(form.get("interval_months")),
          next_run_on: form.get("next_run_on"),
          active: form.has("active"),
        }),
      });
      setNotice("Recurring draft schedule saved.");
      refresh((value) => value + 1);
    } catch (error) {
      setNotice(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }
  async function dismissClaim(claimID) {
    try {
      await api(`/invoices/${id}/payment-claims/${claimID}/dismiss`, {
        method: "POST",
      });
      refresh((value) => value + 1);
    } catch (error) {
      setNotice(errorMessage(error));
    }
  }
  async function act(type) {
    setNotice("");
    setBusy(true);
    try {
      if (
        type === "void" &&
        !window.confirm("Void this invoice and revoke its public link?")
      )
        return;
      if (type === "send") {
        const r = await api(`/invoices/${id}/send`, { method: "POST" });
        await navigator.clipboard.writeText(r.url).catch(() => {});
        setNotice(`Invoice issued. Share link: ${r.url}`);
      } else if (type === "copy") {
        const r = await api(`/invoices/${id}/share-link`);
        await navigator.clipboard.writeText(r.url);
        setNotice("Invoice link copied.");
      } else if (type === "void") {
        await api(`/invoices/${id}/void`, { method: "POST" });
        setNotice("Invoice voided.");
      }
      refresh((v) => v + 1);
    } catch (e) {
      setNotice(errorMessage(e));
    } finally {
      setBusy(false);
    }
  }
  async function record(e) {
    e.preventDefault();
    setBusy(true);
    setNotice("");
    const data = new FormData(e.currentTarget);
    const key = paymentKey || crypto.randomUUID();
    setPaymentKey(key);
    try {
      await api(`/invoices/${id}/payments`, {
        method: "POST",
        body: JSON.stringify({
          amount_minor: Math.round(Number(data.get("amount")) * 100),
          method: data.get("method"),
          reference: data.get("reference"),
          idempotency_key: key,
          claim_id: selectedClaim?.id || "",
        }),
      });
      setPayment(false);
      setSelectedClaim(null);
      setPaymentKey(null);
      setNotice("Payment recorded.");
      refresh((v) => v + 1);
    } catch (e) {
      setNotice(errorMessage(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <>
      <PageHeader
        eyebrow="INVOICE DETAILS"
        title={i ? `Invoice #${String(i.number).padStart(6, "0")}` : "Invoice"}
        description="Review its status, balance, and shareable link."
        action={
          <Link className="button button-light" href="/app/invoices">
            ← Back to invoices
          </Link>
        }
      />
      <Notice
        message={notice}
        kind={
          notice.startsWith("Invoice") || notice.startsWith("Payment")
            ? "success"
            : "error"
        }
      />
      {result.loading ? (
        <Loading />
      ) : result.error ? (
        <Notice message={result.error} />
      ) : (
        <div className="page-columns">
          <section
            className={`panel invoice-sheet invoice-sheet--${i.design_template}`}
            style={{ "--invoice-accent": i.design_color }}
          >
            <div className="invoice-sheet-heading">
              <div className="invoice-sheet-identity">
                {i.show_logo && (
                  <Image
                    unoptimized
                    width={76}
                    height={76}
                    className="invoice-detail-logo"
                    src={`/api/v1/invoices/${encodeURIComponent(id)}/logo`}
                    alt={`${i.sender_business_name} logo`}
                  />
                )}
                <div>
                  <span className="eyebrow">FROM</span>
                  <strong>{i.sender_business_name}</strong>
                  <small>{i.sender_phone}</small>
                </div>
              </div>
              <div className="invoice-sheet-title">
                <span className="eyebrow">
                  {invoiceTemplates.find((item) => item.id === i.design_template)
                    ?.name}{" "}
                  invoice
                </span>
                <h1>#{String(i.number).padStart(6, "0")}</h1>
                <Badge>{i.status}</Badge>
              </div>
            </div>
            <div className="public-invoice-meta">
              <div>
                <small>Billed to</small>
                <strong>{i.customer_name}</strong>
                {i.customer_email && <p>{i.customer_email}</p>}
                {i.customer_phone && <p>{i.customer_phone}</p>}
              </div>
              <div>
                <small>Issued</small>
                <strong>{date(i.issued_at)}</strong>
              </div>
              <div>
                <small>Due</small>
                <strong>{date(i.due_at)}</strong>
              </div>
            </div>
            <div className="table-wrap">
              <table className="data-table">
                <thead>
                  <tr>
                    <th>Description</th>
                    <th>Qty</th>
                    <th>Unit price</th>
                    <th>Amount</th>
                  </tr>
                </thead>
                <tbody>
                  {i.items?.map((item, index) => (
                    <tr key={index}>
                      <td>
                        <strong>{item.description}</strong>
                      </td>
                      <td>{item.quantity}</td>
                      <td>{money(item.unit_minor, i.currency)}</td>
                      <td>{money(item.total_minor, i.currency)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="invoice-totals">
              <div className="invoice-line">
                <span>Subtotal</span>
                <span>{money(i.subtotal_minor, i.currency)}</span>
              </div>
              <div className="invoice-line">
                <span>Discount</span>
                <span>− {money(i.discount_minor, i.currency)}</span>
              </div>
              <div className="invoice-line">
                <span>Tax</span>
                <span>{money(i.tax_minor, i.currency)}</span>
              </div>
              <div className="invoice-line">
                <span>Paid</span>
                <span>− {money(i.paid_minor, i.currency)}</span>
              </div>
              <div className="invoice-line total">
                <span>Balance due</span>
                <span>{money(i.balance_minor, i.currency)}</span>
              </div>
            </div>
            {i.notes && (
              <div className="preview-card">
                <strong>Notes</strong>
                <p>{i.notes}</p>
              </div>
            )}
          </section>
          <aside className="stack">
            <section className="panel">
              <h2>Actions</h2>
              <p>Share the invoice and keep its payment status up to date.</p>
              <div className="stack" style={{ gap: 9 }}>
                {i.status === "draft" ? (
                  <button
                    className="button button-dark"
                    disabled={busy}
                    onClick={() => act("send")}
                  >
                    Issue and copy link →
                  </button>
                ) : (
                  i.status !== "void" && (
                    <button
                      className="button button-dark"
                      disabled={busy}
                      onClick={() => act("copy")}
                    >
                      Copy invoice link
                    </button>
                  )
                )}
                {i.status !== "void" && (
                  <a
                    className="button button-light"
                    href={`/api/v1/invoices/${id}/pdf`}
                    target="_blank"
                    rel="noopener noreferrer"
                  >
                    Download PDF ↗
                  </a>
                )}
                {i.status !== "void" &&
                  i.status !== "draft" &&
                  i.customer_phone && (
                    <WhatsappInvoiceActions id={id} invoice={i} />
                  )}
                {i.status !== "void" &&
                  i.status !== "draft" &&
                  i.balance_minor > 0 && (
                    <button
                      className="button button-light"
                      onClick={() => {
                        setPayment(true);
                        setSelectedClaim(null);
                        setPaymentKey(crypto.randomUUID());
                      }}
                    >
                      Record payment
                    </button>
                  )}
                {i.paid_minor === 0 && i.status !== "void" && (
                  <button
                    className="button button-danger"
                    disabled={busy}
                    onClick={() => act("void")}
                  >
                    Void invoice
                  </button>
                )}
              </div>
            </section>
            <section className="panel">
              <h2>Use this invoice again</h2>
              <form onSubmit={duplicate}>
                <label className="field">
                  New due date
                  <input name="due_at" type="date" required />
                </label>
                <button className="button button-light" disabled={busy}>
                  Create duplicate draft
                </button>
              </form>
            </section>
            {i.status !== "void" && (
              <section className="panel">
                <h2>Recurring drafts</h2>
                <p>
                  LinkPay creates a draft for your review on each scheduled
                  date. It never sends it automatically.
                </p>
                <form key={schedule?.id || "new"} onSubmit={saveSchedule}>
                  <label className="field">
                    Every
                    <select
                      name="interval_months"
                      defaultValue={schedule?.interval_months || 1}
                    >
                      <option value="1">Month</option>
                      <option value="3">3 months</option>
                      <option value="6">6 months</option>
                      <option value="12">Year</option>
                    </select>
                  </label>
                  <label className="field">
                    Next draft date
                    <input
                      name="next_run_on"
                      type="date"
                      required
                      defaultValue={schedule?.next_run_on || ""}
                    />
                  </label>
                  <label className="field checkbox-field">
                    <input
                      name="active"
                      type="checkbox"
                      defaultChecked={schedule?.active !== false}
                    />{" "}
                    Active
                  </label>
                  {schedule?.last_error && (
                    <p className="helper">
                      Needs attention: {schedule.last_error}
                    </p>
                  )}
                  <button className="button button-light" disabled={busy}>
                    Save schedule
                  </button>
                </form>
              </section>
            )}
            <section className="panel">
              <h2>Customer payment reports</h2>
              <p>
                Reports from the invoice link are unverified. Check your account
                before recording a payment.
              </p>
              {claims.loading ? (
                <Loading />
              ) : claims.error ? (
                <Notice message={claims.error} />
              ) : claims.data.items.length ? (
                claims.data.items.map((claim) => (
                  <div className="preview-card" key={claim.id}>
                    <strong>
                      {money(claim.amount_minor, i.currency)} · {claim.status}
                    </strong>
                    <p>{claim.reference || "No reference supplied"}</p>
                    {claim.status === "pending" && (
                      <div className="form-actions">
                        <button
                          onClick={() => {
                            setSelectedClaim(claim);
                            setPayment(true);
                            setPaymentKey(crypto.randomUUID());
                          }}
                        >
                          Record verified payment
                        </button>
                        <button onClick={() => dismissClaim(claim.id)}>
                          Dismiss report
                        </button>
                      </div>
                    )}
                  </div>
                ))
              ) : (
                <p>No payment reports.</p>
              )}
              {claims.data && (
                <div className="pagination">
                  <button
                    disabled={!claims.data.has_prev}
                    onClick={() => setClaimPage(claimPage - 1)}
                  >
                    ← Previous
                  </button>
                  <span>Page {claimPage}</span>
                  <button
                    disabled={!claims.data.has_next}
                    onClick={() => setClaimPage(claimPage + 1)}
                  >
                    Next →
                  </button>
                </div>
              )}
            </section>
            {payment && (
              <section className="panel">
                <h2>Record payment</h2>
                <p>Confirm funds outside LinkPay before recording them here.</p>
                <form key={selectedClaim?.id || "manual"} onSubmit={record}>
                  <label className="field">
                    Amount ({i.currency})
                    <input
                      name="amount"
                      type="number"
                      min="0.01"
                      max={(i.balance_minor / 100).toFixed(2)}
                      step="0.01"
                      required
                      defaultValue={
                        selectedClaim
                          ? (selectedClaim.amount_minor / 100).toFixed(2)
                          : ""
                      }
                    />
                  </label>
                  <label className="field">
                    Method
                    <input
                      name="method"
                      required
                      placeholder="Bank, cash, mobile money"
                    />
                  </label>
                  <label className="field">
                    Reference
                    <input
                      name="reference"
                      placeholder="Optional reference"
                      defaultValue={selectedClaim?.reference || ""}
                    />
                  </label>
                  <div className="form-actions">
                    <button className="button button-dark" disabled={busy}>
                      {busy ? "Saving…" : "Record payment"}
                    </button>
                    <button
                      type="button"
                      className="button button-light"
                      onClick={() => {
                        setPayment(false);
                        setSelectedClaim(null);
                      }}
                    >
                      Cancel
                    </button>
                  </div>
                </form>
              </section>
            )}
          </aside>
        </div>
      )}
    </>
  );
}
