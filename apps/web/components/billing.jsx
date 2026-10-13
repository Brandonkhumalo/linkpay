"use client";

import { useState } from "react";
import { api, date, errorMessage, money } from "../lib/api";
import {
  Badge,
  Empty,
  Loading,
  Notice,
  PageHeader,
  Pagination,
  useResource,
} from "./ui";
import { useWorkspace } from "../app/app/layout";

export default function BillingClient({ page }) {
  const { me } = useWorkspace(),
    [key, refresh] = useState(0),
    [notice, setNotice] = useState(""),
    [busy, setBusy] = useState(false);
  const subscription = useResource("/subscription", key),
    history = useResource(`/billing/payments?page=${page}`, key);
  async function checkout(plan, months) {
    setBusy(true);
    setNotice("");
    try {
      const result = await api("/billing/checkout", {
        method: "POST",
        body: JSON.stringify({ plan, months }),
      });
      window.location.assign(result.checkout_url);
    } catch (error) {
      setNotice(errorMessage(error));
      setBusy(false);
    }
  }
  async function check(id) {
    setBusy(true);
    setNotice("");
    try {
      await api(`/billing/payments/${id}/refresh`, { method: "POST" });
      refresh((v) => v + 1);
      setNotice("Payment status checked.");
    } catch (error) {
      setNotice(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }
  return (
    <>
      <PageHeader
        eyebrow="YOUR PLAN"
        title="Billing & plans"
        description="Choose the space your business needs. Paid plans renew manually."
      />
      <Notice
        message={notice}
        kind={notice.includes("checked") ? "success" : "error"}
      />
      <section className="panel" style={{ marginBottom: 16 }}>
        <div className="panel-head">
          <h2>Current plan</h2>
          {subscription.data && <Badge>{subscription.data.plan}</Badge>}
        </div>
        {subscription.loading ? (
          <Loading />
        ) : subscription.error ? (
          <Notice message={subscription.error} />
        ) : (
          <>
            <p>
              {subscription.data.plan === "free"
                ? "Your free plan is active."
                : `Your ${subscription.data.plan} plan is active${subscription.data.expires_at ? ` through ${date(subscription.data.expires_at)}` : ""}.`}
            </p>
            <div className="form-grid">
              <div className="usage-row">
                <div>
                  <strong>Listings</strong>
                  <span>
                    {subscription.data.products_used} /{" "}
                    {subscription.data.product_limit}
                  </span>
                </div>
                <div className="usage-track">
                  <span
                    style={{
                      width: `${Math.min(100, (100 * subscription.data.products_used) / subscription.data.product_limit)}%`,
                    }}
                  />
                </div>
              </div>
              <div className="usage-row">
                <div>
                  <strong>Invoices this month</strong>
                  <span>
                    {subscription.data.invoices_used} /{" "}
                    {subscription.data.invoice_monthly_limit}
                  </span>
                </div>
                <div className="usage-track">
                  <span
                    style={{
                      width: `${Math.min(100, (100 * subscription.data.invoices_used) / subscription.data.invoice_monthly_limit)}%`,
                    }}
                  />
                </div>
              </div>
            </div>
          </>
        )}
      </section>
      <div className="plan-options">
        <section className="plan-option">
          <span className="eyebrow">STARTER</span>
          <strong>USD 3 / month</strong>
          <p>30 listings, 30 invoices monthly, 100 reminder emails.</p>
          <div className="stack" style={{ gap: 7 }}>
            <button
              className="button button-dark"
              disabled={busy || me.role !== "owner"}
              onClick={() => checkout("starter", 1)}
            >
              Choose monthly
            </button>
            <button
              className="button button-light"
              disabled={busy || me.role !== "owner"}
              onClick={() => checkout("starter", 12)}
            >
              USD 36 / year
            </button>
          </div>
        </section>
        <section className="plan-option">
          <span className="eyebrow">BUSINESS</span>
          <strong>USD 5 / month</strong>
          <p>100 listings, 100 invoices monthly, 500 reminder emails.</p>
          <div className="stack" style={{ gap: 7 }}>
            <button
              className="button button-dark"
              disabled={busy || me.role !== "owner"}
              onClick={() => checkout("business", 1)}
            >
              Choose monthly
            </button>
            <button
              className="button button-light"
              disabled={busy || me.role !== "owner"}
              onClick={() => checkout("business", 12)}
            >
              USD 60 / year
            </button>
          </div>
        </section>
      </div>
      <p className="helper" style={{ margin: "17px 0 28px" }}>
        Checkout opens on Paynow Zimbabwe. Your plan changes only after the
        payment is verified. Only the business owner can purchase a plan.
      </p>
      <section className="panel">
        <h2>Payment history</h2>
        {history.loading ? (
          <Loading />
        ) : history.error ? (
          <Notice message={history.error} />
        ) : history.data.items.length ? (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Plan</th>
                  <th>Amount</th>
                  <th>Status</th>
                  <th>Created</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {history.data.items.map((p) => (
                  <tr key={p.id}>
                    <td>
                      <strong>{p.plan}</strong>
                      <small className="subline">
                        {p.months} month{p.months === 1 ? "" : "s"}
                      </small>
                    </td>
                    <td>{money(p.amount_minor)}</td>
                    <td>
                      <Badge>{p.status}</Badge>
                    </td>
                    <td>{date(p.created_at)}</td>
                    <td>
                      {["created", "pending"].includes(p.status) && (
                        <button
                          className="button button-light small"
                          disabled={busy}
                          onClick={() => check(p.id)}
                        >
                          Check status
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <Empty
            title={
              page > 1 ? "No payments on this page" : "No plan payments yet"
            }
            description="Choose a paid plan above when you need more capacity."
          />
        )}
        <Pagination base="/app/billing" data={history.data} />
      </section>
    </>
  );
}
