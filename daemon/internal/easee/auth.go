// Package easee kapselt die komplette Easee-Cloud-Anbindung: OAuth2-Login +
// proaktiver Token-Refresh (auth.go), REST-Kommandos/State (rest.go) und den
// SignalR-Push-Kanal (signalr.go).
//
// Alle Endpunkt-URLs/Feldnamen in diesem Package sind gegen echte Quellen
// verifiziert (developer.easee.com, den echten evcc-io/evcc-Produktionscode
// unter charger/easee/, und die pyeasee-Python-Bibliothek) - siehe Plan-Datei
// fuer die einzelnen Belege. Was NICHT verifiziert werden konnte, ist explizit
// als solches markiert (siehe signalr.go) statt geraten.
package easee

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// ErrInvalidCredentials wird von Login() zurueckgegeben, wenn der
// Login-Endpoint einen klaren Credential-Fehler meldet (HTTP 400/401) - im
// Unterschied zu einem transienten Fehler (Netzwerk weg, Easee-Cloud 5xx)
// rechtfertigt das KEINEN schnellen Restart-Loop durch systemd: ein falsches
// Passwort behebt sich nicht durch Warten und erneutes Versuchen, und
// wiederholte Login-Versuche im Sekundentakt sind bei den meisten Cloud-
// Auth-APIs genau der Trigger fuer temporaere Account-Sperren. main.go
// behandelt diesen Fall deshalb bewusst separat (langer Retry-Abstand statt
// Prozess-Crash), siehe dortiger Kommentar.
var ErrInvalidCredentials = errors.New("easee auth: Zugangsdaten abgelehnt (HTTP 400/401 vom Login-Endpoint)")

// APIBase ist verifiziert gegen den echten evcc-io/evcc-Produktionscode
// (const API = "https://api.easee.com/api" im Package charger/easee).
const APIBase = "https://api.easee.com/api"

// refreshMargin: proaktiver Refresh, bevor der Token wirklichablaeuft -
// vermeidet, dass eine gerade laufende SignalR-Verbindung oder ein REST-Call
// mitten in der Nutzung auf einen abgelaufenen Token trifft. Easees
// accessToken ist laut developer.easee.com/docs/authentication-1 1h (3600s)
// gueltig.
const refreshMargin = 5 * time.Minute

type tokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int    `json:"expiresIn"`
	TokenType    string `json:"tokenType"`
}

// Auth haelt den aktuellen Token-Satz und sorgt fuer Thread-sicheren,
// proaktiven Refresh. Wird von rest.go UND signalr.go gemeinsam benutzt -
// beide rufen vor jeder Anfrage AccessToken() auf statt selbst zu verwalten.
type Auth struct {
	username, password string
	httpClient         *http.Client

	mu           sync.Mutex
	accessToken  string
	refreshToken string
	expiresAt    time.Time
}

func NewAuth(username, password string) *Auth {
	return &Auth{
		username:   username,
		password:   password,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// Login authentifiziert einmalig mit Benutzername/Passwort. Body-Feldnamen
// (userName/password) und Response-Feldnamen (accessToken/refreshToken/
// expiresIn/tokenType) verifiziert gegen developer.easee.com/reference/
// account_authenticate.
func (a *Auth) Login(ctx context.Context) error {
	body, _ := json.Marshal(map[string]string{
		"userName": a.username,
		"password": a.password,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, APIBase+"/accounts/login", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return a.doTokenRequest(req, true)
}

// refresh erneuert den Token ueber den refreshToken, OHNE erneut Benutzername/
// Passwort zu senden. Pfad/Feldname (refreshToken) verifiziert gegen
// developer.easee.com/reference/account_refreshtoken.
func (a *Auth) refresh(ctx context.Context) error {
	a.mu.Lock()
	rt := a.refreshToken
	a.mu.Unlock()
	if rt == "" {
		return a.Login(ctx)
	}
	body, _ := json.Marshal(map[string]string{"refreshToken": rt})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, APIBase+"/accounts/refresh_token", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	// classifyAuthErrors=false: ein 400/401 hier bedeutet nur "refreshToken
	// abgelaufen/ungueltig", NICHT zwingend "Benutzername/Passwort falsch" -
	// die eigentliche Klassifizierung passiert erst im Login()-Fallback
	// direkt darunter, der bei echten Credential-Fehlern seinerseits
	// ErrInvalidCredentials liefert.
	if err := a.doTokenRequest(req, false); err != nil {
		// Refresh fehlgeschlagen (z.B. refreshToken invalidiert) - kompletter
		// Neu-Login als Fallback, statt dauerhaft haengen zu bleiben.
		return a.Login(ctx)
	}
	return nil
}

func (a *Auth) doTokenRequest(req *http.Request, classifyAuthErrors bool) error {
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("easee auth: request fehlgeschlagen: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		if classifyAuthErrors && (resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized) {
			return fmt.Errorf("%w (HTTP %d: %s)", ErrInvalidCredentials, resp.StatusCode, string(data))
		}
		return fmt.Errorf("easee auth: HTTP %d: %s", resp.StatusCode, string(data))
	}
	var tr tokenResponse
	if err := json.Unmarshal(data, &tr); err != nil {
		return fmt.Errorf("easee auth: Antwort nicht parsbar: %w", err)
	}
	if tr.AccessToken == "" {
		return fmt.Errorf("easee auth: keine accessToken in der Antwort")
	}
	a.mu.Lock()
	a.accessToken = tr.AccessToken
	a.refreshToken = tr.RefreshToken
	expiresIn := tr.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 3600
	}
	a.expiresAt = time.Now().Add(time.Duration(expiresIn) * time.Second)
	a.mu.Unlock()
	return nil
}

// AccessToken liefert einen garantiert (noch mindestens refreshMargin lang)
// gueltigen Access-Token, refresht bei Bedarf proaktiv/synchron.
func (a *Auth) AccessToken(ctx context.Context) (string, error) {
	a.mu.Lock()
	needsRefresh := a.accessToken == "" || time.Now().After(a.expiresAt.Add(-refreshMargin))
	a.mu.Unlock()
	if needsRefresh {
		if err := a.refresh(ctx); err != nil {
			return "", err
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.accessToken, nil
}

// ForceRefresh wird von rest.go nach einem 401 aufgerufen (Token evtl. vom
// Server vorzeitig invalidiert, obwohl unsere lokale Ablaufzeit noch gueltig
// aussah) - einmaliger Retry-Pfad, keine Endlosschleife.
func (a *Auth) ForceRefresh(ctx context.Context) error {
	return a.refresh(ctx)
}

// RunBackgroundRefresh haelt den Token dauerhaft frisch, auch waehrend eine
// lange laufende SignalR-Verbindung selbst keinen neuen AccessToken()-Aufruf
// ausloest (der Token wird nur beim SignalR-Verbindungsaufbau/-Handshake
// gebraucht, siehe signalr.go) - ohne diesen Hintergrund-Ticker wuerde ein
// erst NACH vielen Stunden noetiger SignalR-Reconnect auf einen laengst
// abgelaufenen Token treffen.
func (a *Auth) RunBackgroundRefresh(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := a.AccessToken(ctx); err != nil {
				// Naechster Tick versucht es erneut; REST-/SignalR-Aufrufer
				// bekommen den Fehler ohnehin direkt bei ihrem eigenen
				// AccessToken()-Aufruf zu sehen und loggen ihn dort.
				continue
			}
		}
	}
}
