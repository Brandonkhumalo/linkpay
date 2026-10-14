"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";
import { api, errorMessage, money } from "../../../../lib/api";
import { invoiceTemplates } from "../../../../lib/invoice-design";
import {
  Loading,
  Notice,
  PageHeader,
  useResource,
} from "../../../../components/ui";
import { useWorkspace } from "../../layout";

function NewInvoiceContent() {
  const router = useRouter(),
    params = useSearchParams(),
    { business } = useWorkspace();
  const sourceId = params.get("enquiry");
  const [customerPage, setCustomerPage] = useState(1),
    [productPage, setProductPage] = useState(1),
    [selected, setSelected] = useState(null),
    [recipientName, setRecipientName] = useState(""),
    [recipientEmail, setRecipientEmail] = useState(""),
    [recipientPhone, setRecipientPhone] = useState(""),
    [designTemplate, setDesignTemplate] = useState("simple"),
    [designColor, setDesignColor] = useState(business.theme_color || "#235743"),
    [showLogo, setShowLogo] = useState(false),
    [creatingCustomer, setCreatingCustomer] = useState(false),
    [items, setItems] = useState([]),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  const [source, setSource] = useState(null);
  const [sourceLoading, setSourceLoading] = useState(Boolean(sourceId));
  const [sourceError, setSourceError] = useState("");
  const customers = useResource(`/customers?page=${customerPage}`);
  const products = useResource(`/products?page=${productPage}`);
  useEffect(() => {
    if (!sourceId) return;
    let active = true;
    api(`/enquiries/${encodeURIComponent(sourceId)}`)
      .then((enquiry) => {
        if (!active) return;
        setSource(enquiry);
        setItems(
          enquiry.product_name
            ? [
                {
                  description: enquiry.product_name,
                  quantity: 1,
                  unit_price: enquiry.product_price_on_request
                    ? ""
                    : (enquiry.product_price_minor / 100).toFixed(2),
                },
              ]
            : [],
        );
        setCreatingCustomer(true);
      })
      .catch((requestError) => {
        if (active) setSourceError(errorMessage(requestError));
      })
      .finally(() => active && setSourceLoading(false));
    return () => {
      active = false;
    };
  }, [sourceId]);
  function changeItem(index, key, value) {
    setItems((current) =>
      current.map((item, i) =>
        i === index ? { ...item, [key]: value } : item,
      ),
    );
  }
  function addCustomItem() {
    setItems((current) =>
      current.length >= 50
        ? current
        : [...current, { description: "", quantity: 1, unit_price: "" }],
    );
  }
  function selectCustomer(customer) {
    setSelected(customer);
    setRecipientName(customer.name);
    setRecipientEmail(customer.email || "");
    setRecipientPhone(customer.phone || "");
  }
  async function addCustomer(e) {
    e.preventDefault();
    setBusy(true);
    setError("");
    try {
      const data = Object.fromEntries(new FormData(e.currentTarget));
      const created = await api("/customers", {
        method: "POST",
        body: JSON.stringify(data),
      });
      selectCustomer(created);
      setCreatingCustomer(false);
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  }
  async function submit(e) {
    e.preventDefault();
    setError("");
    if (!selected) {
      setError("Choose a customer first.");
      return;
    }
    if (!items.length) {
      setError("Add at least one line item before creating the invoice.");
      return;
    }
    setBusy(true);
    const data = new FormData(e.currentTarget);
    try {
      const result = await api("/invoices", {
        method: "POST",
        body: JSON.stringify({
          customer_id: selected.id,
          recipient_business_name: recipientName.trim(),
          recipient_email: recipientEmail.trim(),
          recipient_phone: recipientPhone.trim(),
          sender_phone: String(data.get("sender_phone")).trim(),
          design_template: designTemplate,
          design_color: designColor,
          show_logo: showLogo && business.has_logo,
          due_at: data.get("due_at"),
          discount_minor: Math.round(Number(data.get("discount")) * 100),
          tax_minor: Math.round(Number(data.get("tax")) * 100),
          notes: data.get("notes"),
          items: items.map((item) => ({
            description: item.description,
            quantity: Number(item.quantity),
            unit_minor: Math.round(Number(item.unit_price) * 100),
          })),
        }),
      });
      router.push(`/app/invoices/${result.id}`);
    } catch (e) {
      setError(errorMessage(e));
      setBusy(false);
    }
  }
  const subtotal = items.reduce(
    (sum, item) =>
      sum +
      Math.round(
        Number(item.quantity || 0) * Number(item.unit_price || 0) * 100,
      ),
    0,
  );
  if (sourceLoading) return <Loading />;
  if (sourceError)
    return (
      <section className="panel">
        <Notice message={`Could not load the enquiry: ${sourceError}`} />
        <Link href="/app/enquiries" className="button button-light">
          ← Back to enquiries
        </Link>
      </section>
    );
  return (
    <>
      <PageHeader
        eyebrow="NEW INVOICE"
        title="Create an invoice"
        description="Add the details, save a draft, then issue it when you are ready."
        action={
          <Link className="button button-light" href="/app/invoices">
            ← Back to invoices
          </Link>
        }
      />
      <Notice message={error} />
      {source && (
        <div className="notice notice-success" role="status">
          Drafting from {source.brand_name || source.name}&apos;s enquiry.
          Confirm the final scope and price before issuing this invoice.
        </div>
      )}
      <div className="page-columns">
        <div className="stack">
          <section className="panel">
            <div className="panel-head">
              <h2>1. Choose recipient</h2>
              <button
                className="button button-light small"
                onClick={() => setCreatingCustomer(!creatingCustomer)}
              >
                {creatingCustomer ? "Select existing" : "+ New customer"}
              </button>
            </div>
            {selected && (
              <div className="notice notice-success">
                Selected: <strong>{selected.name}</strong>
              </div>
            )}
            {creatingCustomer ? (
              <form onSubmit={addCustomer}>
                <label className="field">
                  Recipient business name
                  <input
                    name="name"
                    required
                    minLength={2}
                    maxLength={160}
                    defaultValue={source?.brand_name || source?.name || ""}
                  />
                </label>
                <div className="form-grid">
                  <label className="field">
                    Recipient email (optional)
                    <input
                      name="email"
                      type="email"
                      maxLength={254}
                      defaultValue={source?.email || ""}
                    />
                  </label>
                  <label className="field">
                    Recipient phone (optional)
                    <input
                      name="phone"
                      type="tel"
                      maxLength={40}
                      defaultValue={source?.phone || ""}
                    />
                  </label>
                </div>
                <input
                  type="hidden"
                  name="notes"
                  value="Created while drafting invoice"
                />
                <button className="button button-dark" disabled={busy}>
                  Add and select customer
                </button>
              </form>
            ) : customers.loading ? (
              <Loading />
            ) : customers.error ? (
              <Notice message={customers.error} />
            ) : (
              <>
                <div className="table-wrap">
                  <table className="data-table">
                    <thead>
                      <tr>
                        <th>Name</th>
                        <th>Contact</th>
                        <th></th>
                      </tr>
                    </thead>
                    <tbody>
                      {customers.data.items.map((c) => (
                        <tr key={c.id}>
                          <td>
                            <strong>{c.name}</strong>
                          </td>
                          <td>{c.email || c.phone || "—"}</td>
                          <td>
                            <button
                              className="button button-light small"
                              onClick={() => selectCustomer(c)}
                              type="button"
                            >
                              Select
                            </button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                {!customers.data.items.length && (
                  <p className="helper">
                    No customers on this page. Add one to continue.
                  </p>
                )}
                <div className="pagination">
                  <button
                    className="button button-light small"
                    disabled={!customers.data.has_prev}
                    onClick={() => setCustomerPage((p) => p - 1)}
                  >
                    ← Previous
                  </button>
                  <span>Page {customerPage}</span>
                  <button
                    className="button button-light small"
                    disabled={!customers.data.has_next}
                    onClick={() => setCustomerPage((p) => p + 1)}
                  >
                    Next →
                  </button>
                </div>
              </>
            )}
          </section>
          <section className="panel">
            <h2>2. Invoice details</h2>
            <form id="invoice-form" onSubmit={submit}>
              <h3 style={{ fontSize: 14 }}>Choose invoice style</h3>
              <div
                className="invoice-template-grid"
                role="group"
                aria-label="Invoice template"
              >
                {invoiceTemplates.map((template) => (
                  <button
                    key={template.id}
                    type="button"
                    className={`invoice-template-choice invoice-template-choice--${template.id} ${designTemplate === template.id ? "selected" : ""}`}
                    aria-pressed={designTemplate === template.id}
                    onClick={() => setDesignTemplate(template.id)}
                    style={{ "--invoice-accent": designColor }}
                  >
                    <span className="invoice-template-mini">
                      <i />
                      <i />
                      <i />
                    </span>
                    <strong>{template.name}</strong>
                    <small>{template.description}</small>
                  </button>
                ))}
              </div>
              <div className="form-grid" style={{ marginTop: 18 }}>
                <label className="field">
                  Invoice color
                  <input
                    type="color"
                    value={designColor}
                    onChange={(event) => setDesignColor(event.target.value)}
                  />
                </label>
                {business.has_logo ? (
                  <label className="field invoice-logo-option">
                    <input
                      type="checkbox"
                      checked={showLogo}
                      onChange={(event) => setShowLogo(event.target.checked)}
                    />
                    Add my shop logo to this invoice
                  </label>
                ) : (
                  <p className="helper">
                    Want your logo on invoices?{" "}
                    <Link href="/app/storefront">
                      Upload it in your storefront
                    </Link>{" "}
                    first.
                  </p>
                )}
              </div>
              <h3 style={{ fontSize: 14 }}>Recipient</h3>
              <label className="field">
                Recipient business name
                <input
                  required
                  minLength={2}
                  maxLength={160}
                  value={recipientName}
                  onChange={(event) => setRecipientName(event.target.value)}
                  placeholder="Business being billed"
                />
              </label>
              <div className="form-grid">
                <label className="field">
                  Recipient email (optional)
                  <input
                    type="email"
                    maxLength={254}
                    value={recipientEmail}
                    onChange={(event) => setRecipientEmail(event.target.value)}
                  />
                </label>
                <label className="field">
                  Recipient phone (optional)
                  <input
                    type="tel"
                    maxLength={40}
                    value={recipientPhone}
                    onChange={(event) => setRecipientPhone(event.target.value)}
                  />
                </label>
              </div>
              <h3 style={{ fontSize: 14, marginTop: 24 }}>
                From {business.name}
              </h3>
              <label className="field">
                Sender phone number
                <input
                  name="sender_phone"
                  type="tel"
                  required
                  maxLength={40}
                  defaultValue={business.whatsapp}
                  placeholder="+263771234567"
                />
              </label>
              <label className="field">
                Due date
                <input name="due_at" type="date" required />
              </label>
              <h3 style={{ fontSize: 14, marginTop: 25 }}>Line items</h3>
              <p className="helper">
                Add saved listings, custom items, or both. Custom items only
                appear on this invoice and do not change your catalogue.
              </p>
              <label className="field">
                Add a saved service or product (optional)
                <select
                  value=""
                  disabled={items.length >= 50}
                  onChange={(event) => {
                    const product = products.data?.items.find(
                      (item) => item.id === event.target.value,
                    );
                    if (!product || product.price_on_request) return;
                    setItems((current) =>
                      current.length >= 50
                        ? current
                        : [
                            ...current,
                            {
                              description: product.name,
                              quantity: 1,
                              unit_price: (product.price_minor / 100).toFixed(
                                2,
                              ),
                            },
                          ],
                    );
                  }}
                >
                  <option value="">Choose a priced listing to add</option>
                  {products.data?.items
                    .filter((item) => item.active && !item.price_on_request)
                    .map((item) => (
                      <option key={item.id} value={item.id}>
                        {item.name} ·{" "}
                        {money(item.price_minor, business.currency)}
                      </option>
                    ))}
                </select>
              </label>
              {products.data && (
                <div className="pagination">
                  <button
                    type="button"
                    disabled={!products.data.has_prev}
                    onClick={() => setProductPage(productPage - 1)}
                  >
                    ← Previous
                  </button>
                  <span>Listing page {productPage}</span>
                  <button
                    type="button"
                    disabled={!products.data.has_next}
                    onClick={() => setProductPage(productPage + 1)}
                  >
                    Next →
                  </button>
                </div>
              )}
              <button
                type="button"
                className="button button-light small"
                disabled={items.length >= 50}
                onClick={addCustomItem}
              >
                + Add custom item
              </button>
              {!items.length && (
                <p className="helper" role="status">
                  No items yet. Choose a saved listing or add a custom item.
                </p>
              )}
              {items.map((item, index) => (
                <div className="invoice-item-editor" key={index}>
                  <div className="invoice-item-header">
                    <strong>Item {index + 1}</strong>
                    <button
                      type="button"
                      className="button button-ghost small"
                      onClick={() =>
                        setItems((current) =>
                          current.filter((_, i) => i !== index),
                        )
                      }
                    >
                      Remove
                    </button>
                  </div>
                  <label className="field">
                    Description
                    <input
                      required
                      minLength={2}
                      maxLength={500}
                      value={item.description}
                      onChange={(e) =>
                        changeItem(index, "description", e.target.value)
                      }
                      placeholder="What is this for?"
                    />
                  </label>
                  <div className="form-grid">
                    <label className="field">
                      Qty
                      <input
                        type="number"
                        min="1"
                        max="100000"
                        required
                        value={item.quantity}
                        onChange={(e) =>
                          changeItem(index, "quantity", e.target.value)
                        }
                      />
                    </label>
                    <label className="field">
                      Unit price
                      <input
                        type="number"
                        min="0"
                        max="10000000"
                        step="0.01"
                        required
                        value={item.unit_price}
                        onChange={(e) =>
                          changeItem(index, "unit_price", e.target.value)
                        }
                      />
                    </label>
                  </div>
                </div>
              ))}
              {items.length > 0 && (
                <button
                  type="button"
                  className="button button-light small"
                  disabled={items.length >= 50}
                  onClick={addCustomItem}
                >
                  + Add another custom item
                </button>
              )}
              <div className="form-grid" style={{ marginTop: 22 }}>
                <label className="field">
                  Discount ({business.currency})
                  <input
                    name="discount"
                    type="number"
                    min="0"
                    step="0.01"
                    defaultValue="0"
                  />
                </label>
                <label className="field">
                  Tax ({business.currency})
                  <input
                    name="tax"
                    type="number"
                    min="0"
                    step="0.01"
                    defaultValue="0"
                  />
                </label>
              </div>
              <label className="field">
                Notes
                <textarea
                  name="notes"
                  maxLength={2000}
                  placeholder="Payment instructions or other details"
                />
              </label>
              <button className="button button-dark" disabled={busy}>
                {busy ? "Creating…" : "Create draft invoice →"}
              </button>
            </form>
          </section>
        </div>
        <aside className="stack">
          <section className="panel">
            <span className="eyebrow">SUMMARY</span>
            <h2>Invoice preview</h2>
            <p>
              {
                invoiceTemplates.find((item) => item.id === designTemplate)
                  ?.name
              }{" "}
              style · {showLogo ? "With logo" : "No logo"}
            </p>
            <p>From: {business.name}</p>
            <p>To: {recipientName || "Select a recipient"}</p>
            <div className="invoice-line">
              <span>Subtotal</span>
              <strong>{money(subtotal, business.currency)}</strong>
            </div>
            <p className="helper" style={{ marginTop: 18 }}>
              Discount and tax are applied when you save the draft. You can
              review it before issuing.
            </p>
          </section>
          <section className="panel" style={{ background: "#edf5e6" }}>
            <h2>What happens next?</h2>
            <p>
              Your invoice starts as a draft. Issue it to create a shareable
              invoice link and branded PDF. Customers pay you outside LinkPay;
              record the payment here after you receive it.
            </p>
          </section>
        </aside>
      </div>
    </>
  );
}
export default function NewInvoice() {
  return (
    <Suspense fallback={<Loading />}>
      <NewInvoiceContent />
    </Suspense>
  );
}
