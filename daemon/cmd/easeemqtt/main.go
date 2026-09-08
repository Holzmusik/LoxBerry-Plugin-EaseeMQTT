// easeemqtt: reine Protokollbruecke Easee Cloud (SignalR-Push + REST) <-> MQTT.
// Keine eigene Lastmanagement-Logik - die bleibt vollstaendig in Loxone, siehe
// Plan-Datei/README. Konfigurationsdatei ueber EASEEMQTT_CONFIG (Default:
// relativer Pfad "config.json", siehe systemd-Unit fuer den echten Pfad -
// analog zum Schwester-Plugin KNXtoLOX, dessen knx-mqtt-Engine denselben
// Env-Var-statt-Flag-Ansatz verwendet, siehe dessen postroot.sh/CHANGELOG).
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"easeemqtt/internal/bridge"
	"easeemqtt/internal/config"
	"easeemqtt/internal/easee"
)

// runCrypto bedient "easeemqtt encrypt <config-pfad>" / "easeemqtt decrypt
// <config-pfad>" - liest den zu (ent-)schluesselnden Wert von STDIN statt als
// Kommandozeilenargument, damit ein Passwort nie kurzzeitig in der
// Prozessliste (ps aux) sichtbar wird. api.cgi ruft dies als Subprocess auf,
// damit die eigentliche Kryptografie an EINER Stelle (hier, mit Gos
// Standardbibliothek) lebt statt zusaetzlich in Perl dupliziert zu werden.
func runCrypto(mode, cfgPath string) {
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimRight(input, "\r\n")

	var out string
	var err error
	switch mode {
	case "encrypt":
		out, err = config.Encrypt(cfgPath, input)
	case "decrypt":
		var ok bool
		out, ok = config.Decrypt(cfgPath, input)
		if !ok {
			err = fmt.Errorf("kein gueltiges Chiffrat")
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "easeemqtt "+mode+": "+err.Error())
		os.Exit(1)
	}
	fmt.Println(out)
}

// credentialRetryDelay: Abstand zwischen zwei Login-Versuchen, NACHDEM
// Easee die Zugangsdaten explizit abgelehnt hat (HTTP 400/401). Bewusst
// lang (5 Minuten) statt sich auf systemds RestartSec=5 zu verlassen - ein
// falsches Passwort behebt sich nicht durch schnelles Neuversuchen, und
// dichte Login-Versuche im Sekundentakt sind bei Cloud-Auth-APIs oft genau
// der Trigger fuer temporaere Account-Sperren (siehe ErrInvalidCredentials-
// Kommentar in auth.go).
const credentialRetryDelay = 5 * time.Minute

// loginWithCredentialBackoff versucht Login(), bis es entweder klappt oder
// ctx endet (SIGINT/SIGTERM). Ein transienter Fehler (Netzwerk, Easee-Cloud
// 5xx) wird SOFORT nach oben durchgereicht - main() beendet den Prozess
// dann per log.Fatalf, und systemds Restart=on-failure/RestartSec=5 greift
// wie gehabt (dafuer ist dieser schnelle Restart-Loop gedacht: transiente
// Fehler sollen sich schnell von selbst loesen). NUR bei
// easee.ErrInvalidCredentials wird HIER, im Prozess selbst, mit
// credentialRetryDelay weitergewartet und erneut versucht, statt den
// Prozess ueberhaupt zu beenden - genau das verhindert den Login-Hammer bei
// dauerhaft falschem Passwort.
func loginWithCredentialBackoff(ctx context.Context, auth *easee.Auth) error {
	for {
		err := auth.Login(ctx)
		if err == nil {
			return nil
		}
		if !errors.Is(err, easee.ErrInvalidCredentials) {
			return err
		}
		log.Printf("Easee-Login abgelehnt - bitte Benutzername/Passwort in der Web-UI pruefen: %v", err)
		log.Printf("naechster Login-Versuch in %s", credentialRetryDelay)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(credentialRetryDelay):
		}
	}
}

func main() {
	log.SetFlags(log.LstdFlags)

	if len(os.Args) >= 3 && (os.Args[1] == "encrypt" || os.Args[1] == "decrypt") {
		runCrypto(os.Args[1], os.Args[2])
		return
	}

	cfgPath := os.Getenv("EASEEMQTT_CONFIG")
	if cfgPath == "" {
		cfgPath = "config.json"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatalf("Konfiguration konnte nicht geladen werden: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	auth := easee.NewAuth(cfg.Easee.Username, cfg.Easee.Password)
	if err := loginWithCredentialBackoff(ctx, auth); err != nil {
		// Nur hier (ctx.Done(), also SIGINT/SIGTERM waehrend des Wartens)
		// oder ein echter, nicht-klassifizierbarer Fehler kommt zurueck -
		// ErrInvalidCredentials selbst fuehrt NIE hierher, siehe
		// loginWithCredentialBackoff().
		log.Fatalf("Easee-Login fehlgeschlagen: %v", err)
	}
	go auth.RunBackgroundRefresh(ctx)

	restClient := easee.NewClient(auth)

	br := bridge.New(cfg, restClient)
	if err := br.Connect(); err != nil {
		log.Fatalf("MQTT-Verbindung fehlgeschlagen: %v", err)
	}
	defer br.Disconnect()

	chargerIDs := make([]string, 0, len(cfg.Easee.Chargers))
	for _, ch := range cfg.Easee.Chargers {
		chargerIDs = append(chargerIDs, ch.ID)
	}
	hub := easee.NewHub(auth, chargerIDs, br.HandleObservation)

	log.Printf("easeemqtt gestartet - %d Charger, MQTT-Prefix %q", len(chargerIDs), cfg.MQTT.TopicPrefix)

	if err := hub.Run(ctx); err != nil && ctx.Err() == nil {
		// ctx.Err() == nil bedeutet: Run() ist NICHT wegen regulaerem Shutdown
		// (SIGINT/SIGTERM) zurueckgekehrt, sondern wegen eines echten Fehlers
		// (z.B. Client-Erstellung fehlgeschlagen) - systemd startet den Dienst
		// dank Restart=on-failure (siehe systemd-Unit) automatisch neu.
		log.Fatalf("SignalR-Hub beendet: %v", err)
	}
	log.Println("easeemqtt beendet")
}
