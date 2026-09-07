package easee

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client fuehrt authentifizierte REST-Aufrufe gegen die Easee Cloud API aus.
// Kommando-Pfade verifiziert gegen pyeasee/charger.py (nordicopen/pyeasee,
// echter Produktionscode einer weit genutzten Python-Bibliothek fuer dieselbe
// API) und developer.easee.com/docs/api-command-and-control.
type Client struct {
	auth       *Auth
	httpClient *http.Client
}

func NewClient(auth *Auth) *Client {
	return &Client{auth: auth, httpClient: &http.Client{Timeout: 15 * time.Second}}
}

// doJSON fuehrt eine authentifizierte Anfrage aus und retryt EINMAL nach
// einem 401 mit erzwungenem Token-Refresh (der Server kann einen Token auch
// vor der von uns lokal berechneten Ablaufzeit invalidieren) - keine
// Endlosschleife, maximal ein Retry pro Aufruf.
func (c *Client) doJSON(ctx context.Context, method, path string, body any, out any) error {
	doOnce := func() (*http.Response, []byte, error) {
		token, err := c.auth.AccessToken(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("easee rest: kein gueltiger Token: %w", err)
		}
		var reqBody io.Reader
		if body != nil {
			b, err := json.Marshal(body)
			if err != nil {
				return nil, nil, err
			}
			reqBody = bytes.NewReader(b)
		}
		req, err := http.NewRequestWithContext(ctx, method, APIBase+path, reqBody)
		if err != nil {
			return nil, nil, err
		}
		req.Header.Set("Authorization", "Bearer "+token)
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, nil, fmt.Errorf("easee rest: %s %s fehlgeschlagen: %w", method, path, err)
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		return resp, data, nil
	}

	resp, data, err := doOnce()
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		if rerr := c.auth.ForceRefresh(ctx); rerr == nil {
			resp, data, err = doOnce()
			if err != nil {
				return err
			}
		}
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("easee rest: %s %s: HTTP %d: %s", method, path, resp.StatusCode, string(data))
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("easee rest: %s %s: Antwort nicht parsbar: %w", method, path, err)
		}
	}
	return nil
}

// ChargerInfo ist die (gekuerzte) Antwortform von GET /chargers - fuer die
// "Ladegeraete suchen"-Funktion in der Web-UI (api.cgi ruft dies NICHT direkt
// auf, siehe dortiger eigener Perl-Login/-Liste-Code; hier fuer den Daemon
// selbst nicht zwingend benoetigt, aber als Hilfsfunktion fuer spaetere
// Erweiterung/Debug-CLI bereitgehalten).
type ChargerInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (c *Client) ListChargers(ctx context.Context) ([]ChargerInfo, error) {
	var out []ChargerInfo
	err := c.doJSON(ctx, http.MethodGet, "/chargers", nil, &out)
	return out, err
}

func (c *Client) StartCharging(ctx context.Context, chargerID string) error {
	return c.doJSON(ctx, http.MethodPost, "/chargers/"+chargerID+"/commands/start_charging", nil, nil)
}

func (c *Client) StopCharging(ctx context.Context, chargerID string) error {
	return c.doJSON(ctx, http.MethodPost, "/chargers/"+chargerID+"/commands/stop_charging", nil, nil)
}

func (c *Client) PauseCharging(ctx context.Context, chargerID string) error {
	return c.doJSON(ctx, http.MethodPost, "/chargers/"+chargerID+"/commands/pause_charging", nil, nil)
}

func (c *Client) ResumeCharging(ctx context.Context, chargerID string) error {
	return c.doJSON(ctx, http.MethodPost, "/chargers/"+chargerID+"/commands/resume_charging", nil, nil)
}

// SetDynamicCurrent setzt das dynamische Strom-Limit (A) - der Hebel, ueber
// den Loxones Lastmanagement die Ladeleistung steuert.
//
// KORRIGIERT 2026-09-07 (Bug gefunden beim Live-Debugging: Wert kam im
// MQTT-Log korrekt an, in der Easee-App blieb das Limit aber bei 0). Ursache:
// {currentP1,currentP2,currentP3} ist das Body-Schema des CIRCUIT-Endpoints
// (set_dynamic_charger_circuit_current, Site-/Circuit-Ebene fuer mehrere
// Boxen an einem Circuit). Der hier aufgerufene CHARGER-Endpoint
// (.../chargers/{id}/commands/set_dynamic_charger_current) erwartet
// stattdessen {"amps": int, "minutes": int} - verifiziert gegen
// pyeasee/charger.py (nordicopen/pyeasee, set_dynamic_charger_current()) UND
// die offizielle Referenz developer.easee.com/reference/
// charger_set_dynamic_charger_current. Mit den falschen Feldnamen nahm die
// Easee-Cloud den Request ohne HTTP-Fehler an, ignorierte aber die
// unbekannten Felder - daher kein Fehler im Log, aber wirkungslos.
// minutes=0 bedeutet "bis zur naechsten Aenderung gueltig" (kein TTL).
func (c *Client) SetDynamicCurrent(ctx context.Context, chargerID string, amps float64) error {
	body := map[string]any{
		"amps":    amps,
		"minutes": 0,
	}
	return c.doJSON(ctx, http.MethodPost, "/chargers/"+chargerID+"/commands/set_dynamic_charger_current", body, nil)
}

