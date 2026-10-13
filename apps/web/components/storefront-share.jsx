"use client";

import Image from "next/image";
import { useEffect, useState } from "react";
import QRCode from "qrcode";

export default function StorefrontShare({ slug, name }) {
  const [url, setURL] = useState("");
  const [qr, setQR] = useState("");
  const [notice, setNotice] = useState("");
  useEffect(() => {
    const link = `${window.location.origin}/store/${slug}`;
    QRCode.toDataURL(link, { width: 240, margin: 2, errorCorrectionLevel: "M" })
      .then((image) => {
        setURL(link);
        setQR(image);
      })
      .catch(() =>
        setNotice("QR code unavailable. You can still share the link."),
      );
  }, [slug]);
  const message = `Explore ${name} on LinkPay: ${url}`;
  async function copyMessage() {
    try {
      await navigator.clipboard.writeText(message);
      setNotice("Share message copied.");
    } catch {
      setNotice(
        "Could not copy. Open the storefront and share its address instead.",
      );
    }
  }
  return (
    <section className="panel">
      <span className="eyebrow">SHARE YOUR STOREFRONT</span>
      <h2>Give people a quick way in.</h2>
      {qr && (
        <Image
          unoptimized
          src={qr}
          width={240}
          height={240}
          alt={`QR code for ${name} storefront`}
        />
      )}
      <p className="helper">Scan to open {url || "your storefront"}.</p>
      {notice && (
        <p role="status" className="helper">
          {notice}
        </p>
      )}
      <div className="form-actions">
        <button className="button button-light" onClick={copyMessage}>
          Copy share message
        </button>
        {url && (
          <a
            className="button button-light"
            href={`https://wa.me/?text=${encodeURIComponent(message)}`}
            target="_blank"
            rel="noopener noreferrer"
          >
            Share on WhatsApp ↗
          </a>
        )}
        {qr && (
          <a
            className="button button-light"
            href={qr}
            download={`${slug}-qr.png`}
          >
            Download QR code
          </a>
        )}
      </div>
    </section>
  );
}
