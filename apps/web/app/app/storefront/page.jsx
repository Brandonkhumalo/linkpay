"use client";

import Link from "next/link";
import { useState } from "react";
import { api, errorMessage } from "../../../lib/api";
import { Badge, Icon, Notice, PageHeader } from "../../../components/ui";
import { useWorkspace } from "../layout";

export default function Storefront() {
  const { business, setBusiness } = useWorkspace();
  const [notice, setNotice] = useState(""),
    [busy, setBusy] = useState(false);
  const link = `/store/${business.slug}`;
  async function save(e) {
    e.preventDefault();
    setBusy(true);
    setNotice("");
    const data = new FormData(e.currentTarget);
    const body = Object.fromEntries(data);
    body.currency = String(body.currency).toUpperCase();
    body.published = data.has("published");
    try {
      const saved = await api("/storefront", {
        method: "PATCH",
        body: JSON.stringify(body),
      });
      setBusiness(saved);
      setNotice("Storefront saved.");
    } catch (error) {
      setNotice(errorMessage(error));
    } finally {
      setBusy(false);
    }
  }
  async function copy() {
    try {
      await navigator.clipboard.writeText(`${window.location.origin}${link}`);
      setNotice("Storefront link copied.");
    } catch {
      setNotice(
        "Could not copy the link. Open the storefront and copy its address instead.",
      );
    }
  }
  return (
    <>
      <PageHeader
        eyebrow="YOUR DIGITAL FRONT DOOR"
        title="Your storefront"
        description="Your shop is optional. Publish it when you're ready for customers to find you."
        action={
          business.published ? (
            <Link
              className="button button-light"
              href={`/store/${business.slug}`}
              target="_blank"
            >
              Preview storefront <Icon name="external" size={16} />
            </Link>
          ) : null
        }
      />
      <Notice
        message={notice}
        kind={
          notice.includes("saved") || notice.includes("copied")
            ? "success"
            : "error"
        }
      />
      <div className="page-columns">
        <section className="panel">
          <div className="panel-head">
            <h2>Business profile</h2>
            <Badge>{business.published ? "active" : "draft"}</Badge>
          </div>
          <p>The details customers will see on your public page.</p>
          <form onSubmit={save}>
            <label className="field">
              Business name
              <input
                name="name"
                required
                minLength={2}
                maxLength={120}
                defaultValue={business.name}
              />
            </label>
            <label className="field">
              Description
              <textarea
                name="description"
                maxLength={2000}
                defaultValue={business.description}
                placeholder="Tell customers what makes your business special"
              />
            </label>
            <div className="form-grid">
              <label className="field">
                Contact email
                <input
                  name="email"
                  type="email"
                  defaultValue={business.email}
                />
              </label>
              <label className="field">
                WhatsApp number
                <input
                  name="whatsapp"
                  placeholder="+263..."
                  inputMode="tel"
                  pattern="\+?[1-9][0-9]{6,14}"
                  defaultValue={business.whatsapp}
                />
                <span className="helper">
                  Required to publish. Include your country code, for example
                  +263771234567. Customers will open a chat with this number.
                </span>
              </label>
              <label className="field">
                Currency code
                <input
                  name="currency"
                  required
                  maxLength={3}
                  pattern="[A-Za-z]{3}"
                  defaultValue={business.currency}
                />
              </label>
              <label className="field">
                Timezone
                <input
                  name="timezone"
                  required
                  defaultValue={business.timezone}
                />
              </label>
            </div>
            <label className="field">
              Brand colour
              <input
                name="theme_color"
                type="color"
                defaultValue={business.theme_color}
              />
            </label>
            <h3 style={{ fontSize: 15, marginTop: 25 }}>Social links</h3>
            <p className="helper">
              Use full HTTPS links. Customers can visit them from your
              storefront.
            </p>
            <div className="form-grid">
              <label className="field">
                Instagram
                <input
                  name="instagram_url"
                  type="url"
                  defaultValue={business.instagram_url}
                  placeholder="https://www.instagram.com/..."
                />
              </label>
              <label className="field">
                Facebook
                <input
                  name="facebook_url"
                  type="url"
                  defaultValue={business.facebook_url}
                  placeholder="https://www.facebook.com/..."
                />
              </label>
              <label className="field">
                TikTok
                <input
                  name="tiktok_url"
                  type="url"
                  defaultValue={business.tiktok_url}
                  placeholder="https://www.tiktok.com/..."
                />
              </label>
              <label className="field">
                Website
                <input
                  name="website_url"
                  type="url"
                  defaultValue={business.website_url}
                  placeholder="https://..."
                />
              </label>
            </div>
            <label className="field checkbox-field">
              <input
                name="published"
                type="checkbox"
                defaultChecked={business.published}
              />{" "}
              Publish storefront
            </label>
            <p className="helper">
              Published shops appear in the public directory. Keep this off if
              you only use invoices.
            </p>
            <button className="button button-dark" disabled={busy}>
              {busy ? "Saving…" : "Save storefront"}
            </button>
          </form>
        </section>
        <aside className="stack">
          <section className="panel">
            <span className="eyebrow">YOUR SHAREABLE LINK</span>
            <h2>Put your business out there.</h2>
            <p>
              {business.published
                ? "Share this on WhatsApp, social media, or your business card."
                : "Publish your shop to make this link available to customers."}
            </p>
            <div className="preview-card">
              <span className="preview-link">{link}</span>
            </div>
            <div className="form-actions" style={{ marginTop: 16 }}>
              {business.published && (
                <button className="button button-primary" onClick={copy}>
                  Copy link
                </button>
              )}
              {business.published && (
                <Link
                  className="button button-light"
                  href={`/store/${business.slug}`}
                  target="_blank"
                >
                  Open ↗
                </Link>
              )}
            </div>
          </section>
          <section className="panel" style={{ background: "#eff5e8" }}>
            <h2>
              {business.published
                ? "Your storefront is live"
                : "Ready to go live?"}
            </h2>
            <p>
              {business.published
                ? "Customers can view your listings and send enquiries."
                : "Turn on Publish storefront and save to make your page visible."}
            </p>
          </section>
        </aside>
      </div>
    </>
  );
}
