"use client";

import { useState } from "react";
import { api, errorMessage, money } from "../lib/api";
import { Notice } from "./ui";

export default function PublicEnquiry({
  slug,
  product,
  creator = false,
  currency = "USD",
}) {
  const [open, setOpen] = useState(false),
    [busy, setBusy] = useState(false),
    [message, setMessage] = useState(""),
    [error, setError] = useState("");
  async function submit(e) {
    e.preventDefault();
    setBusy(true);
    setError("");
    const data = Object.fromEntries(new FormData(e.currentTarget));
    if (creator) {
      data.budget_minor = data.budget
        ? Math.round(Number(data.budget) * 100)
        : null;
      delete data.budget;
    }
    if (!data.email && !data.phone) {
      setError("Add an email address or phone number.");
      setBusy(false);
      return;
    }
    try {
      await api(`/public/stores/${encodeURIComponent(slug)}/enquiries`, {
        method: "POST",
        body: JSON.stringify({
          ...data,
          ...(product?.id ? { product_id: product.id } : {}),
        }),
      });
      setMessage(
        creator
          ? "Your brief was sent. The creator can contact you using the details provided."
          : "Your enquiry was sent. The business can contact you using the details provided.",
      );
      setOpen(false);
    } catch (error) {
      setError(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }
  return (
    <>
      <button
        className="button button-dark small"
        onClick={() => {
          setOpen(true);
          setMessage("");
        }}
      >
        {creator
          ? "Send partnership brief →"
          : product?.price_on_request
            ? "Request a quote →"
            : "Request invoice →"}
      </button>
      {message && (
        <div className="notice notice-success" role="status">
          {message}
        </div>
      )}
      {open && (
        <div
          className="modal-overlay"
          onMouseDown={(e) => e.target === e.currentTarget && setOpen(false)}
        >
          <section
            className="modal-card"
            role="dialog"
            aria-modal="true"
            aria-label="Send an enquiry"
          >
            <div className="modal-head">
              <h2>
                {creator
                  ? "Propose a collaboration"
                  : product?.price_on_request
                    ? `Request a quote for ${product.name}`
                    : `Request an invoice for ${product.name}`}
              </h2>
              <button
                className="modal-close"
                aria-label="Close"
                onClick={() => setOpen(false)}
              >
                ×
              </button>
            </div>
            <p className="muted">
              {product && !product.price_on_request
                ? `Listed price: ${money(product.price_minor, currency)}. `
                : ""}
              Share a little about what you need. The business will follow up
              directly.
            </p>
            <Notice message={error} />
            <form onSubmit={submit}>
              {creator && (
                <>
                  <label className="field">
                    Brand or agency name
                    <input
                      name="brand_name"
                      required
                      minLength={2}
                      maxLength={160}
                    />
                  </label>
                  <label className="field">
                    Deliverables
                    <textarea
                      name="deliverables"
                      required
                      minLength={5}
                      maxLength={2000}
                      placeholder="For example, two short videos and one story"
                    />
                  </label>
                  <div className="form-grid">
                    <label className="field">
                      Budget ({currency}, optional)
                      <input
                        name="budget"
                        type="number"
                        min="0"
                        max="1000000000"
                        step="0.01"
                      />
                    </label>
                    <label className="field">
                      Target date (optional)
                      <input name="campaign_deadline" type="date" />
                    </label>
                  </div>
                  <label className="field">
                    Content usage rights (optional)
                    <textarea
                      name="usage_rights"
                      maxLength={1000}
                      placeholder="Where and for how long will the brand use the content?"
                    />
                  </label>
                </>
              )}
              <label className="field">
                Your name
                <input name="name" required minLength={2} maxLength={160} />
              </label>
              <div className="form-grid">
                <label className="field">
                  Email
                  <input name="email" type="email" maxLength={254} />
                </label>
                <label className="field">
                  Phone
                  <input name="phone" maxLength={40} />
                </label>
              </div>
              <label className="field">
                {creator ? "Campaign details" : "Message"}
                <textarea
                  name="message"
                  required
                  minLength={5}
                  maxLength={3000}
                  defaultValue={
                    product?.name
                      ? `I'm interested in ${product.name}.`
                      : "We'd like to discuss a collaboration."
                  }
                />
              </label>
              <button className="button button-dark" disabled={busy}>
                {busy ? "Sending…" : creator ? "Send brief" : "Send enquiry"}
              </button>
            </form>
          </section>
        </div>
      )}
    </>
  );
}
