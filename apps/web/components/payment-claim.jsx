"use client";

import { useState } from "react";
import { api, errorMessage } from "../lib/api";

export default function PaymentClaim({ token, balanceMinor, currency }) {
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  if (balanceMinor <= 0) return null;
  async function submit(event) {
    event.preventDefault();
    setBusy(true);
    setNotice("");
    const form = new FormData(event.currentTarget);
    try {
      await api(
        `/public/invoices/${encodeURIComponent(token)}/payment-claims`,
        {
          method: "POST",
          body: JSON.stringify({
            amount_minor: Math.round(Number(form.get("amount")) * 100),
            reference: String(form.get("reference") || "").trim(),
          }),
        },
      );
      setNotice(
        "Your payment report was sent to the business for verification. It has not been marked paid yet.",
      );
      event.currentTarget.reset();
    } catch (error) {
      setNotice(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }
  return (
    <section className="panel" style={{ marginTop: 18 }}>
      <h2>Already paid?</h2>
      <p>
        Let the business know so they can check the payment and update this
        invoice.
      </p>
      {notice && <p role="status">{notice}</p>}
      <form onSubmit={submit}>
        <div className="form-grid">
          <label className="field">
            Amount paid ({currency})
            <input
              name="amount"
              type="number"
              min="0.01"
              max={(balanceMinor / 100).toFixed(2)}
              step="0.01"
              required
            />
          </label>
          <label className="field">
            Payment reference (optional)
            <input name="reference" maxLength={120} />
          </label>
        </div>
        <button className="button button-light" disabled={busy}>
          {busy ? "Sending…" : "Report payment for review"}
        </button>
      </form>
    </section>
  );
}
