"use client";

import Link from "next/link";
import Image from "next/image";
import { useState } from "react";
import { api, date, errorMessage, money } from "../lib/api";
import {
  Badge,
  Empty,
  Icon,
  Loading,
  Notice,
  PageHeader,
  Pagination,
  useResource,
} from "./ui";
import { useWorkspace } from "../app/app/layout";

function Modal({ title, onClose, children }) {
  return (
    <div
      className="modal-overlay"
      onMouseDown={(e) => e.target === e.currentTarget && onClose()}
    >
      <section
        className="modal-card"
        role="dialog"
        aria-modal="true"
        aria-label={title}
      >
        <div className="modal-head">
          <h2>{title}</h2>
          <button
            className="modal-close"
            type="button"
            aria-label="Close"
            onClick={onClose}
          >
            ×
          </button>
        </div>
        {children}
      </section>
    </div>
  );
}

export function Products({ page }) {
  const { business } = useWorkspace();
  const [key, refresh] = useState(0);
  const [editing, setEditing] = useState(null);
  const [quote, setQuote] = useState(false);
  const [kind, setKind] = useState("product");
  const [price, setPrice] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const result = useResource(`/products?page=${page}`, key);
  function openEditor(listing) {
    setEditing(listing);
    setQuote(Boolean(listing.price_on_request));
    setKind(listing.kind || "product");
    setPrice(
      !listing.price_on_request && listing.price_minor != null
        ? (listing.price_minor / 100).toFixed(2)
        : "",
    );
  }
  async function save(e) {
    e.preventDefault();
    setBusy(true);
    setNotice("");
    const form = e.currentTarget;
    const data = new FormData(form);
    const body = {
      name: String(data.get("name")).trim(),
      description: String(data.get("description")).trim(),
      kind,
      price_minor: quote ? 0 : Math.round(Number(data.get("price")) * 100),
      price_on_request: quote,
      active: data.has("active"),
    };
    try {
      const saved = await api(
        editing?.id ? `/products/${editing.id}` : "/products",
        { method: editing?.id ? "PATCH" : "POST", body: JSON.stringify(body) },
      );
      setEditing(saved);
      const image = data.get("image");
      if (image?.size) {
        const upload = new FormData();
        upload.set("image", image);
        try {
          await api(`/products/${saved.id}/image`, {
            method: "POST",
            body: upload,
          });
        } catch (error) {
          throw new Error(
            `Listing saved, but image upload failed: ${errorMessage(error)}`,
          );
        }
      }
      setEditing(null);
      refresh((v) => v + 1);
      setNotice("Listing saved.");
    } catch (error) {
      setNotice(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }
  async function remove() {
    if (
      !window.confirm(
        "Delete this listing? Listings with enquiries can be archived instead.",
      )
    )
      return;
    try {
      await api(`/products/${editing.id}`, { method: "DELETE" });
      setEditing(null);
      refresh((v) => v + 1);
      setNotice("Listing deleted.");
    } catch (error) {
      setNotice(errorMessage(error));
    }
  }
  return (
    <>
      <PageHeader
        eyebrow="YOUR CATALOGUE"
        title="Products & services"
        description="Show customers exactly what you offer."
        action={
          <button className="button button-dark" onClick={() => openEditor({})}>
            <Icon name="plus" size={16} /> Add listing
          </button>
        }
      />
      <Notice
        message={notice}
        kind={
          notice === "Listing saved." || notice === "Listing deleted."
            ? "success"
            : "error"
        }
      />
      <section className="panel">
        {result.loading ? (
          <Loading />
        ) : result.error ? (
          <Notice message={result.error} />
        ) : result.data.items.length ? (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Listing</th>
                  <th>Type</th>
                  <th>Price</th>
                  <th>Status</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {result.data.items.map((p) => (
                  <tr key={p.id}>
                    <td>
                      <div className="product-cell">
                        {p.has_image ? (
                          <Image
                            unoptimized
                            width={42}
                            height={42}
                            className="product-thumb"
                            src={`/api/v1/products/${p.id}/image`}
                            alt=""
                          />
                        ) : (
                          <span className="product-thumb">✦</span>
                        )}
                        <span>
                          <strong>{p.name}</strong>
                          <small className="subline">
                            {p.description.slice(0, 55)}
                          </small>
                        </span>
                      </div>
                    </td>
                    <td>{p.kind}</td>
                    <td>
                      <strong>
                        {p.price_on_request
                          ? "Request a quote"
                          : money(p.price_minor, business.currency)}
                      </strong>
                    </td>
                    <td>
                      <Badge>{p.active ? "active" : "hidden"}</Badge>
                    </td>
                    <td>
                      <button
                        className="button button-light small"
                        onClick={() => openEditor(p)}
                      >
                        Edit
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <Empty
            title={
              page > 1 ? "No listings on this page" : "Add your first listing"
            }
            description="Products and services you add will appear on your storefront."
            action={
              page === 1 ? (
                <div className="form-actions">
                  <button
                    className="button button-primary"
                    onClick={() => openEditor({})}
                  >
                    Add listing
                  </button>
                  <button
                    className="button button-light"
                    onClick={() =>
                      openEditor({
                        name: "Example service",
                        description:
                          "Describe the result your customer receives",
                        kind: "service",
                        price_minor: 0,
                        active: false,
                      })
                    }
                  >
                    Start from example
                  </button>
                </div>
              ) : null
            }
          />
        )}
        <Pagination base="/app/products" data={result.data} />
      </section>
      {editing && (
        <Modal
          title={editing.id ? "Edit listing" : "Add listing"}
          onClose={() => setEditing(null)}
        >
          <form onSubmit={save}>
            <label className="field">
              Name
              <input
                name="name"
                defaultValue={editing.name || ""}
                required
                minLength={2}
                maxLength={160}
              />
            </label>
            <label className="field">
              Description
              <textarea
                name="description"
                defaultValue={editing.description || ""}
                maxLength={2000}
              />
            </label>
            <label className="field">
              Type
              <select
                name="kind"
                value={kind}
                onChange={(event) => {
                  setKind(event.target.value);
                  if (event.target.value !== "service") setQuote(false);
                }}
              >
                <option value="product">Product</option>
                <option value="service">Service</option>
              </select>
            </label>
            {kind === "service" && (
              <fieldset className="listing-pricing">
                <legend>How should customers see the price?</legend>
                <label>
                  <input
                    type="radio"
                    name="pricing_mode"
                    checked={!quote}
                    onChange={() => setQuote(false)}
                  />
                  Show my fixed price
                </label>
                <label>
                  <input
                    type="radio"
                    name="pricing_mode"
                    checked={quote}
                    onChange={() => setQuote(true)}
                  />
                  Price on request for custom work
                </label>
              </fieldset>
            )}
            {!quote && (
              <label className="field">
                Price ({business.currency})
                <input
                  name="price"
                  type="number"
                  min="0"
                  max="1000000000"
                  step="0.01"
                  value={price}
                  onChange={(event) => setPrice(event.target.value)}
                  required
                />
                <span className="helper">
                  {business.profile_type === "creator"
                    ? "Brand partners will see this price and can send a brief without requesting a quote."
                    : "Customers will see this price and can request an invoice without asking for a quote."}
                </span>
              </label>
            )}
            <label className="field">
              Image (JPEG or PNG, up to 3 MB)
              <input name="image" type="file" accept="image/jpeg,image/png" />
            </label>
            <label className="field checkbox-field">
              <input
                name="active"
                type="checkbox"
                defaultChecked={editing.active !== false}
              />{" "}
              Show on storefront
            </label>
            <div className="form-actions">
              <button className="button button-dark" disabled={busy}>
                {busy ? "Saving…" : "Save listing"}
              </button>
              {editing.id && (
                <button
                  type="button"
                  className="button button-danger"
                  onClick={remove}
                >
                  Delete
                </button>
              )}
            </div>
          </form>
        </Modal>
      )}
    </>
  );
}

export function Customers({ page }) {
  const [key, refresh] = useState(0),
    [show, setShow] = useState(false),
    [notice, setNotice] = useState(""),
    [busy, setBusy] = useState(false);
  const result = useResource(`/customers?page=${page}`, key);
  async function save(e) {
    e.preventDefault();
    setBusy(true);
    setNotice("");
    const data = Object.fromEntries(new FormData(e.currentTarget));
    try {
      await api("/customers", { method: "POST", body: JSON.stringify(data) });
      setShow(false);
      refresh((v) => v + 1);
      setNotice("Customer saved.");
    } catch (error) {
      setNotice(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }
  return (
    <>
      <PageHeader
        eyebrow="PEOPLE BEHIND THE PAYMENTS"
        title="Customers"
        description="Keep the details that help you stay connected."
        action={
          <button className="button button-dark" onClick={() => setShow(true)}>
            <Icon name="plus" size={16} /> Add customer
          </button>
        }
      />
      <Notice
        message={notice}
        kind={notice.includes("saved") ? "success" : "error"}
      />
      <section className="panel">
        {result.loading ? (
          <Loading />
        ) : result.error ? (
          <Notice message={result.error} />
        ) : result.data.items.length ? (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Customer</th>
                  <th>Email</th>
                  <th>Phone</th>
                  <th>Notes</th>
                </tr>
              </thead>
              <tbody>
                {result.data.items.map((c) => (
                  <tr key={c.id}>
                    <td>
                      <strong>{c.name}</strong>
                    </td>
                    <td>{c.email || "—"}</td>
                    <td>{c.phone || "—"}</td>
                    <td>{c.notes || "—"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <Empty
            title={
              page > 1
                ? "No customers on this page"
                : "Your customer list starts here"
            }
            description="Add a customer to create their first invoice."
            action={
              page === 1 ? (
                <button
                  className="button button-primary"
                  onClick={() => setShow(true)}
                >
                  Add customer
                </button>
              ) : null
            }
          />
        )}
        <Pagination base="/app/customers" data={result.data} />
      </section>
      {show && (
        <Modal title="Add customer" onClose={() => setShow(false)}>
          <form onSubmit={save}>
            <label className="field">
              Full name
              <input name="name" required minLength={2} maxLength={160} />
            </label>
            <div className="form-grid">
              <label className="field">
                Email
                <input name="email" type="email" maxLength={254} />
              </label>
              <label className="field">
                Phone
                <input name="phone" maxLength={40} />
              </label>
            </div>
            <label className="field">
              Notes
              <textarea name="notes" maxLength={2000} />
            </label>
            <button className="button button-dark" disabled={busy}>
              {busy ? "Saving…" : "Save customer"}
            </button>
          </form>
        </Modal>
      )}
    </>
  );
}

export function Enquiries({ page }) {
  const { business } = useWorkspace();
  const [key, refresh] = useState(0);
  const result = useResource(`/enquiries?page=${page}`, key);
  const [notice, setNotice] = useState("");
  const [quoting, setQuoting] = useState(null);
  const [quoteLink, setQuoteLink] = useState("");
  const [quoteBusy, setQuoteBusy] = useState(false);
  async function updateStage(id, stage) {
    try {
      await api(`/enquiries/${id}/stage`, {
        method: "PATCH",
        body: JSON.stringify({ stage }),
      });
      window.dispatchEvent(new Event("linkpay:notifications-changed"));
      refresh((value) => value + 1);
    } catch (error) {
      setNotice(errorMessage(error));
    }
  }
  async function createQuote(event) {
    event.preventDefault();
    if (quoteBusy) return;
    setQuoteBusy(true);
    const form = new FormData(event.currentTarget);
    try {
      const result = await api("/quotes", {
        method: "POST",
        body: JSON.stringify({
          enquiry_id: quoting.id,
          description: String(form.get("description")).trim(),
          amount_minor: Math.round(Number(form.get("amount")) * 100),
          expires_at: form.get("expires_at"),
        }),
      });
      setQuoting(null);
      setQuoteLink(result.url);
      setNotice("Quote created. Share its private link with the customer.");
      window.dispatchEvent(new Event("linkpay:notifications-changed"));
      refresh((value) => value + 1);
    } catch (error) {
      setNotice(errorMessage(error));
    } finally {
      setQuoteBusy(false);
    }
  }
  async function addCustomer(e) {
    try {
      await api("/customers", {
        method: "POST",
        body: JSON.stringify({
          name: e.name,
          email: e.email,
          phone: e.phone,
          notes: "Created from storefront enquiry",
        }),
      });
      setNotice("Customer added. You can now create an invoice for them.");
    } catch (error) {
      setNotice(errorMessage(error));
    }
  }
  return (
    <>
      <PageHeader
        eyebrow="NEW CONVERSATIONS"
        title="Enquiries"
        description="Follow up with people interested in what you offer."
        action={
          <Link className="button button-light" href="/app/quotes">
            View quotes →
          </Link>
        }
      />
      <Notice
        message={notice}
        kind={
          notice.startsWith("Customer added") ||
          notice.startsWith("Quote created") ||
          notice.startsWith("Quote link copied")
            ? "success"
            : "error"
        }
      />
      {quoteLink && (
        <section className="panel" style={{ marginBottom: 16 }}>
          <strong>Quote ready to share</strong>
          <p className="helper">{quoteLink}</p>
          <div className="form-actions">
            <button
              className="button button-light"
              onClick={async () => {
                try {
                  await navigator.clipboard.writeText(quoteLink);
                  setNotice("Quote link copied.");
                } catch {
                  setNotice("Copy the quote link shown above.");
                }
              }}
            >
              Copy quote link
            </button>
            <a
              className="button button-light"
              href={quoteLink}
              target="_blank"
              rel="noopener noreferrer"
            >
              Preview quote ↗
            </a>
          </div>
        </section>
      )}
      <section className="panel">
        {result.loading ? (
          <Loading />
        ) : result.error ? (
          <Notice message={result.error} />
        ) : result.data.items.length ? (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>From</th>
                  <th>Message</th>
                  <th>Received</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {result.data.items.map((e) => (
                  <tr key={e.id}>
                    <td>
                      <strong>{e.name}</strong>
                      <small className="subline">{e.email || e.phone}</small>
                    </td>
                    <td style={{ maxWidth: 360 }}>
                      {e.brand_name && (
                        <strong>
                          {e.brand_name}
                          <br />
                        </strong>
                      )}
                      {e.product_name && (
                        <small className="subline">
                          Package: {e.product_name}
                        </small>
                      )}
                      {e.message}
                      {e.deliverables && (
                        <p>
                          <strong>Deliverables:</strong> {e.deliverables}
                        </p>
                      )}
                      {e.budget_minor != null && (
                        <p>
                          <strong>Budget:</strong>{" "}
                          {money(e.budget_minor, business.currency)}
                        </p>
                      )}
                      {e.campaign_deadline && (
                        <p>
                          <strong>Target date:</strong>{" "}
                          {date(e.campaign_deadline)}
                        </p>
                      )}
                      {e.usage_rights && (
                        <p>
                          <strong>Usage rights:</strong> {e.usage_rights}
                        </p>
                      )}
                    </td>
                    <td>{date(e.created_at)}</td>
                    <td>
                      <div className="list-actions">
                        <select
                          aria-label={`Stage for ${e.name}`}
                          value={e.stage}
                          onChange={(event) =>
                            updateStage(e.id, event.target.value)
                          }
                        >
                          <option value="new">New</option>
                          <option value="contacted">Contacted</option>
                          <option value="quoted">Quoted</option>
                          <option value="won">Won</option>
                          <option value="lost">Lost</option>
                        </select>
                        <button onClick={() => setQuoting(e)}>
                          Create quote
                        </button>
                        <button onClick={() => addCustomer(e)}>
                          Add customer
                        </button>
                        <Link
                          className="table-action"
                          href={`/app/invoices/new?enquiry=${e.id}`}
                        >
                          Create invoice after agreement →
                        </Link>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <Empty
            title={page > 1 ? "No enquiries on this page" : "No enquiries yet"}
            description="Share your storefront so customers can reach you."
          />
        )}
        <Pagination base="/app/enquiries" data={result.data} />
      </section>
      {quoting && (
        <Modal
          title={`Quote for ${quoting.name}`}
          onClose={() => setQuoting(null)}
        >
          <form onSubmit={createQuote}>
            <label className="field">
              Scope and deliverables
              <textarea
                name="description"
                minLength={2}
                maxLength={2000}
                required
                defaultValue={
                  quoting.deliverables || quoting.product_name || ""
                }
              />
            </label>
            <div className="form-grid">
              <label className="field">
                Price ({business.currency})
                <input
                  name="amount"
                  type="number"
                  min="0.01"
                  max="10000000"
                  step="0.01"
                  required
                  defaultValue={
                    quoting.product_name && !quoting.product_price_on_request
                      ? (quoting.product_price_minor / 100).toFixed(2)
                      : ""
                  }
                />
              </label>
              <label className="field">
                Valid until
                <input name="expires_at" type="date" required />
              </label>
            </div>
            <p className="helper">
              Creating a quote gives you a private link to share. The customer
              can accept or decline it.
            </p>
            <button className="button button-dark" disabled={quoteBusy}>
              {quoteBusy ? "Creating…" : "Create quote"}
            </button>
          </form>
        </Modal>
      )}
    </>
  );
}

export function Invoices({ page }) {
  const result = useResource(`/invoices?page=${page}`);
  return (
    <>
      <PageHeader
        eyebrow="GETTING PAID"
        title="Invoices"
        description="Create, share, and track every invoice."
        action={
          <Link className="button button-dark" href="/app/invoices/new">
            <Icon name="plus" size={16} /> Create invoice
          </Link>
        }
      />
      <section className="panel">
        {result.loading ? (
          <Loading />
        ) : result.error ? (
          <Notice message={result.error} />
        ) : result.data.items.length ? (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Invoice</th>
                  <th>Customer</th>
                  <th>Due date</th>
                  <th>Status</th>
                  <th>Balance</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {result.data.items.map((i) => (
                  <tr key={i.id}>
                    <td>
                      <strong>#{String(i.number).padStart(6, "0")}</strong>
                    </td>
                    <td>{i.customer_name}</td>
                    <td>{date(i.due_at)}</td>
                    <td>
                      <Badge>{i.status}</Badge>
                    </td>
                    <td>
                      <strong>{money(i.balance_minor, i.currency)}</strong>
                    </td>
                    <td>
                      <Link
                        className="button button-light small"
                        href={`/app/invoices/${i.id}`}
                      >
                        Open →
                      </Link>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <Empty
            title={
              page > 1
                ? "No invoices on this page"
                : "Your first invoice starts here"
            }
            description="Create a draft, issue it, and share its link with your customer."
            action={
              page === 1 ? (
                <Link
                  className="button button-primary"
                  href="/app/invoices/new"
                >
                  Create invoice
                </Link>
              ) : null
            }
          />
        )}
        <Pagination base="/app/invoices" data={result.data} />
      </section>
    </>
  );
}
