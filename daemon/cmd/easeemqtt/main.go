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
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

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
	if err := auth.Login(ctx); err != nil {
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
