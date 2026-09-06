export type AreaStat = {
  id: number;
  name: string;
  trip_count: number;
  avg_fare_cents: number;
  city_avg: number;
  demand_rank: number;
  vs_city_avg: number;
  demand_ratio: number;
};

export type WeatherInfo = {
  temp_c: number;
  precipitation_mm: number;
  wind_kmh: number;
  weather_code: number;
  label: string;
  pressure: string;
  source: string;
  fetched_at: string;
};

export type Recommendation = {
  action_needed: boolean;
  severity: "hold" | "moderate" | "high" | string;
  headline: string;
  action: string;
  why: string;
  why_facts: string[];
  focus: AreaStat | null;
  areas: AreaStat[];
  city_avg_trips: number;
  threshold_used: number;
  suggested_drivers: number;
  window: {
    weekday: number;
    weekday_name: string;
    hour: number;
    label: string;
    timezone: string;
  };
  weather: WeatherInfo | null;
  weather_available: boolean;
  weather_note: string;
  uncertainty: string[];
  generated_at: string;
};
