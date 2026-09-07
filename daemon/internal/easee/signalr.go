package easee

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/philippseith/signalr"
)

// HubURL verifiziert gegen den echten evcc-io/evcc-Produktionscode
// (charger/easee.go: "https://streams.easee.com/hubs/chargers", Stand
// 2026-09-07). ACHTUNG (siehe Plan-Datei): ein GitHub-Discussion-Thread
// (evcc-io/evcc #29776, Mai 2026) zeigt, dass Easee bei zu aggressivem
// Reconnect-Verhalten IPs zeitweise in einen "Blackhole" schickt
// (blackhole.easee.com) - laut Easee-Mitarbeiter ein IP-Ratelimit, KEINE
// URL-Aenderung. Deshalb hier bewusst kein aggressiver Backoff, siehe
// newBackoff().
const HubURL = "https://streams.easee.com/hubs/chargers"

// Observation ist ein einzelnes, dekodiertes Easee-SignalR-Event.
type Observation struct {
	ChargerID string
	ID        int
	DataType  int
	Value     string // immer als String weitergereicht, Interpretation liegt bei bridge.go
	Timestamp time.Time
}

// Hub haelt die Verbindung zu Easees SignalR-Hub aufrecht (Auto-Reconnect
// via philippseith/signalr's WithConnector/WithBackoff, siehe Run()) und
// meldet jedes empfangene Observation-Event an onObservation.
type Hub struct {
	auth          *Auth
	chargerIDs    []string
	onObservation func(Observation)
	httpClient    *http.Client
}

func NewHub(auth *Auth, chargerIDs []string, onObservation func(Observation)) *Hub {
	return &Hub{
		auth:          auth,
		chargerIDs:    chargerIDs,
		onObservation: onObservation,
		httpClient:    &http.Client{Timeout: 30 * time.Second},
	}
}

// receiver implementiert die Server->Client-Callback-Methoden, die
// philippseith/signalr per Reflection anhand des Invocation-Target-Namens
// aufruft. Methodennamen (ProductUpdate/ChargerUpdate/SubscribeToMyProduct/
// CommandResponse) UND das eingebettete signalr.Hub sind gegen den echten
// evcc-io/evcc-Produktionscode verifiziert (derselbe Hub, dieselbe API) -
// ein falscher Name wuerde ein Event still verwerfen statt einen Fehler zu
// werfen, deshalb hier bewusst NICHT geraten.
type receiver struct {
	signalr.Hub
	hub *Hub
}

func (r *receiver) ProductUpdate(raw json.RawMessage)        { r.hub.handleUpdate(raw) }
func (r *receiver) ChargerUpdate(raw json.RawMessage)        { r.hub.handleUpdate(raw) }
func (r *receiver) SubscribeToMyProduct(raw json.RawMessage) {}
func (r *receiver) CommandResponse(raw json.RawMessage)      {}

// rawObservation bildet das von Easee gesendete JSON ab. Feldname fuer die
// Charger-ID KORRIGIERT (2026-09-07, per echtem MQTT-Mitschnitt gefunden):
// urspruenglich als "chargerId" geraten, tatsaechlich aber "Mid" - verifiziert
// gegen den echten evcc-io/evcc-Sourcecode (dessen `easee.Observation`-Struct
// hat GAR KEINE JSON-Tags: `Mid string; DataType DataType; ID ObservationID;
// Timestamp time.Time; Value string` - Go matcht ohne Tag automatisch
// case-insensitiv gegen den exportierten Feldnamen, das JSON-Feld heisst also
// "Mid"/"mid"). id/dataType/value trafen davor zufaellig trotzdem, weil deren
// Namen sich nur in der Gross-/Kleinschreibung von "Id"/"DataType"/"Value"
// unterschieden (von Go's case-insensitivem Fallback-Matching abgedeckt) -
// nur "chargerId" hatte mit "Mid" gar keine Aehnlichkeit und blieb deshalb
// immer leer. Symptom war: alle Charger kollidierten auf DEMSELBEN MQTT-Topic
// (leere ChargerID -> "<prefix>/state/..." statt "<prefix><id>/state/..."),
// dadurch ueberschrieben sich mehrere Wallboxen gegenseitig statt eigene
// Topic-Aeste zu bekommen.
type rawObservation struct {
	ChargerID string          `json:"mid"`
	ID        int             `json:"id"`
	DataType  int             `json:"dataType"`
	Value     json.RawMessage `json:"value"`
}