// ── Restliches Kommando-Set fuer "komplett steuerbar" ───────────────────────
// Alle folgenden Pfade/Feldnamen verifiziert gegen den echten Quellcode von
// pyeasee/charger.py (nordicopen/pyeasee), nicht geraten.

func (c *Client) ToggleCharging(ctx context.Context, chargerID string) error {
	return c.doJSON(ctx, http.MethodPost, "/chargers/"+chargerID+"/commands/toggle_charging", nil, nil)
}

func (c *Client) Reboot(ctx context.Context, chargerID string) error {
	return c.doJSON(ctx, http.MethodPost, "/chargers/"+chargerID+"/commands/reboot", nil, nil)
}

// OverrideSchedule laesst den Charger einen aktiven Ladeplan (siehe
// CHARGING_SCHEDULE/BASIC_CHARGE_PLAN in Easees eigener App) fuer die
// aktuelle Session ignorieren und sofort laden - relevant nur, falls in der
// Easee-App selbst ein Zeitplan hinterlegt ist.
func (c *Client) OverrideSchedule(ctx context.Context, chargerID string) error {
	return c.doJSON(ctx, http.MethodPost, "/chargers/"+chargerID+"/commands/override_schedule", nil, nil)
}

// SetMaxCurrent setzt das NICHT-fluechtige Maximal-Limit (Observation 47,
// "MAX_CHARGER_CURRENT") - anders als SetDynamicCurrent (fluechtig, wird bei
// Neustart des Chargers zurueckgesetzt) bleibt dieser Wert dauerhaft
// gespeichert. Amperewert wird 1:1 fuer alle Phasen gesetzt (Easee kennt hier
// nur einen einzelnen Wert, kein P1/P2/P3-Tripel wie bei set_dynamic_current).
func (c *Client) SetMaxCurrent(ctx context.Context, chargerID string, amps int) error {
	return c.doJSON(ctx, http.MethodPost, "/chargers/"+chargerID+"/settings", map[string]int{"maxChargerCurrent": amps}, nil)
}

// SetEnabled schaltet den Charger komplett ein/aus (Observation 31,
// "IS_ENABLED") - staerkerer Eingriff als pause/resume (ein deaktivierter
// Charger nimmt gar keine Ladebefehle mehr an).
func (c *Client) SetEnabled(ctx context.Context, chargerID string, enabled bool) error {
	return c.doJSON(ctx, http.MethodPost, "/chargers/"+chargerID+"/settings", map[string]bool{"enabled": enabled}, nil)
}

// LockCablePermanently sperrt/entsperrt das Ladekabel dauerhaft am Typ-2-
// Stecker (Observation 30, "LOCK_CABLE_PERMANENTLY") - eigener commands/-
// Endpunkt, kein /settings.
func (c *Client) LockCablePermanently(ctx context.Context, chargerID string, lock bool) error {
	return c.doJSON(ctx, http.MethodPost, "/chargers/"+chargerID+"/commands/lock_state", map[string]bool{"state": lock}, nil)
}

// SetSinglePhaseLimit erzwingt 1-phasiges Laden (Observation 34/38-Umfeld).
func (c *Client) SetSinglePhaseLimit(ctx context.Context, chargerID string, enabled bool) error {
	return c.doJSON(ctx, http.MethodPost, "/chargers/"+chargerID+"/settings", map[string]bool{"limitToSinglePhaseCharging": enabled}, nil)
}

// SetPhaseMode setzt den Phasenmodus (Observation 38, "PHASE_MODE"):
// 1=fest 1-phasig, 2=Auto, 3=fest 3-phasig (nur Home-Modelle) - Bedeutung
// laut Kommentar im echten evcc-Enum, nicht unabhaengig re-verifiziert.
func (c *Client) SetPhaseMode(ctx context.Context, chargerID string, mode int) error {
	return c.doJSON(ctx, http.MethodPost, "/chargers/"+chargerID+"/settings", map[string]int{"phaseMode": mode}, nil)
}

// SetSmartCharging steuert das "Smart Charging"-Flag (Observation 102).
func (c *Client) SetSmartCharging(ctx context.Context, chargerID string, enabled bool) error {
	return c.doJSON(ctx, http.MethodPost, "/chargers/"+chargerID+"/settings", map[string]bool{"smartCharging": enabled}, nil)
}

// SetLedStripBrightness setzt die LED-Streifen-Helligkeit 0-100%
// (Observation 40, "LED_STRIP_BRIGHTNESS").
func (c *Client) SetLedStripBrightness(ctx context.Context, chargerID string, brightness int) error {
	return c.doJSON(ctx, http.MethodPost, "/chargers/"+chargerID+"/settings", map[string]int{"ledStripBrightness": brightness}, nil)
}

// SetIdleCurrentEnabled steuert, ob der Charger nach Ladeende weiterhin
// minimalen Strom signalisiert (Observation 37, "ENABLE_IDLE_CURRENT").
func (c *Client) SetIdleCurrentEnabled(ctx context.Context, chargerID string, enabled bool) error {
	return c.doJSON(ctx, http.MethodPost, "/chargers/"+chargerID+"/settings", map[string]bool{"enableIdleCurrent": enabled}, nil)
}
