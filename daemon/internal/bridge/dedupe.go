package bridge

import (
	"math"
	"strconv"
	"strings"
	"sync"
	"time"
)

// valueKind bestimmt, wie Soll- (MQTT-Payload) und Ist-Werte (Observation)
// normalisiert werden, damit sie als String vergleichbar sind.
type valueKind int

const (
	kindBool valueKind = iota
	kindInt
	kindFloat
)

// observedSetting ordnet eine Easee-Observation dem set/*-Topic zu, dessen
// Wirkung sie zurueckmeldet. Damit kann die Bridge den von Loxone gewollten
// Wert mit dem tatsaechlichen Zustand der Wallbox vergleichen (siehe
// sendGate). IDs/Namen aus observationTopics.
var observedSetting = map[int]struct {
	set  string
	kind valueKind
}{
	30:  {"cable_lock", kindBool},     // lock_cable_permanently
	31:  {"enabled", kindBool},        // is_enabled
	37:  {"idle_current", kindBool},   // enable_idle_current
	38:  {"phase_mode", kindInt},      // phase_mode
	40:  {"led_brightness", kindInt},  // led_strip_brightness
	47:  {"max_current", kindInt},     // max_current_a
	48:  {"dynamic_current", kindInt}, // dynamic_current_a
	102: {"smart_charging", kindBool}, // smart_charging
}

// normalize bringt einen Wert in die Form, in der die set/*-Handler ihn
// vergleichen: Bool "true"/"false", Int ganzzahlig, Float mit 2
// Nachkommastellen. ok=false bei nicht interpretierbarem Wert.
func normalize(kind valueKind, raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	switch kind {
	case kindBool:
		v, err := parseBool(s)
		if err != nil {
			return "", false
		}
		return strconv.FormatBool(v), true
	case kindInt:
		f, err := strconv.ParseFloat(s, 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return "", false
		}
		return strconv.Itoa(int(math.Round(f))), true
	case kindFloat:
		f, err := strconv.ParseFloat(s, 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
			return "", false
		}
		return strconv.FormatFloat(f, 'f', 2, 64), true
	}
	return "", false
}

const (
	// sentTTL: so lange gilt ein erfolgreich gesendeter Wert als "erledigt".
	// Danach wird derselbe Wert wieder durchgelassen - Rueckfallebene fuer
	// Einstellungen ohne Observation und falls eine Observation ausbleibt.
	sentTTL = 10 * time.Minute
	// observeGrace: nach dem Senden so lange warten, bevor eine abweichende
	// Observation als "Wallbox hat den Wert nicht (mehr)" gewertet wird - die
	// Cloud meldet die Aenderung erst mit Verzoegerung zurueck.
	observeGrace = 60 * time.Second
)

type sentEntry struct {
	value string
	at    time.Time
}

// sendGate entscheidet, ob ein set/*-Wert an die Easee-Cloud geht.
// Hintergrund: Loxone schreibt Sollwerte (v.a. dynamic_current) oft im
// Sekundentakt neu, auch unveraendert - ohne Sperre waere jede Nachricht ein
// REST-Call (Rate-Limit-Risiko). Frueher wurde NUR gegen den zuletzt
// gesendeten Wert verglichen; startete die Wallbox neu, setzte sie ihren
// (fluechtigen) Dynamic Current zurueck, Loxone schickte denselben Wert wie
// vorher, und die Bridge verwarf ihn fuer immer. Jetzt wird zusaetzlich gegen
// den beobachteten Ist-Wert verglichen, und Eintraege verfallen nach sentTTL.
//
// Schluessel ist das set/*-Topic (Charger-ID + Einstellung in einem).
type sendGate struct {
	mu       sync.Mutex
	sent     map[string]sentEntry
	observed map[string]string
}

func newSendGate() *sendGate {
	return &sendGate{sent: map[string]sentEntry{}, observed: map[string]string{}}
}

// shouldSend: true, wenn value gesendet werden soll; reason fuer das Log.
func (g *sendGate) shouldSend(key, value string, now time.Time) (bool, string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	e, ok := g.sent[key]
	if !ok {
		return true, "erster Wert"
	}
	if e.value != value {
		return true, "geaendert"
	}
	if now.Sub(e.at) >= sentTTL {
		return true, "zuletzt vor mehr als 10 min gesendet"
	}
	if obs, ok := g.observed[key]; ok && obs != value && now.Sub(e.at) >= observeGrace {
		return true, "Wallbox meldet " + obs
	}
	return false, ""
}

// markSent nur NACH einem erfolgreichen REST-Call - ein fehlgeschlagener Call
// darf den Wert nicht als erledigt verbuchen.
func (g *sendGate) markSent(key, value string, now time.Time) {
	g.mu.Lock()
	g.sent[key] = sentEntry{value: value, at: now}
	g.mu.Unlock()
}

func (g *sendGate) observe(key, value string) {
	g.mu.Lock()
	g.observed[key] = value
	g.mu.Unlock()
}
