"use client";

import { useState } from "react";
import { api, errorMessage } from "../lib/api";

export default function QuoteResponse({ token, initialStatus }) {
  const [status, setStatus] = useState(initialStatus);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState("");
  async function respond(decision) {
    setBusy(true);
    setNotice("");
    try {
      const result = await api(
        `/public/quotes/${encodeURIComponent(token)}/respond`,
        {
          method: "POST",
          body: JSON.stringify({ decision }),
        },
      );
      setStatus(result.status);
      setNotice(
        decision === "accepted"
          ? "Quote accepted. The business will prepare your invoice."
          : "Quote declined. The business has been notified.",
      );
    } catch (error) {
      setNotice(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }
  return (
    <>
      {notice && <p role="status">{notice}</p>}
      {status === "open" ? (
        <div className="form-actions">
          <button
            className="button button-dark"
            disabled={busy}
            onClick={() => respond("accepted")}
          >
            Accept quote
          </button>
          <button
            className="button button-light"
            disabled={busy}
            onClick={() => respond("declined")}
          >
            Decline
          </button>
        </div>
      ) : (
        <p className="helper">Status: {status.replaceAll("_", " ")}</p>
      )}
    </>
  );
}
