#!/bin/bash
# postroot.sh wird bei Installation/Update mit Root-Rechten ausgefuehrt.
# Parameter, die LoxBerry uebergibt: $1=TempFolder $2=Version $3=Base $4=PluginName $5=... (siehe LoxBerry SDK)

# PSCRIPTFOLDER ist das Wurzelverzeichnis des entpackten Plugin-Archivs
# (postroot.sh liegt dort direkt neben config/, templates/, daemon/, ... als
# Geschwister - gleiches, bereits bei KNXtoLOX real-hardware-verifiziertes
# Muster, siehe dessen postroot.sh).
PSCRIPTFOLDER=$(dirname "$0")

BINDIR="/opt/loxberry/bin/plugins/easeemqtt"
CFGDIR="/opt/loxberry/config/plugins/easeemqtt"
LOGDIR="/opt/loxberry/log/plugins/easeemqtt"
SYSTEMDUNIT="/etc/systemd/system/easeemqtt.service"

mkdir -p "$BINDIR" "$CFGDIR" "$LOGDIR"
chown loxberry.loxberry "$LOGDIR" 2>/dev/null || true

# ── Go-Toolchain sicherstellen ──────────────────────────────────────────────
# Gleiches Muster wie KNXtoLOX/postroot.sh (dort real-hardware-verifiziert:
# go.dev/dl und der Go-Modul-Proxy sind von der LoxBerry-Box aus erreichbar,
# der Build lief dort erfolgreich durch). Debians golang-go-Paket ist oft
# aelter als 1.21 (die Version, ab der Go selbststaendig die in go.mod per
# "go 1.23"-Zeile geforderte neuere Version nachladen kann) - deshalb hier
# zuerst pruefen und bei Bedarf das offizielle Go-Tarball direkt installieren.
NEED_GO_INSTALL=1
if command -v go &>/dev/null; then
  GO_VER="$(go version | grep -oE 'go[0-9]+\.[0-9]+' | head -1 | sed 's/go//')"
  GO_MAJOR="${GO_VER%%.*}"
  GO_MINOR="${GO_VER##*.}"
  if [ "${GO_MAJOR:-0}" -gt 1 ] || { [ "${GO_MAJOR:-0}" -eq 1 ] && [ "${GO_MINOR:-0}" -ge 21 ]; }; then
    NEED_GO_INSTALL=0
  fi
fi

if [ "$NEED_GO_INSTALL" = "1" ]; then
  echo "Kein ausreichend aktuelles Go gefunden (>=1.21 fuer automatischen Toolchain-Nachlad noetig) - installiere offizielles Go-Tarball..."
  case "$(dpkg --print-architecture)" in
    arm64) GOTAR_ARCH="arm64" ;;
    armhf) GOTAR_ARCH="armv6l" ;;
    amd64) GOTAR_ARCH="amd64" ;;
    *) GOTAR_ARCH="arm64" ;;
  esac
  GO_BOOTSTRAP_VERSION="go1.24.3"
  curl -fsSL "https://go.dev/dl/${GO_BOOTSTRAP_VERSION}.linux-${GOTAR_ARCH}.tar.gz" -o /tmp/go.tar.gz \
    && rm -rf /usr/local/go \
    && tar -C /usr/local -xzf /tmp/go.tar.gz \
    && rm -f /tmp/go.tar.gz
  export PATH="/usr/local/go/bin:$PATH"
fi

if ! command -v go &>/dev/null; then
  echo "FEHLER: Go-Compiler konnte nicht installiert werden - easeemqtt kann nicht gebaut werden." >&2
  exit 1
fi
echo "Go-Version: $(go version)"

# ── Bauen aus dem MITGELIEFERTEN Quellcode (daemon/) ────────────────────────
# Anders als bei KNXtoLOX's knx-mqtt (fremdes Repo, per git clone geholt):
# unser eigener Go-Sourcecode liegt bereits im Plugin-Zip auf der Platte -
# kein git clone/externe Repo-URL noetig, das vermeidet die ganze "erfundene
# Release-URL/falscher Clone-Pfad"-Fehlerklasse von vornherein.
#
# WICHTIG: NICHT direkt in $PSCRIPTFOLDER/daemon bauen (erste Version dieses
# Skripts tat das) - postroot.sh laeuft als root, "go mod tidy" schreibt dabei
# go.mod/go.sum um und der Go-Build-Cache legt Dateien an, alles als root, in
# genau dem temporaeren Upload-Verzeichnis, das LoxBerrys eigener Installer
# danach selbst wieder aufraeumt (als weniger privilegierter Nutzer). Auf
# echter Hardware beobachtet: der Aufraeum-Schritt scheiterte reihenweise mit
# "Permission denied" an genau den von go beruehrten Dateien, weil die dem
# Aufraeum-Prozess gehoerende Temp-Kopie durch den root-Build nachtraeglich
# root-owned wurde. Fix: in eine EIGENE, root-eigene Scratch-Kopie bauen, die
# wir selbst (als root, also garantiert erfolgreich) wieder entfernen - die
# vom Installer verwaltete Temp-Kopie unter uploads/ bleibt dabei komplett
# unangetastet.
echo "Baue easeemqtt aus $PSCRIPTFOLDER/daemon (in einer root-eigenen Kopie) ..."
BUILD_DIR="$(mktemp -d)"
cp -r "$PSCRIPTFOLDER/daemon/." "$BUILD_DIR/"
(
  cd "$BUILD_DIR" || exit 1
  export CGO_ENABLED=0
  # go mod tidy statt nur "go build": es liegt bewusst KEIN vorgefertigtes
  # go.sum im Plugin-Zip (dessen Prüfsummen könnten ohne echten Go-Build auf
  # dem Dev-Rechner nicht verifiziert werden) - go mod tidy loest die in
  # go.mod geforderten Versionen auf, laedt sie vom Go-Modul-Proxy und
  # erzeugt ein korrektes go.sum, bevor gebaut wird.
  go mod tidy && go build -o "$BINDIR/easeemqtt" ./cmd/easeemqtt
)
BUILD_RC=$?
rm -rf "$BUILD_DIR"

if [ $BUILD_RC -ne 0 ] || [ ! -x "$BINDIR/easeemqtt" ]; then
  echo "FEHLER: Bauen des easeemqtt-Binaries fehlgeschlagen." >&2
  exit 1
fi

# Default-Config anlegen, falls noch keine existiert (wird spaeter ueber Web-UI ueberschrieben)
if [ ! -f "$CFGDIR/config.json" ]; then
  cp "$PSCRIPTFOLDER/config/config.json.default" "$CFGDIR/config.json"
fi
# postroot.sh laeuft als root - api.cgi (User "loxberry") muss config.json
# spaeter beim Speichern ueberschreiben koennen (gleiches Muster/gleicher
# Bug wie bei KNXtoLOX, dort erst nachtraeglich gefunden - hier von Anfang an
# mit drin).
chown -R loxberry.loxberry "$CFGDIR" 2>/dev/null || true

# systemd-Unit installieren
cp "$PSCRIPTFOLDER/templates/system/etc/systemd/system/easeemqtt.service" "$SYSTEMDUNIT"
systemctl daemon-reload
systemctl enable easeemqtt.service
systemctl restart easeemqtt.service

echo "Easee-MQTT Plugin Installation abgeschlossen."
exit 0
