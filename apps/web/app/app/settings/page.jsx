"use client";

import Link from "next/link";
import { Badge, PageHeader } from "../../../components/ui";
import { useWorkspace } from "../layout";

export default function Settings() {
  const { me, business } = useWorkspace();
  return (
    <>
      <PageHeader
        eyebrow="ACCOUNT DETAILS"
        title="Settings"
        description="Your account and business details in one place."
      />
      <div className="two-col">
        <section className="panel">
          <h2>Account</h2>
          <div className="invoice-line">
            <span>Email</span>
            <strong>{me.email}</strong>
          </div>
          <div className="invoice-line">
            <span>Role</span>
            <Badge>{me.role}</Badge>
          </div>
          <div className="invoice-line">
            <span>Business</span>
            <strong>{business.name}</strong>
          </div>
          <div className="invoice-line">
            <span>Storefront</span>
            <strong>/store/{business.slug}</strong>
          </div>
          <p className="helper" style={{ marginTop: 20 }}>
            Need to change your business profile?{" "}
            <Link className="table-action" href="/app/storefront">
              Edit storefront →
            </Link>
          </p>
        </section>
        <section className="panel">
          <h2>Subscription</h2>
          <p>
            Your current plan is <strong>{business.plan}</strong>.
          </p>
          <p>
            See billing to review usage, plan options, and Paynow payment
            history.
          </p>
          <Link className="button button-light" href="/app/billing">
            View billing →
          </Link>
        </section>
      </div>
    </>
  );
}
