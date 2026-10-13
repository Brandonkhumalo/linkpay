"use client";

import Link from "next/link";
import { Suspense, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { api, errorMessage } from "../../../lib/api";
import { Brand, Loading, Notice } from "../../../components/ui";

function BillingReturnContent() {
  const id = useSearchParams().get("payment"),
    [state, setState] = useState({ loading: true, error: "" });
  const invalid = !id || !/^[0-9a-f-]{36}$/.test(id);
  useEffect(() => {
    if (invalid) return;
    let active = true;
    api(`/billing/payments/${id}/refresh`, { method: "POST" })
      .then(() => active && setState({ loading: false, error: "" }))
      .catch(
        (error) =>
          active && setState({ loading: false, error: errorMessage(error) }),
      );
    return () => {
      active = false;
    };
  }, [id, invalid]);
  const display = invalid
    ? { loading: false, error: "Payment reference is missing." }
    : state;
  return (
    <div className="auth-plain">
      <header className="public-header">
        <Brand />
        <Link href="/app/billing">Go to billing →</Link>
      </header>
      <section className="auth-card">
        <span className="eyebrow">PAYNOW ZIMBABWE</span>
        <h1>
          {display.loading
            ? "Checking your payment"
            : display.error
              ? "Verification pending"
              : "Payment checked"}
        </h1>
        <p>
          {display.loading
            ? "We are confirming the result with Paynow."
            : "Your plan status is based on Paynow's verified result. Review it in billing."}
        </p>
        {display.loading ? <Loading /> : <Notice message={display.error} />}
        <Link className="button button-dark" href="/app/billing">
          View billing
        </Link>
      </section>
    </div>
  );
}
export default function BillingReturn() {
  return (
    <Suspense fallback={<Loading />}>
      <BillingReturnContent />
    </Suspense>
  );
}
