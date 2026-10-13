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
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const result = useResource(`/products?page=${page}`, key);
  async function save(e) {
    e.preventDefault();
    setBusy(true);
    setNotice("");
    const form = e.currentTarget;
    const data = new FormData(form);
    const body = {
      name: String(data.get("name")).trim(),
      description: String(data.get("description")).trim(),
      kind: data.get("kind"),
      price_minor: Math.round(Number(data.get("price")) * 100),
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
          <button className="button button-dark" onClick={() => setEditing({})}>
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
                      <strong>{money(p.price_minor, business.currency)}</strong>
                    </td>
                    <td>
                      <Badge>{p.active ? "active" : "hidden"}</Badge>
                    </td>
                    <td>
                      <button
                        className="button button-light small"
                        onClick={() => setEditing(p)}
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
                <button
                  className="button button-primary"
                  onClick={() => setEditing({})}
                >
                  Add listing
                </button>
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
            <div className="form-grid">
              <label className="field">
                Type
                <select name="kind" defaultValue={editing.kind || "product"}>
                  <option value="product">Product</option>
                  <option value="service">Service</option>
                </select>
              </label>
              <label className="field">
                Price ({business.currency})
                <input
                  name="price"
                  type="number"
                  min="0"
                  max="1000000000"
                  step="0.01"
                  defaultValue={
                    editing.price_minor != null
                      ? (editing.price_minor / 100).toFixed(2)
                      : ""
                  }
                  required
                />
              </label>
            </div>
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
  const result = useResource(`/enquiries?page=${page}`);
  const [notice, setNotice] = useState("");
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
      />
      <Notice
        message={notice}
        kind={notice.startsWith("Customer added") ? "success" : "error"}
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
                    <td style={{ maxWidth: 300 }}>{e.message}</td>
                    <td>{date(e.created_at)}</td>
                    <td>
                      <div className="list-actions">
                        <button onClick={() => addCustomer(e)}>
                          Add customer
                        </button>
                        <Link
                          className="table-action"
                          href={`/app/invoices/new?name=${encodeURIComponent(e.name)}&email=${encodeURIComponent(e.email)}&phone=${encodeURIComponent(e.phone)}`}
                        >
                          Create invoice →
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
