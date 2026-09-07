// easeemqtt: reine Protokollbruecke Easee Cloud (SignalR-Push + REST) <-> MQTT.
// Keine eigene Lastmanagement-Logik - die bleibt vollstaendig in Loxone, siehe
// Plan-Datei/README. Konfigurationsdatei ueber EASEEMQTT_CONFIG (Default:
// relativer Pfad "config.json", siehe systemd-Unit fuer den echten Pfad -
// analog zum Schwester-Plugin KNXtoLOX, dessen knx-mqtt-Engine denselben
// Env-Var-statt-Flag-Ansatz verwendet, siehe dessen postroot.sh/CHANGELOG).
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"easeemqtt/internal/bridge"
	"easeemqtt/internal/config"
	"easeemqtt/internal/easee"
)

func main() {
	log.SetFlags(log.LstdFlags)

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
