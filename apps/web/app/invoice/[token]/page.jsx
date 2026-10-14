import Link from "next/link";
import Image from "next/image";
import { Brand, Badge } from "../../../components/ui";
import { date, money } from "../../../lib/api";
import { publicApi } from "../../../lib/server-api";
import PaymentClaim from "../../../components/payment-claim";
import { invoiceTemplates } from "../../../lib/invoice-design";

export const metadata = {
  title: "View invoice",
  robots: { index: false, follow: false },
};
export default async function InvoicePage({ params }) {
  const { token } = await params;
  const { invoice: i, business: b } = await publicApi(
    `/public/invoices/${encodeURIComponent(token)}`,
  );
  return (
    <>
      <header className="public-header">
        <Brand />
        <span className="helper">Private invoice link</span>
      </header>
      <main className="invoice-public">
        <section
          className={`panel invoice-sheet invoice-sheet--${i.design_template}`}
          style={{ "--invoice-accent": i.design_color }}
        >
          <div className="invoice-sheet-heading">
            <div className="invoice-sheet-identity">
              {i.show_logo && (
                <Image
                  unoptimized
                  width={76}
                  height={76}
                  src={`/api/v1/public/invoices/${encodeURIComponent(token)}/logo`}
                  alt={`${b.name} logo`}
                />
              )}
              <div>
                <span className="eyebrow">FROM</span>
                <strong>{b.name}</strong>
                <small>{b.phone}</small>
              </div>
            </div>
            <div className="invoice-sheet-title">
              <span className="eyebrow">
                {
                  invoiceTemplates.find((item) => item.id === i.design_template)
                    ?.name
                }{" "}
                invoice
              </span>
              <h1>#{String(i.number).padStart(6, "0")}</h1>
              <Badge>{i.status}</Badge>
            </div>
          </div>
          <div className="public-invoice-meta">
            <div>
              <small>From</small>
              <strong>{b.name}</strong>
              {b.phone && <p>{b.phone}</p>}
            </div>
            <div>
              <small>Billed to</small>
              <strong>{i.customer_name}</strong>
            </div>
            <div>
              <small>Due date</small>
              <strong>{date(i.due_at)}</strong>
            </div>
          </div>
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Description</th>
                  <th>Qty</th>
                  <th>Amount</th>
                </tr>
              </thead>
              <tbody>
                {i.items.map((item, index) => (
                  <tr key={index}>
                    <td>
                      <strong>{item.description}</strong>
                    </td>
                    <td>{item.quantity}</td>
                    <td>{money(item.total_minor, i.currency)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <div className="invoice-totals">
            <div className="invoice-line">
              <span>Total</span>
              <span>{money(i.total_minor, i.currency)}</span>
            </div>
            <div className="invoice-line">
              <span>Paid</span>
              <span>− {money(i.paid_minor, i.currency)}</span>
            </div>
            <div className="invoice-line total">
              <span>Balance due</span>
              <span>{money(i.balance_minor, i.currency)}</span>
            </div>
          </div>
          {i.notes && (
            <div className="preview-card">
              <strong>Scope and notes</strong>
              <p>{i.notes}</p>
            </div>
          )}
          <div className="preview-card" style={{ marginTop: 16 }}>
            <strong>How to pay {b.name}</strong>
            <p>
              {b.payment_instructions ||
                `Contact ${b.name} to arrange payment directly.`}
            </p>
            {b.whatsapp && (
              <a
                href={`https://wa.me/${b.whatsapp.replace(/\D/g, "")}`}
                target="_blank"
                rel="noopener noreferrer"
              >
                Ask on WhatsApp ↗
              </a>
            )}
          </div>
          <p className="helper" style={{ marginTop: 20 }}>
            Payment is arranged directly with {b.name}. LinkPay does not collect
            your payment on this page.
          </p>
        </section>
        <PaymentClaim
          token={token}
          balanceMinor={i.balance_minor}
          currency={i.currency}
        />
        <p className="helper" style={{ textAlign: "center", marginTop: 25 }}>
          Powered by <Link href="/">LinkPay</Link>
        </p>
      </main>
    </>
  );
}
