package weather

// WMO weather interpretation codes used by Open-Meteo.
func Label(code int) string {
	switch {
	case code == 0:
		return "Clear"
	case code <= 3:
		return "Partly cloudy"
	case code == 45 || code == 48:
		return "Fog"
	case code >= 51 && code <= 57:
		return "Drizzle"
	case code >= 61 && code <= 67:
		return "Rain"
	case code >= 71 && code <= 77:
		return "Snow"
	case code >= 80 && code <= 82:
		return "Rain showers"
	case code == 85 || code == 86:
		return "Snow showers"
	case code >= 95:
		return "Thunderstorm"
	default:
		return "Mixed conditions"
	}
}

// Pressure classifies how weather typically stresses ride-hail supply
// (fewer drivers willing to work) and lifts demand (more people avoid walking/transit).
func Pressure(code int, precipMM, tempC float64) string {
	severe := code >= 71 && code <= 77 ||
		code == 85 || code == 86 ||
		code >= 95 ||
		precipMM >= 2.0 ||
		tempC <= -10 ||
		tempC >= 35
	if severe {
		return "severe"
	}
	elevated := code >= 51 && code <= 67 ||
		code >= 80 && code <= 82 ||
		precipMM >= 0.2 ||
		tempC <= 0 ||
		tempC >= 32
	if elevated {
		return "elevated"
	}
	return "normal"
}
