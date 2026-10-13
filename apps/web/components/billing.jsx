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
  async function checkout(purchase) {
    setBusy(true);
    setNotice("");
    try {
      const result = await api("/billing/checkout", {
        method: "POST",
        body: JSON.stringify(purchase),
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
          {subscription.data && (
            <Badge>
              {subscription.data.trial_active
                ? `${subscription.data.plan} trial`
                : subscription.data.plan}
            </Badge>
          )}
        </div>
        {subscription.loading ? (
          <Loading />
        ) : subscription.error ? (
          <Notice message={subscription.error} />
        ) : (
          <>
            <p>
              {subscription.data.trial_active
                ? `Your ${subscription.data.trial_plan} trial ends ${new Date(subscription.data.trial_expires_at).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" })}. Your account moves to Free automatically unless you buy a plan.`
                : subscription.data.plan === "free"
                  ? subscription.data.trial_expires_at
                    ? "Your trial has ended. Your Free plan is active."
                    : "Your Free plan is active."
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
                  <strong>Plan invoices this month</strong>
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
              <div className="usage-row">
                <div>
                  <strong>Purchased invoices available</strong>
                  <span>{subscription.data.invoice_topup_balance}</span>
                </div>
                {subscription.data.invoice_topup_debt > 0 && (
                  <p className="helper">
                    {subscription.data.invoice_topup_debt} previously used
                    invoices were refunded. Future top-ups cover those first.
                  </p>
                )}
              </div>
            </div>
          </>
        )}
      </section>
      <div className="plan-options">
        <section className="plan-option">
          <span className="eyebrow">STARTER</span>
          <strong>USD 3 / month</strong>
          <p>30 listings, 60 invoices monthly, 100 reminder emails.</p>
          <p>150 MB image storage. USD 36 per year.</p>
          {subscription.data?.trial_active &&
            subscription.data.plan === "business" && (
              <p>
                Buying Starter during your Business trial changes your limits to
                Starter after payment is verified. The paid term extends from
                the trial end.
              </p>
            )}
          <div className="stack" style={{ gap: 7 }}>
            <button
              className="button button-dark"
              disabled={busy || me.role !== "owner"}
              onClick={() => checkout({ plan: "starter", months: 1 })}
            >
              Choose monthly
            </button>
            <button
              className="button button-light"
              disabled={busy || me.role !== "owner"}
              onClick={() => checkout({ plan: "starter", months: 12 })}
            >
              USD 36 / year
            </button>
          </div>
        </section>
        <section className="plan-option">
          <span className="eyebrow">BUSINESS</span>
          <strong>USD 5 / month</strong>
          <p>500 listings, 500 invoices monthly, 500 reminder emails.</p>
          <p>500 MB image storage. USD 60 per year.</p>
          <div className="stack" style={{ gap: 7 }}>
            <button
              className="button button-dark"
              disabled={busy || me.role !== "owner"}
              onClick={() => checkout({ plan: "business", months: 1 })}
            >
              Choose monthly
            </button>
            <button
              className="button button-light"
              disabled={busy || me.role !== "owner"}
              onClick={() => checkout({ plan: "business", months: 12 })}
            >
              USD 60 / year
            </button>
          </div>
        </section>
      </div>
      <section className="panel" style={{ marginTop: 16 }}>
        <h2>Extra invoices</h2>
        <p>
          Your monthly plan invoices are used first. Purchased invoices stay
          available across months until used, even if your plan changes.
        </p>
        <div className="plan-options">
          <section className="plan-option">
            <strong>30 invoices · USD 1</strong>
            <button
              className="button button-dark"
              disabled={busy || me.role !== "owner"}
              onClick={() => checkout({ topup_invoices: 30 })}
            >
              Buy 30 invoices
            </button>
          </section>
          <section className="plan-option">
            <strong>100 invoices · USD 2</strong>
            <button
              className="button button-dark"
              disabled={busy || me.role !== "owner"}
              onClick={() => checkout({ topup_invoices: 100 })}
            >
              Buy 100 invoices
            </button>
          </section>
        </div>
      </section>
      <p className="helper" style={{ margin: "17px 0 28px" }}>
        Checkout opens on Paynow Zimbabwe. Your plan or invoice top-up activates
        only after payment is verified. Only the business owner can purchase.
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
                  <th>Purchase</th>
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
                      <strong>
                        {p.kind === "invoice_topup"
                          ? `${p.invoice_quantity} invoices`
                          : p.plan}
                      </strong>
                      {p.kind === "subscription" && (
                        <small className="subline">
                          {p.months} month{p.months === 1 ? "" : "s"}
                        </small>
                      )}
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
            title={page > 1 ? "No payments on this page" : "No payments yet"}
            description="Choose a plan or buy extra invoices above when you need more capacity."
          />
        )}
        <Pagination base="/app/billing" data={history.data} />
      </section>
    </>
  );
}
