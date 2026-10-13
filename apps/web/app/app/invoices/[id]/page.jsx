"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useState } from "react";
import { api, date, errorMessage, money } from "../../../../lib/api";
import {
  Badge,
  Loading,
  Notice,
  PageHeader,
  useResource,
} from "../../../../components/ui";

export default function InvoiceDetail() {
  const { id } = useParams();
  const [key, refresh] = useState(0),
    [notice, setNotice] = useState(""),
    [payment, setPayment] = useState(false),
    [busy, setBusy] = useState(false),
    [paymentKey, setPaymentKey] = useState(null);
  const result = useResource(`/invoices/${encodeURIComponent(id)}`, key),
    i = result.data;
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
      } else if (type === "whatsapp") {
        const r = await api(`/invoices/${id}/share-link`);
        const number = i.customer_phone.replace(/\D/g, "");
        if (!number) throw new Error("Customer has no phone number");
        const message = `Hello ${i.customer_name}, invoice #${String(i.number).padStart(6, "0")} has ${money(i.balance_minor, i.currency)} outstanding. ${r.url}`;
        window.open(
          `https://wa.me/${number}?text=${encodeURIComponent(message)}`,
          "_blank",
          "noopener,noreferrer",
        );
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
        }),
      });
      setPayment(false);
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
          <section className="panel">
            <div className="public-invoice-top">
              <div>
                <span className="eyebrow">
                  INVOICE #{String(i.number).padStart(6, "0")}
                </span>
                <h2>{i.customer_name}</h2>
                <p>{i.customer_email || i.customer_phone}</p>
              </div>
              <Badge>{i.status}</Badge>
            </div>
            <div className="public-invoice-meta">
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
                  i.balance_minor > 0 &&
                  i.customer_phone && (
                    <button
                      className="button button-light"
                      disabled={busy}
                      onClick={() => act("whatsapp")}
                    >
                      WhatsApp reminder
                    </button>
                  )}
                {i.status !== "void" &&
                  i.status !== "draft" &&
                  i.balance_minor > 0 && (
                    <button
                      className="button button-light"
                      onClick={() => {
                        setPayment(true);
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
            {payment && (
              <section className="panel">
                <h2>Record payment</h2>
                <p>Confirm funds outside LinkPay before recording them here.</p>
                <form onSubmit={record}>
                  <label className="field">
                    Amount ({i.currency})
                    <input
                      name="amount"
                      type="number"
                      min="0.01"
                      max={(i.balance_minor / 100).toFixed(2)}
                      step="0.01"
                      required
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
                    <input name="reference" placeholder="Optional reference" />
                  </label>
                  <div className="form-actions">
                    <button className="button button-dark" disabled={busy}>
                      {busy ? "Saving…" : "Record payment"}
                    </button>
                    <button
                      type="button"
                      className="button button-light"
                      onClick={() => setPayment(false)}
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
