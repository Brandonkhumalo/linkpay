"use client";

import Link from "next/link";
import { useState } from "react";
import { useRouter } from "next/navigation";
import { api, errorMessage } from "../lib/api";
import { Brand, Notice } from "./ui";

const passwordRequirements =
  "Use at least 5 characters with a lowercase letter, uppercase letter, number, and special character (maximum 72 bytes).";

export default function AuthForm({ mode, token, initialPlan = "free" }) {
  const router = useRouter();
  const [error, setError] = useState("");
  const [success, setSuccess] = useState("");
  const [busy, setBusy] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [selectedPlan, setSelectedPlan] = useState(initialPlan);
  const titles = {
    login: "Welcome back",
    register: "Create your account",
    forgot: "Reset your password",
    reset: "Choose a new password",
  };
  const subtitles = {
    login: "Log in to see what is happening with your business.",
    register:
      "Start on Free or try a paid package for exactly 168 hours. No payment details needed.",
    forgot: "Enter your email and we will send a 30-minute reset link.",
    reset: passwordRequirements,
  };
  async function submit(event) {
    event.preventDefault();
    setError("");
    setSuccess("");
    const fields = Object.fromEntries(new FormData(event.currentTarget));
    if (
      (mode === "register" || mode === "reset") &&
      (Array.from(fields.password).length < 5 ||
        new TextEncoder().encode(fields.password).length > 72 ||
        !/[a-z]/.test(fields.password) ||
        !/[A-Z]/.test(fields.password) ||
        !/[0-9]/.test(fields.password) ||
        !/[\p{P}\p{S}]/u.test(fields.password))
    ) {
      setError(passwordRequirements);
      return;
    }
    setBusy(true);
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
            trial_plan: fields.trial_plan,
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
                  <fieldset className="trial-choice">
                    <legend>Choose how to start</legend>
                    <p>
                      Starter and Business trials last exactly 168 hours, then
                      move to Free unless you buy a plan. There is no automatic
                      charge.
                    </p>
                    <div className="trial-options">
                      <label
                        className={`trial-option ${selectedPlan === "free" ? "selected" : ""}`}
                      >
                        <input
                          type="radio"
                          name="trial_plan"
                          value="free"
                          checked={selectedPlan === "free"}
                          onChange={() => setSelectedPlan("free")}
                        />
                        <span>
                          <strong>Free</strong>
                          <small>
                            5 listings · 30 invoices/month · 25 MB images · No
                            trial expiry
                          </small>
                        </span>
                      </label>
                      <label
                        className={`trial-option ${selectedPlan === "starter" ? "selected" : ""}`}
                      >
                        <input
                          type="radio"
                          name="trial_plan"
                          value="starter"
                          checked={selectedPlan === "starter"}
                          onChange={() => setSelectedPlan("starter")}
                        />
                        <span>
                          <strong>Starter</strong>
                          <small>
                            30 listings · 60 invoices/month · 100 reminder
                            emails/month · 150 MB images
                          </small>
                        </span>
                      </label>
                      <label
                        className={`trial-option ${selectedPlan === "business" ? "selected" : ""}`}
                      >
                        <input
                          type="radio"
                          name="trial_plan"
                          value="business"
                          checked={selectedPlan === "business"}
                          onChange={() => setSelectedPlan("business")}
                        />
                        <span>
                          <strong>Business</strong>
                          <small>
                            500 listings · 500 invoices/month · 500 reminder
                            emails/month · 500 MB images
                          </small>
                        </span>
                      </label>
                    </div>
                  </fieldset>
                  <label className="field">
                    Business or creator name
                    <input
                      name="business_name"
                      required
                      minLength={2}
                      maxLength={120}
                      autoComplete="organization"
                      placeholder="Your business or creator name"
                    />
                  </label>
                  <label className="field">
                    Reserve your storefront link
                    <input
                      name="slug"
                      required
                      minLength={3}
                      maxLength={40}
                      pattern="[a-z0-9-]{3,40}"
                      placeholder="your-business"
                    />
                    <span className="helper">
                      Lowercase letters, numbers and hyphens only. Your profile
                      stays private until you publish it.
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
                <div className="field">
                  <label htmlFor="auth-password">Password</label>
                  <div className="password-input">
                    <input
                      id="auth-password"
                      name="password"
                      type={showPassword ? "text" : "password"}
                      required
                      minLength={mode === "login" ? undefined : 5}
                      maxLength={mode === "login" ? undefined : 72}
                      autoComplete={
                        mode === "login" ? "current-password" : "new-password"
                      }
                      placeholder="••••••••••••"
                    />
                    <button
                      type="button"
                      className="password-toggle"
                      onClick={() => setShowPassword((visible) => !visible)}
                      aria-controls="auth-password"
                      aria-pressed={showPassword}
                      aria-label={
                        showPassword ? "Hide password" : "Show password"
                      }
                    >
                      {showPassword ? "Hide" : "Show"}
                    </button>
                  </div>
                  {mode === "register" && (
                    <span className="helper">{passwordRequirements}</span>
                  )}
                </div>
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
                    ? selectedPlan === "free"
                      ? "Create free account →"
                      : "Start 7-day free trial →"
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
              No account yet? <Link href="/register">Try a paid plan free</Link>
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
