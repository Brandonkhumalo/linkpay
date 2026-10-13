export async function GET() {
  try {
    const origin = process.env.API_INTERNAL_URL || "http://localhost:8080";
    const response = await fetch(`${origin}/healthz`, {
      cache: "no-store",
      signal: AbortSignal.timeout(3000),
    });
    if (response.ok) return Response.json({ status: "ok" });
  } catch {
    // Railway should report the whole container as unhealthy when the API is unavailable.
  }
  return Response.json({ status: "unavailable" }, { status: 503 });
}
