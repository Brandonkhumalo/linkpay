import Link from "next/link";
import { Brand, Badge } from "../../../components/ui";
import { date, money } from "../../../lib/api";
import { publicApi } from "../../../lib/server-api";

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
        <section className="panel" style={{ padding: 30 }}>
          <div className="public-invoice-top">
            <div>
              <span className="eyebrow">
                INVOICE FROM {b.name.toUpperCase()}
              </span>
              <h1>#{String(i.number).padStart(6, "0")}</h1>
            </div>
            <Badge>{i.status}</Badge>
          </div>
          <div className="public-invoice-meta">
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
              <strong>Payment notes</strong>
              <p>{i.notes}</p>
            </div>
          )}
          <p className="helper" style={{ marginTop: 20 }}>
            Payment is arranged directly with {b.name}. LinkPay does not collect
            your payment on this page.
          </p>
        </section>
        <p className="helper" style={{ textAlign: "center", marginTop: 25 }}>
          Powered by <Link href="/">LinkPay</Link>
        </p>
      </main>
    </>
  );
}
