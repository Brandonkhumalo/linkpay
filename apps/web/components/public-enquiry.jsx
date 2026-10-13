"use client";

import { useState } from "react";
import { api, errorMessage } from "../lib/api";
import { Notice } from "./ui";

export default function PublicEnquiry({ slug, product }) {
  const [open, setOpen] = useState(false),
    [busy, setBusy] = useState(false),
    [message, setMessage] = useState(""),
    [error, setError] = useState("");
  async function submit(e) {
    e.preventDefault();
    setBusy(true);
    setError("");
    const data = Object.fromEntries(new FormData(e.currentTarget));
    if (!data.email && !data.phone) {
      setError("Add an email address or phone number.");
      setBusy(false);
      return;
    }
    try {
      await api(`/public/stores/${encodeURIComponent(slug)}/enquiries`, {
        method: "POST",
        body: JSON.stringify({ ...data, product_id: product.id }),
      });
      setMessage(
        "Your enquiry was sent. The business can contact you using the details provided.",
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
        Request invoice →
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
              <h2>Ask about {product.name}</h2>
              <button
                className="modal-close"
                aria-label="Close"
                onClick={() => setOpen(false)}
              >
                ×
              </button>
            </div>
            <p className="muted">
              Share a little about what you need. The business will follow up
              directly.
            </p>
            <Notice message={error} />
            <form onSubmit={submit}>
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
                Message
                <textarea
                  name="message"
                  required
                  minLength={5}
                  maxLength={3000}
                  defaultValue={`I'm interested in ${product.name}.`}
                />
              </label>
              <button className="button button-dark" disabled={busy}>
                {busy ? "Sending…" : "Send enquiry"}
              </button>
            </form>
          </section>
        </div>
      )}
    </>
  );
}
