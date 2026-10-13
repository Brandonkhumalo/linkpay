"use client";

import Link from "next/link";
import { useState } from "react";
import { Brand, Notice } from "../../components/ui";

export default function Admin() {
  const [results, setResults] = useState(null),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  async function submit(e) {
    e.preventDefault();
    setBusy(true);
    setError("");
    const form = e.currentTarget;
    const code = new FormData(form).get("code");
    try {
      const response = await fetch("/api/v1/admin/overview", {
        credentials: "same-origin",
        headers: { "X-Admin-Code": String(code) },
      });
      const data = await response.json();
      if (!response.ok) throw new Error(data.error || "Admin access denied");
      setResults(data);
      form.reset();
    } catch (error) {
      setError(error.message);
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="auth-plain">
      <header className="public-header">
        <Brand />
        <Link href="/app">Back to workspace →</Link>
      </header>
      <section className="auth-card">
        <span className="eyebrow">PLATFORM OPERATIONS</span>
        <h1>Admin monitoring</h1>
        <p>Enter the current code from your authenticator app.</p>
        <Notice message={error} />
        <form onSubmit={submit}>
          <label className="field">
            Authenticator code
            <input
              name="code"
              inputMode="numeric"
              pattern="[0-9]{6}"
              maxLength={6}
              required
              autoComplete="one-time-code"
            />
          </label>
          <button className="button button-dark" disabled={busy}>
            {busy ? "Checking…" : "View platform status"}
          </button>
        </form>
      </section>
      {results && (
        <div
          className="stats-grid"
          style={{ maxWidth: 850, margin: "30px auto", padding: "0 20px" }}
        >
          {Object.entries(results).map(([key, value]) => (
            <div className="stat-card" key={key}>
              <small>{key.replaceAll("_", " ")}</small>
              <strong>{Number(value)}</strong>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
