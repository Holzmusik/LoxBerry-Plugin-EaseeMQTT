module easeemqtt

// go 1.25 verifiziert aus einem echten Install-Log (2026-09-07, real
// Hardware): Go's eigener Toolchain-Nachlademechanismus meldete
// "github.com/philippseith/signalr@v0.11.0 requires go >= 1.25.0", laedt
// go1.26.8 nach und baute damit erfolgreich - hier auf den tatsaechlich
// benoetigten Stand korrigiert statt bei der urspruenglich geschaetzten 1.23
// zu belassen.
go 1.25.0

// Versionen verifiziert gegen pkg.go.dev (Stand 2026-09-07), nicht geraten -
// siehe Plan-Datei/README fuer die Recherchequelle.
require (
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/eclipse/paho.mqtt.golang v1.5.1
	github.com/philippseith/signalr v0.11.0
)

require github.com/go-kit/log v0.2.1

require (
	github.com/coder/websocket v1.8.15 // indirect
	github.com/dunglas/httpsfv v1.1.0 // indirect
	github.com/go-logfmt/logfmt v0.6.1 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	github.com/quic-go/quic-go v0.61.0 // indirect
	github.com/quic-go/webtransport-go v0.12.0 // indirect
	github.com/teivah/onecontext v1.3.0 // indirect
	github.com/vmihailenco/msgpack/v5 v5.4.1 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	golang.org/x/crypto v0.54.0 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
)
