module easeemqtt

// go 1.25 verifiziert aus einem echten Install-Log (2026-09-07, real
// Hardware): Go's eigener Toolchain-Nachlademechanismus meldete
// "github.com/philippseith/signalr@v0.11.0 requires go >= 1.25.0", laedt
// go1.26.8 nach und baute damit erfolgreich - hier auf den tatsaechlich
// benoetigten Stand korrigiert statt bei der urspruenglich geschaetzten 1.23
// zu belassen.
go 1.25

// Versionen verifiziert gegen pkg.go.dev (Stand 2026-09-07), nicht geraten -
// siehe Plan-Datei/README fuer die Recherchequelle.
require (
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/eclipse/paho.mqtt.golang v1.5.1
	github.com/philippseith/signalr v0.11.0
)
