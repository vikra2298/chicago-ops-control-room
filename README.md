# Chicago Ops Control Room

A small operations console for a ride-hail marketplace operator. It answers **one** question:

**Where should we add driver supply in Chicago right now?**

The app combines a historical Chicago Transportation Network Provider (TNP) demand baseline with live Open-Meteo weather. It returns a single action (or an explicit hold), the reason, the evidence, and the ways that call can be wrong.

This is a functional prototype, not a dashboard.

## Judge this in 5 minutes

1. **Run** — `docker compose up --build` (or `.\run.ps1` / `make run`).
2. **Open** — [http://localhost:8080](http://localhost:8080). Click **Refresh**.
3. **Read the page** — one recommended action, historical demand vs average, live weather, why, and uncertainty.
4. **SQL** — ranking + city average live in `backend/internal/db/rank.go` (`RANK()`, `AVG()`).
5. **Tests** — `cd backend && go test ./...` (three flagship cases below).

## Run locally

**Docker (one command)**

```bash
docker compose up --build
```

**Go + Node**

```powershell
.\run.ps1
```

```bash
make run
```

Both paths build the React app and serve it from Go on port 8080. First start loads `backend/data/chicago_tnp_baselines.csv` into SQLite.

### Tests

```bash
cd backend && go test ./...
```

Flagship cases (volume is not the point):

| Test | What it proves |
|---|---|
| `TestRankDemandOrdersByTripCountAndComputesGap` | SQL ranking + gap vs city average |
| `TestDecideRecommendsWhenDemandFarAboveAverageAndWeatherIsBad` / `TestDecideHoldsWhenDemandIsCloseToAverage` | Act vs hold heuristic |
| `TestCurrentReturnsErrorWhenAPIFails` / `TestCurrentTimesOut` | Open-Meteo failure handling |

## What the operator sees

One recommendation page — not a multi-insight dashboard.

- **Recommended action** — e.g. “Reposition 8 idle drivers to Near North Side for the next 60 minutes,” or an explicit hold.
- **Historical expected demand** — trip baseline for the top ranked area vs the monitored-area average (from SQL).
- **Current weather (live)** — Open-Meteo Chicago observation, plus whether a weather adjustment was applied.
- **Why this action** — decision threshold, focus area, suggested drivers, and live weather details.
- **When this could be wrong** — limits of historical demand, coarse community areas, city-wide weather, and the driver-count heuristic.

The ranked top-N table is **not** shown in the UI (to avoid looking like a dashboard). Ranking still runs in SQL on the server to pick the single focus area.

## Architecture

```
React (TypeScript)  ──GET /api/recommendation──►  Go net/http
                                                  │
                                                  ├─ SQLite ranking query
                                                  │    community_areas
                                                  │    hourly_baselines
                                                  │
                                                  └─ Open-Meteo (Chicago)
                                                       cached in weather_snapshots
```

- **Go** standard library HTTP. No heavy framework.
- **SQLite** (pure Go driver, `modernc.org/sqlite`) so the evaluator does not need Postgres.
- **React + TypeScript** is a single page. Production builds are served by the Go process.
- **No auth, no map, no ML.** Extra surface area would not improve the one decision.

On each request the server:

1. Converts “now” to `America/Chicago`.
2. Runs a window-function SQL query that **ranks** community areas for that weekday and hour and compares each to the **city average**.
3. Fetches live weather. On failure it uses a snapshot less than 30 minutes old, otherwise it proceeds without weather and says so.
4. Applies a threshold heuristic and returns one decision.

## Data sources

| Source | Role |
|---|---|
| **Chicago TNP-style mobility baselines** | `backend/data/chicago_tnp_baselines.csv` — pre-aggregated hour-of-week trip demand by community area (public mobility dataset style; see [Chicago TNP trips](https://data.cityofchicago.org/Transportation/Transportation-Network-Providers-Trips/m6dm-c72p)) |
| **Open-Meteo** | Live Chicago weather (`temperature`, `precipitation`, `weather_code`) |

On first boot the CSV is loaded into SQLite (`community_areas` + `hourly_baselines`). Live weather is fetched from Open-Meteo on every recommendation request.

## Database design

Three tables (see `backend/internal/db/schema.sql`):

| Table | Role |
|---|---|
| `community_areas` | Official Chicago areas 1–77 |
| `hourly_baselines` | Expected trips and fare by area × weekday × hour (from CSV) |
| `weather_snapshots` | Successful live weather pulls, used as a short cache |

The running app does **not** store millions of raw TNP trips. Those rows would make local startup slow. The CSV is the mobility dataset artifact graders can inspect; SQL ranks it for the current Chicago hour.

### Important SQL

Ranking, city-wide average, and the gap vs average are computed in SQLite, not in Go:

```sql
WITH window_demand AS (
    SELECT
        ca.id,
        ca.name,
        hb.trip_count,
        hb.avg_fare_cents,
        AVG(hb.trip_count * 1.0) OVER () AS city_avg,
        RANK() OVER (ORDER BY hb.trip_count DESC) AS demand_rank
    FROM hourly_baselines hb
    JOIN community_areas ca ON ca.id = hb.community_area_id
    WHERE hb.weekday = ? AND hb.hour = ?
)
SELECT
    id, name, trip_count, avg_fare_cents, city_avg, demand_rank,
    trip_count - city_avg AS vs_city_avg,
    CAST(trip_count AS REAL) / NULLIF(city_avg, 0) AS demand_ratio
FROM window_demand
ORDER BY demand_rank ASC, name ASC;
```

Go only applies the weather-aware threshold to the already-ranked result.

## Recommendation heuristic

Reasonable operations assumptions, not a fitted model:

1. The area with rank 1 is the only candidate. The product is “the one action,” not a list of equally important zones.
2. Intervene only if `demand_ratio` exceeds a threshold:
   - **1.45×** city average in normal weather
   - **1.28×** when weather pressure is elevated (rain, heat, near-freezing)
   - **1.18×** when weather is severe (snow, thunderstorm, heavy precip, extreme temperature)
3. Suggested drivers = clamp(round((trips − city average) / 18), 8, 28).
4. If nothing crosses the line, **hold**, and still show the ranked evidence.

Weather pressure is derived from Open-Meteo WMO codes, precipitation, and temperature. Bad weather usually increases ride demand and keeps some drivers offline — that is why the threshold relaxes.

## Assumptions, trade-offs, and limitations

**Assumptions**

- A city / marketplace operator can reposition idle supply at community-area resolution in the next hour.
- Historical hour-of-week demand is a good enough prior when a live trip feed is not available.
- City-wide weather is a useful modifier, not a complete picture of street conditions.

**Trade-offs**

- **Pre-aggregated CSV instead of raw TNP rows.** Matches the brief’s mobility dataset requirement with a file you can open (`chicago_tnp_baselines.csv`), while keeping local startup fast. A production ETL would rebuild that CSV from the live Chicago TNP extract.
- **One recommendation instead of a dashboard.** Matches the brief. The operator does not have to pick the important row out of a chart.
- **SQLite instead of Postgres.** Zero-install local run. Window functions are enough for this query.
- **Heuristic instead of a marketplace model.** The brief allows it. A real system would need live supply, ETAs, and events.

**Known limitations**

- Community areas are coarse. A stadium pin or a subway stop can be the real hotspot.
- Special events are invisible.
- Weather is one Chicago point, not neighborhood radar.
- The driver count is a starting number, not a quota.
- If Open-Meteo is down and there is no recent snapshot, the call ignores live conditions and says so.

## API

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/recommendation` | The single decision payload |
| GET | `/api/health` | Process + database sanity |

## Screenshots

Captured from the running app:

- `docs/screenshots/01-recommendation.png` — single recommended action
- `docs/screenshots/02-evidence.png` — historical demand + live weather
- `docs/screenshots/03-why-action.png` — why this action (threshold, focus area, drivers)
- `docs/screenshots/04-uncertainty.png` — when this could be wrong

The page has two honest states: **Act now** when one area is far above baseline, and **Hold** when it is not. Hold is covered by the heuristic tests.

## AI-use disclosure

AI tools were used and encouraged for this exercise. This section is the record of that work.

### Tools and what they were used for

- **Cursor (Grok)** — project layout, Go server/SQL/tests, React page, Docker/README, and iteration on the recommendation copy.
- **No other codegen tools** were used for the submitted source.

### Suggestions that were accepted

- Keep the product to **one operator action**, with why / evidence / uncertainty on a single page.
- Use **SQLite + a pre-aggregated hour-of-week table** so local startup stays in the 8–10 hour spirit of the brief.
- Compute **RANK() and city average in SQL**, then apply a small weather threshold in Go.
- Serve the production frontend from the Go process so evaluators have a one-command path.

### Suggestion that was rejected

An early AI sketch stored **every raw trip row** and ranked areas in Go after a `SELECT *`.

That was rejected because:

1. The brief requires a meaningful **aggregation / comparison / ranking in SQL**.
2. Shipping or downloading a raw TNP extract would make “run this locally” fragile and slow.
3. Ranking in the application layer would hide the database design the interview is meant to discuss.

The accepted design is: seed (or later, ETL) into `hourly_baselines`, rank in SQLite, decide in Go.

### How the AI output was reviewed

- Read every generated file. Removed unused tables, extra endpoints, and dashboard-shaped UI.
- Added tests for the three failure/logic points that actually matter: SQL order, act-vs-hold, weather 502/timeout.
- Checked timezone handling (`America/Chicago` + `time/tzdata`) so Docker does not silently use UTC.
- Treated weather as optional: the API still returns a decision if Open-Meteo is down.
- Did not accept “deploy this to Kubernetes” or authentication. They do not improve the core decision.

Risks the tools introduced and that were corrected:

- Assuming CGO SQLite on Windows. Switched to `modernc.org/sqlite`.
- Assuming Node is always installed. Added Docker as the no-Node path.
- Over-confident recommendation copy. Uncertainty is first-class in the product, not only in this README.

### What was technically hardest

Turning a coarse historical baseline and a city-wide weather point into **one action an operator can take in the next hour**, without pretending the model can see live supply, events, or neighborhood radar. The threshold and the uncertainty list are the whole product judgment. The rest is plumbing.

## Follow-up interview notes

The live change that fits this codebase cleanly: adjust the weather threshold, add a weekday/hour override for replay, or swap the seed for a real TNP aggregation. The ranking SQL and `recommend.Decide` are the two places that encode the product.
