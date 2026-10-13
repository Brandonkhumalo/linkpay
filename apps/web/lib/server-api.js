import { notFound } from "next/navigation";

export async function publicApi(path) {
  const origin = process.env.API_INTERNAL_URL || "http://localhost:8080";
  const response = await fetch(`${origin}/api/v1${path}`, {
    cache: "no-store",
  });
  if (response.status === 404) notFound();
  if (!response.ok) throw new Error("This page is temporarily unavailable.");
  return response.json();
}
