"use client";

import Link from "next/link";
import { useState } from "react";
import { api, errorMessage, notificationDestination } from "../../lib/api";
import { money } from "../../lib/api";
import {
  Badge,
  Empty,
  Icon,
  Loading,
  Notice,
  PageHeader,
  useResource,
} from "../../components/ui";
import { useWorkspace } from "./layout";

export default function Overview() {
  const { business, me } = useWorkspace();
  const [notificationKey, refreshNotifications] = useState(0);
  const [notice, setNotice] = useState("");
  const overview = useResource("/analytics/overview");
  const subscription = useResource("/subscription");
  const invoices = useResource("/invoices?page=1");
  const notifications = useResource("/notifications", notificationKey);
  const followups = useResource("/followups");
  async function readNotification(id) {
    try {
      await api(`/notifications/${id}/read`, { method: "POST" });
      refreshNotifications((value) => value + 1);
    } catch (error) {
      setNotice(errorMessage(error));
    }
  }
  const firstName = me.email.split("@")[0].split(/[._-]/)[0];
  const stats = overview.data && [
    ["Total invoiced", overview.data.invoiced_minor, "file"],
    ["Payments recorded", overview.data.received_minor, "card"],
    ["Outstanding", overview.data.outstanding_minor, "grid"],
    ["Overdue", overview.data.overdue_minor, "bell"],
  ];
  return (
    <>
      <PageHeader
        eyebrow="YOUR BUSINESS AT A GLANCE"
        title={`Good to see you, ${firstName}.`}
        description="Here is what is happening with your business today."
        action={
          <Link className="button button-dark" href="/app/invoices/new">
            <Icon name="plus" size={17} /> Create invoice
          </Link>
        }
      />
      <Notice message={notice} />
      <section className="panel" style={{ marginBottom: 20 }}>
        <div className="panel-head">
          <div>
            <span className="eyebrow">GET STARTED</span>
            <h2>Make your link useful</h2>
          </div>
        </div>
        <div className="quick-actions">
          <Link href="/app/storefront">
            {business.published ? "✓" : "1"} Publish your storefront{" "}
            <span>↗</span>
          </Link>
          <Link href="/app/products">
            {subscription.data?.products_used ? "✓" : "2"} Add a product or
            service <span>↗</span>
          </Link>
          <Link href="/app/storefront">
            3 Share your link and QR code <span>↗</span>
          </Link>
        </div>
      </section>
      {overview.loading ? (
        <Loading />
      ) : overview.error ? (
        <Notice message={overview.error} />
      ) : (
        <div className="stats-grid">
          {stats.map(([label, value, icon], index) => (
            <div
              className={`stat-card ${index === 2 ? "featured" : ""}`}
              key={label}
            >
              <span className="stat-icon">
                <Icon name={icon} size={17} />
              </span>
              <small>{label}</small>
              <strong>{money(value, business.currency)}</strong>
            </div>
          ))}
        </div>
      )}
      <div className="content-grid">
        <div className="stack">
          <section className="panel">
            <div className="panel-head">
              <h2>Activity</h2>
              <Link href="/app/activity">View all →</Link>
            </div>
            {notifications.loading ? (
              <Loading />
            ) : notifications.error ? (
              <Notice message={notifications.error} />
            ) : notifications.data.items.length ? (
              <div className="stack" style={{ gap: 10 }}>
                {notifications.data.items.slice(0, 5).map((item) => (
                  <div className="preview-card" key={item.id}>
                    <strong>
                      {item.read ? "" : "● "}
                      {item.title}
                    </strong>
                    <div className="form-actions">
                      <Link href={notificationDestination(item.kind)}>
                        Review →
                      </Link>
                      {!item.read && (
                        <button onClick={() => readNotification(item.id)}>
                          Mark read
                        </button>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <p>No new activity yet.</p>
            )}
            {followups.data && (
              <p className="helper">
                <Link href="/app/followups">View customer follow-ups →</Link>
              </p>
            )}
          </section>
          <section className="panel">
            <div className="panel-head">
              <div>
                <h2>Recent invoices</h2>
                <span className="helper">
                  Keep track of what has been sent and paid.
                </span>
              </div>
              <Link className="button button-light small" href="/app/invoices">
                View all →
              </Link>
            </div>
            {invoices.loading ? (
              <Loading />
            ) : invoices.error ? (
              <Notice message={invoices.error} />
            ) : invoices.data.items.length ? (
              <div className="table-wrap">
                <table className="data-table">
                  <thead>
                    <tr>
                      <th>Invoice</th>
                      <th>Customer</th>
                      <th>Due</th>
                      <th>Status</th>
                      <th>Balance</th>
                    </tr>
                  </thead>
                  <tbody>
                    {invoices.data.items.slice(0, 5).map((i) => (
                      <tr key={i.id}>
                        <td>
                          <Link
                            className="table-action"
                            href={`/app/invoices/${i.id}`}
                          >
                            #{String(i.number).padStart(6, "0")}
                          </Link>
                        </td>
                        <td>
                          <strong>{i.customer_name}</strong>
                        </td>
                        <td>{i.due_at}</td>
                        <td>
                          <Badge>{i.status}</Badge>
                        </td>
                        <td>
                          <strong>{money(i.balance_minor, i.currency)}</strong>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <Empty
                title="Your first invoice starts here"
                description="Create an invoice to start tracking payments."
                action={
                  <Link
                    className="button button-primary"
                    href="/app/invoices/new"
                  >
                    Create invoice →
                  </Link>
                }
              />
            )}
          </section>
          <section className="panel">
            <h2>Quick actions</h2>
            <p>A few clicks to keep your business moving.</p>
            <div className="quick-actions">
              <Link href="/app/products">
                Add a listing <span>↗</span>
              </Link>
              <Link href="/app/customers">
                Add a customer <span>↗</span>
              </Link>
              <Link href="/app/storefront">
                Edit storefront <span>↗</span>
              </Link>
            </div>
          </section>
        </div>
        <div className="stack">
          <section className="panel">
            <div className="panel-head">
              <h2>Your plan</h2>
              {subscription.data && <Badge>{subscription.data.plan}</Badge>}
            </div>
            {subscription.loading ? (
              <Loading />
            ) : subscription.error ? (
              <Notice message={subscription.error} />
            ) : (
              <>
                <p>Your plan is ready to support your next step.</p>
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
                <p className="helper">
                  {subscription.data.invoice_topup_balance} purchased invoices
                  available after your monthly plan invoices are used.
                </p>
                <Link className="button button-light small" href="/app/billing">
                  Explore plans and top-ups →
                </Link>
              </>
            )}
          </section>
          <section className="panel" style={{ background: "#e9f2e0" }}>
            <span className="eyebrow">YOUR PUBLIC LINK</span>
            <h2>Ready to share?</h2>
            <p>Make it easy for customers to discover what you offer.</p>
            <Link
              className="button button-dark small"
              href={`/store/${business.slug}`}
              target="_blank"
            >
              Open storefront <Icon name="external" size={14} />
            </Link>
          </section>
        </div>
      </div>
    </>
  );
}
