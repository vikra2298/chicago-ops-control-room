import { useEffect, useState } from "react";
import { fetchRecommendation } from "./api";
import type { Recommendation, WeatherInfo } from "./types";
import "./App.css";

function formatNumber(n: number): string {
  return new Intl.NumberFormat("en-US", { maximumFractionDigits: 0 }).format(n);
}

function formatRatio(n: number): string {
  return `${n.toFixed(1)}×`;
}

export default function App() {
  const [data, setData] = useState<Recommendation | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshState, setRefreshState] = useState<"idle" | "loading" | "done">("idle");

  async function load(fromRefresh = false) {
    try {
      setError(null);
      if (fromRefresh) {
        setRefreshState("loading");
      }
      const next = await fetchRecommendation();
      setData(next);
      if (fromRefresh) {
        setRefreshState("done");
        window.setTimeout(() => setRefreshState("idle"), 2000);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "Could not load recommendation");
      if (fromRefresh) {
        setRefreshState("idle");
      }
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    void load();
    const id = window.setInterval(() => void load(), 60_000);
    return () => window.clearInterval(id);
  }, []);

  return (
    <div className="page">
      <header className="topbar">
        <div>
          <p className="kicker">Chicago marketplace · live decision</p>
          <h1>Operations Control Room</h1>
        </div>
        <div className="refresh-wrap">
          <button
            type="button"
            className="refresh"
            disabled={refreshState === "loading"}
            onClick={() => void load(true)}
          >
            {refreshState === "loading" ? "Refreshing…" : "Refresh"}
          </button>
          {refreshState === "loading" ? (
            <span className="refresh-hint">Fetching latest…</span>
          ) : null}
          {refreshState === "done" ? (
            <span className="refresh-hint ok">Updated</span>
          ) : null}
        </div>
      </header>

      {loading && !data ? <p className="status">Finding the one action that matters…</p> : null}
      {error ? <p className="status error">{error}</p> : null}

      {data ? <RecommendationView data={data} /> : null}
    </div>
  );
}

function RecommendationView({ data }: { data: Recommendation }) {
  const tone = data.action_needed ? data.severity : "hold";
  const focus = data.focus;
  const statusTag = weatherStatusTag(data);
  const conditionTag = weatherConditionTag(data);
  const insight = whyWeatherInsight(data);

  return (
    <main>
      <section className={`hero tone-${tone}`}>
        <p className="pill">{data.action_needed ? "Do this now" : "No action needed"}</p>
        <h2>{data.headline}</h2>
        <p className="action">{data.action}</p>
        <p className="window">
          {data.window.label} · {data.window.timezone}
        </p>
      </section>

      <section className="signal-grid">
        <article className="signal-card">
          <p className="signal-label">Historical expected demand</p>
          <p className="signal-value">
            {focus ? `${formatNumber(focus.trip_count)} expected trips` : "No baseline"}
          </p>
          <p className="signal-meta">
            {focus
              ? `${focus.name} · #${focus.demand_rank} priority · ${formatRatio(focus.demand_ratio)} the monitored-area average`
              : "No community area ranked for this hour"}
          </p>
        </article>

        <article className="signal-card">
          <p className="signal-label">Current weather (live)</p>
          <p className="signal-value">{weatherHeadline(data)}</p>
          {statusTag ? (
            <span className={`weather-tag tone-${statusTag.tone}`}>{statusTag.label}</span>
          ) : null}
          <p className="signal-meta">{weatherDetails(data)}</p>
        </article>
      </section>

      <section className="panel evidence-card">
        <h3>Why this action</h3>
        <div className="weather-insight-row">
          {conditionTag ? (
            <span className={`weather-tag tone-${conditionTag.tone}`}>{conditionTag.label}</span>
          ) : null}
          <p className="weather-insight">{insight}</p>
        </div>
        <div className="metrics">
          <Metric
            label="Monitored-area average"
            value={`${formatNumber(data.city_avg_trips)} trips`}
          />
          <Metric
            label="Decision threshold"
            value={`${Math.round(data.threshold_used * 100)}% of average`}
          />
          <Metric
            label="Focus area"
            value={focus ? `#${focus.demand_rank} ${focus.name}` : "—"}
          />
          <Metric
            label="Suggested drivers"
            value={data.action_needed ? String(data.suggested_drivers) : "0"}
          />
        </div>
        <p className="why-note">
          One recommendation for this hour — based on expected demand vs the
          monitored-area average.
        </p>
      </section>

      <section className="panel caution">
        <h3>When this could be wrong</h3>
        <ul>
          {data.uncertainty.map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ul>
      </section>

      <footer>
        Generated {new Date(data.generated_at).toLocaleString()} · Chicago TNP
        hour-of-week baselines + Open-Meteo. One recommendation only.
      </footer>
    </main>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="metric">
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function weatherHeadline(data: Recommendation): string {
  if (!data.weather_available || !data.weather) {
    return "Unavailable";
  }
  const w = data.weather;
  return `${w.label}, ${Math.round(w.temp_c)}°C`;
}

function weatherStatusTag(data: Recommendation): { label: string; tone: string } | null {
  if (!data.weather_available || !data.weather) {
    return { label: "Weather offline", tone: "missing" };
  }
  switch (data.weather.pressure) {
    case "severe":
      return { label: "Weather-urgent", tone: "severe" };
    case "elevated":
      return { label: "Rain-adjusted", tone: "elevated" };
    default:
      return { label: "Demand-only signal", tone: "normal" };
  }
}

function weatherConditionTag(data: Recommendation): { label: string; tone: string } | null {
  if (!data.weather_available || !data.weather) {
    return { label: "Weather unavailable", tone: "missing" };
  }
  const w = data.weather;
  const temp = `${Math.round(w.temp_c)}°C`;
  switch (w.pressure) {
    case "severe":
      return { label: `Severe weather · ${temp}`, tone: "severe" };
    case "elevated":
      return { label: elevatedConditionLabel(w, temp), tone: "elevated" };
    default:
      return { label: `${w.label} · ${temp}`, tone: "normal" };
  }
}

function elevatedConditionLabel(w: WeatherInfo, temp: string): string {
  const rainy =
    w.precipitation_mm > 0 ||
    /rain|drizzle|shower|thunder/i.test(w.label);
  if (rainy) {
    return `Elevated rain · ${temp}`;
  }
  return `Elevated · ${w.label} · ${temp}`;
}

function weatherDetails(data: Recommendation): string {
  if (!data.weather_available || !data.weather) {
    return data.weather_note || "No live weather — historical demand only";
  }
  const w = data.weather;
  const observed = w.fetched_at
    ? `Observed ${new Date(w.fetched_at).toLocaleString()}`
    : "Live observation";
  return `${w.precipitation_mm.toFixed(1)} mm precip · ${Math.round(w.wind_kmh)} km/h wind · ${w.source} · ${observed}`;
}

function whyWeatherInsight(data: Recommendation): string {
  if (!data.weather_available || !data.weather) {
    return "Weather insight: live weather unavailable";
  }
  switch (data.weather.pressure) {
    case "severe":
      return "Weather insight: severe weather raising urgency";
    case "elevated":
      return "Weather insight: mild rain pressure";
    default:
      return "Weather insight: no weather adjustment";
  }
}
