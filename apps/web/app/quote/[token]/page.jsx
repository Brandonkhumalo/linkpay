import Link from "next/link";
import { Brand } from "../../../components/ui";
import QuoteResponse from "../../../components/quote-response";
import { date, money } from "../../../lib/api";
import { publicApi } from "../../../lib/server-api";

export const metadata = {
  title: "Review quote",
  robots: { index: false, follow: false },
};

export default async function QuotePage({ params }) {
  const { token } = await params;
  const { quote, business_name: businessName } = await publicApi(
    `/public/quotes/${encodeURIComponent(token)}`,
  );
  return (
    <>
      <header className="public-header">
        <Brand />
        <span className="helper">Private quote link</span>
      </header>
      <main className="invoice-public">
        <section className="panel" style={{ padding: 30 }}>
          <span className="eyebrow">
            QUOTE FROM {businessName.toUpperCase()}
          </span>
          <h1>Review your quote</h1>
          <p>Prepared for {quote.customer_name}</p>
          <div className="preview-card">
            <strong>Scope</strong>
            <p>{quote.description}</p>
          </div>
          <div className="invoice-line total">
            <span>Total</span>
            <strong>{money(quote.amount_minor, quote.currency)}</strong>
          </div>
          <p className="helper">
            Valid until {date(quote.expires_at)}. Accepting confirms the scope
            and price shown here; payment is arranged directly with{" "}
            {businessName} after invoicing.
          </p>
          <QuoteResponse token={token} initialStatus={quote.status} />
        </section>
        <p className="helper" style={{ textAlign: "center", marginTop: 25 }}>
          Powered by <Link href="/">LinkPay</Link>
        </p>
      </main>
    </>
  );
}