func (h *Hub) handleUpdate(raw json.RawMessage) {
	var ro rawObservation
	if err := json.Unmarshal(raw, &ro); err != nil {
		log.Printf("easee signalr: Observation nicht parsbar: %v (raw=%s)", err, string(raw))
		return
	}
	if h.onObservation == nil {
		return
	}
	h.onObservation(Observation{
		ChargerID: ro.ChargerID,
		ID:        ro.ID,
		DataType:  ro.DataType,
		Value:     rawJSONToString(ro.Value),
		Timestamp: time.Now(),
	})
}

// rawJSONToString funktioniert unabhaengig davon, ob "value" als JSON-String
// ("123") oder als roher JSON-Wert (123, true) gesendet wird - beides wurde
// in unterschiedlichen Easee-Client-Bibliotheken beobachtet, hier bewusst
// beide Faelle abgedeckt statt eine Form vorauszusetzen.
func rawJSONToString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

// connect ist der "Connector" fuer philippseith/signalr's WithConnector -
// wird bei JEDEM (Re-)Verbindungsaufbau neu aufgerufen, holt sich deshalb bei
// jedem Aufruf frisch einen (proaktiv von auth.go verwalteten) gueltigen
// Token. Authorization-Header-Uebergabe via WithHTTPHeaders verifiziert
// gegen den echten evcc-io/evcc-Code (identisches Muster).
func (h *Hub) connect(ctx context.Context) (signalr.Connection, error) {
	token, err := h.auth.AccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("easee signalr: kein gueltiger Token: %w", err)
	}
	negotiateCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	return signalr.NewHTTPConnection(negotiateCtx, HubURL,
		signalr.WithHTTPClient(h.httpClient),
		signalr.WithHTTPHeaders(func() http.Header {
			return http.Header{"Authorization": []string{"Bearer " + token}}
		}),
	)
}

// newBackoff: bewusst OHNE MaxElapsedTime-Limit. Der Standard von
// backoff.NewExponentialBackOff() gibt nach 15 Minuten auf (liefert
// backoff.Stop) - ein WithConnector-basierter Client bleibt danach dauerhaft
// in ClientClosed haengen, ohne von selbst je wieder zu verbinden. evcc-io/
// evcc kommentiert genau dieses Verhalten explizit als Grund fuer ihre eigene
// "no maximum elapsed time"-Einstellung im selben Code - hier bewusst
// uebernommen, nicht geraten. InitialInterval/MaxInterval moderat (nicht
// aggressiv) gewaehlt wegen des "Blackhole"-Ratelimit-Risikos, siehe
// HubURL-Kommentar.
func newBackoff() backoff.BackOff {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = 2 * time.Second
	b.MaxInterval = 2 * time.Minute
	b.MaxElapsedTime = 0
	return b
}

// Run baut den Client auf und haelt ihn am Laufen (Auto-Reconnect inklusive),
// bis ctx endet. Bei jedem Uebergang nach ClientConnected werden alle
// konfigurierten Charger-IDs neu abonniert - SubscribeWithCurrentState ist
// verbindungsgebundener Server-Zustand, kein dauerhaftes Server-Setting
// (Invoke-Methodenname UND das zweite true-Argument "inkl. aktuellem Zustand"
// aus dem echten evcc-io/evcc-Aufruf uebernommen, dessen genaue
// Argumentbedeutung nicht unabhaengig re-verifiziert wurde).
func (h *Hub) Run(ctx context.Context) error {
	c, err := signalr.NewClient(ctx,
		signalr.WithConnector(func() (signalr.Connection, error) { return h.connect(ctx) }),
		signalr.WithBackoff(newBackoff),
		signalr.WithReceiver(&receiver{hub: h}),
	)
	if err != nil {
		return fmt.Errorf("easee signalr: Client konnte nicht erstellt werden: %w", err)
	}

	stateCh := make(chan signalr.ClientState, 4)
	cancelObserve := c.ObserveStateChanged(stateCh)
	defer cancelObserve()

	go func() {
		for state := range stateCh {
			if state != signalr.ClientConnected {
				continue
			}
			log.Printf("easee signalr: verbunden, abonniere %d Charger", len(h.chargerIDs))
			for _, id := range h.chargerIDs {
				if serr := <-c.Send("SubscribeWithCurrentState", id, true); serr != nil {
					log.Printf("easee signalr: Abonnieren von %s fehlgeschlagen: %v", id, serr)
				}
			}
		}
	}()

	c.Start()
	defer c.Stop()

	<-ctx.Done()
	return ctx.Err()
}
