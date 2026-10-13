"use client";

import Link from "next/link";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { api, errorMessage } from "../lib/api";
import { Brand, Notice } from "./ui";

export default function AuthForm({ mode, token }) {
  const router = useRouter();
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [busy, setBusy] = useState(false);
  const titles = {
    login: "Welcome back",
    register: "Create your account",
    forgot: "Reset your password",
    reset: "Choose a new password",
  };
  const subtitles = {
    login: "Log in to see what is happening with your business.",
    register: "Your storefront, customers, and invoices start here.",
    forgot: "Enter your email and we will send a 30-minute reset link.",
    reset: "Use a password of at least 12 characters.",
  };
  async function submit(event) {
    event.preventDefault();
    setBusy(true);
    setError("");
    setSuccess("");
    const fields = Object.fromEntries(new FormData(event.currentTarget));
    try {
      if (mode === "login") {
        await api("/auth/login", {
          method: "POST",
          body: JSON.stringify({
            email: fields.email,
            password: fields.password,
          }),
        });
        const next = new URLSearchParams(window.location.search).get("next");
        router.replace(
          next === "/app" || next?.startsWith("/app/") ? next : "/app",
        );
        router.refresh();
      } else if (mode === "register") {
        await api("/auth/register", {
          method: "POST",
          body: JSON.stringify({
            email: fields.email,
            password: fields.password,
            business_name: fields.business_name,
            slug: fields.slug,
          }),
        });
        router.replace("/app");
        router.refresh();
      } else if (mode === "forgot") {
        const result = await api("/auth/password-reset/request", {
          method: "POST",
          body: JSON.stringify({ email: fields.email }),
        });
        setSuccess(result.message);
        event.currentTarget.reset();
      } else {
        const result = await api("/auth/password-reset/confirm", {
          method: "POST",
          body: JSON.stringify({ token, password: fields.password }),
        });
        setSuccess(result.message);
      }
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="auth-page">
      <aside className="auth-side">
        <Brand />
        <div>
          <span className="eyebrow">THE BUSINESS TOOL THAT MOVES WITH YOU</span>
          <h2>
            Everything you need.
            <br />
            <em>All in one link.</em>
          </h2>
          <p>
            Make a great first impression, build customer relationships, and
            stay on top of every invoice.
          </p>
          <div className="auth-side-art">
            <span />
            <span />
            <span />
          </div>
        </div>
        <small>
          © {new Date().getFullYear()} LinkPay · Made for businesses that keep
          moving.
        </small>
      </aside>
      <main className="auth-main">
        <div className="auth-top-link">
          {mode === "login" ? (
            <>
              New here? <Link href="/register">Create account</Link>
            </>
          ) : (
            <>
              <Link href="/login">Back to login</Link>
            </>
          )}
        </div>
        <section className="auth-card">
          <span className="eyebrow">
            {mode === "register"
              ? "LET'S GET STARTED"
              : mode === "login"
                ? "WELCOME BACK"
                : "ACCOUNT RECOVERY"}
          </span>
          <h1>{titles[mode]}</h1>
          <p>{subtitles[mode]}</p>
          <Notice message={error} />
          <Notice message={success} kind="success" />
          {!(mode === "reset" && success) && (
            <form onSubmit={submit}>
              {mode === "register" && (
                <>
                  <label className="field">
                    Business name
                    <input
                      name="business_name"
                      required
                      minLength={2}
                      maxLength={120}
                      autoComplete="organization"
                      placeholder="Your business name"
                    />
                  </label>
                  <label className="field">
                    Storefront link
                    <input
                      name="slug"
                      required
                      minLength={3}
                      maxLength={40}
                      pattern="[a-z0-9-]{3,40}"
                      placeholder="your-business"
                    />
                    <span className="helper">
                      Lowercase letters, numbers and hyphens only.
                    </span>
                  </label>
                </>
              )}
              {mode !== "reset" && (
                <label className="field">
                  Email address
                  <input
                    name="email"
                    type="email"
                    required
                    autoComplete="email"
                    placeholder="you@business.com"
                  />
                </label>
              )}
              {(mode === "login" ||
                mode === "register" ||
                mode === "reset") && (
                <label className="field">
                  Password
                  <input
                    name="password"
                    type="password"
                    required
                    minLength={mode === "login" ? undefined : 12}
                    maxLength={mode === "login" ? undefined : 72}
                    autoComplete={
                      mode === "login" ? "current-password" : "new-password"
                    }
                    placeholder="••••••••••••"
                  />
                </label>
              )}
              {mode === "login" && (
                <p style={{ textAlign: "right", fontSize: 12 }}>
                  <Link href="/forgot-password">Forgot password?</Link>
                </p>
              )}
              <button className="button button-dark" disabled={busy}>
                {busy
                  ? "Please wait…"
                  : mode === "register"
                    ? "Create free account →"
                    : mode === "login"
                      ? "Log in →"
                      : mode === "forgot"
                        ? "Send reset link →"
                        : "Update password →"}
              </button>
            </form>
          )}
          {mode === "login" && (
            <div className="auth-footer">
              No account yet? <Link href="/register">Get started free</Link>
            </div>
          )}
          {mode === "reset" && success && (
            <Link className="button button-dark" href="/login">
              Go to login
            </Link>
          )}
        </section>
      </main>
    </div>
  );
}
