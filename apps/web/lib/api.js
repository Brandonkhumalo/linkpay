export class ApiError extends Error {
  constructor(message, status) {
    super(message);
    this.status = status;
  }
}

export async function api(path, options = {}) {
  const headers = {
    ...(options.body instanceof FormData
      ? {}
      : { "Content-Type": "application/json" }),
    ...options.headers,
  };
  const response = await fetch(`/api/v1${path}`, {
    credentials: "same-origin",
    ...options,
    headers,
  });
  if (response.status === 204) return null;
  const data = await response.json().catch(() => null);
  if (!response.ok)
    throw new ApiError(
      data?.error || "Something went wrong. Please try again.",
      response.status,
    );
  return data;
}

export function money(minor, currency = "USD") {
  return `${currency} ${(Number(minor || 0) / 100).toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
}

export function date(value) {
  if (!value) return "—";
  const parsed = new Date(value);
  return Number.isNaN(parsed.getTime())
    ? String(value)
    : parsed.toLocaleDateString("en-ZW", {
        day: "numeric",
        month: "short",
        year: "numeric",
        ...(/^\d{4}-\d{2}-\d{2}$/.test(value) ? { timeZone: "UTC" } : {}),
      });
}

export function errorMessage(error) {
  return error instanceof Error
    ? error.message
    : "Something went wrong. Please try again.";
}

export function pageNumber(value) {
  const parsed = Number(value);
  return Number.isInteger(parsed) && parsed > 0 && parsed <= 10000 ? parsed : 1;
}

export function notificationDestination(kind) {
  if (kind === "enquiry") return "/app/enquiries";
  if (kind === "followup") return "/app/followups";
  if (kind.startsWith("quote")) return "/app/quotes";
  return "/app/invoices";
}
