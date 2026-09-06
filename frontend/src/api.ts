import type { Recommendation } from "./types";

export async function fetchRecommendation(): Promise<Recommendation> {
  const res = await fetch("/api/recommendation");
  if (!res.ok) {
    throw new Error(`Recommendation request failed (${res.status})`);
  }
  return res.json();
}
