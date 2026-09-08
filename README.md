# LoxBerry-Plugin-EaseeMQTT

Reine Protokollbrücke zwischen Easee-Wallboxen und MQTT, für LoxBerry. 
Lastmanagement/Entscheidungslogik bleibt **komplett extern** (z.B. in Loxone) - dieser
Dienst übersetzt nur 1:1 zwischen Easees Cloud-API und MQTT, ohne eigene
Logik.

## Architektur

```
Easee Cloud (SignalR-Push + REST) <-> easeemqtt (Go, systemd: easeemqtt.service) <-> MQTT-Broker <-> Loxone
```
Der Go-Daemon (`daemon/`, aus dem mitgelieferten Quellcode gebaut, kein
externer `git clone` nötig):

1. Login + proaktiver Token-Refresh (5 Min vor Ablauf der 1h-Gültigkeit,
   zusätzlich Retry-mit-Refresh bei HTTP 401)
2. SignalR-Verbindung zu `https://streams.easee.com/hubs/chargers`,
   Auto-Reconnect mit moderatem Backoff (siehe Hinweis unten), Abonnement
   ALLER in `config.json` gelisteten Charger-IDs
3. Übersetzt empfangene Observation-Events -> retained MQTT-State-Topics
4. Abonniert MQTT-Kommando-Topics -> echte Easee-REST-Aufrufe (keine eigene
   Entscheidungslogik)

## MQTT-Topic-Schema

Präfix konfigurierbar (Default `easee/`). Alle Wallbox-Informationen landen
in MQTT, und die Wallboxen sind komplett steuerbar - Lastmanagement bleibt
trotzdem extern, der Daemon übersetzt nur.

**Status (Daemon -> Loxone, retained)** - `<prefix><charger_id>/state/<name>`,
für alle 170 bekannten Easee-Observation-IDs (siehe
`daemon/internal/bridge/bridge.go`'s `observationTopics`-Tabelle für die
vollständige, verbindliche Liste). Die alltagsrelevantesten:

```
<prefix><charger_id>/state/opmode                        Lademodus (numerischer Easee-Code)
<prefix><charger_id>/state/pilot_mode                     IEC-61851-Ladezustand (A-F)
<prefix><charger_id>/state/power_kw                       aktuelle Leistung
<prefix><charger_id>/state/session_energy_kwh             Energie dieser Ladesession
<prefix><charger_id>/state/lifetime_energy_kwh            Energie gesamt (Lebenszeit)
<prefix><charger_id>/state/cable_locked                   Kabel verriegelt (0/1)
<prefix><charger_id>/state/is_enabled                     Charger aktiviert (0/1)
<prefix><charger_id>/state/dynamic_current_a              aktuell erlaubtes Limit (flüchtig)
<prefix><charger_id>/state/max_current_a                  dauerhaftes Maximal-Limit
<prefix><charger_id>/state/output_phase                   1- oder 3-phasig aktiv
<prefix><charger_id>/state/derating_active / derated_current   thermische Drosselung
<prefix><charger_id>/state/error_code / fatal_error_code / error_string
<prefix><charger_id>/state/connected_to_cloud              Cloud-Verbindung der Box selbst
<prefix><charger_id>/state/current_t2 .. current_t5        Phasenströme (Rohbenennung, siehe Hinweis unten)
<prefix><charger_id>/state/in_volt_t1_t2 .. in_volt_t4_t5  Netzspannungen zwischen den Terminals (Rohbenennung)
<prefix><charger_id>/state/temp_max                        Temperaturüberwachung
... (alle weiteren 150+ IDs ebenso, siehe observationTopics in bridge.go)
<prefix>bridge/status   (online/offline, Last-Will des Daemons selbst)
```

**Kommandos (Loxone -> Daemon)** - komplettes Kommando-Set:

```
<prefix><charger_id>/cmd/start|stop|pause|resume|toggle|reboot|override_schedule   (beliebiger Payload löst aus)
<prefix><charger_id>/set/dynamic_current       Zahl in A - flüchtiges Limit, auf P1/P2/P3 symmetrisch angewandt
<prefix><charger_id>/set/max_current           Ganzzahl in A - dauerhaftes Limit
<prefix><charger_id>/set/enabled               0/1 - Charger komplett aktivieren/deaktivieren
<prefix><charger_id>/set/cable_lock            0/1 - Kabel dauerhaft verriegeln
<prefix><charger_id>/set/single_phase_limit    0/1 - auf 1-phasiges Laden begrenzen
<prefix><charger_id>/set/phase_mode            1=1-phasig fest, 2=Auto, 3=3-phasig fest
<prefix><charger_id>/set/smart_charging        0/1
<prefix><charger_id>/set/led_brightness        0-100
<prefix><charger_id>/set/idle_current          0/1 - Reststrom nach Ladeende signalisieren
```

Bool-Payloads akzeptieren großzügig `0/1`, `true/false` und `on/off` (siehe
`parseBool()` in `bridge.go`) - unabhängig davon, welcher
Loxone-MQTT-Output-Baustein verwendet wird.

**Hinweis `current_t2..t5`/`in_volt_t*_t*`**: die exakte Zuordnung
Easee-Terminal-Nummer -> Netzphase (L1/L2/L3) ist nicht gegen echte
Hardware verifiziert - deshalb bewusst Easees eigene Rohbenennung
übernommen statt eine möglicherweise falsche L1/L2/L3-Zuordnung
vorzutäuschen. Bei Bedarf anhand eines echten Mitschnitts während eines
laufenden Ladevorgangs korrekt ummappen.

## Konfiguration (`config.json`)

Eine einzige Datei, sowohl von der Web-UI (`api.cgi`) geschrieben als auch
vom Go-Daemon direkt gelesen:

```json
{
  "easee": { "username": "", "password": "", "chargers": [{"id": "", "name": ""}] },
  "mqtt": {
    "use_local_broker": true,
    "host": "", "port": 1883, "username": "", "password": "",
    "topic_prefix": "easee/", "client_id": "easeemqtt",
    "enabled_observations": [31, 47, 48, 100, 103, 109, 110, 115, 116, 119, 120, 121, 124, 150, 182, 183, 184, 185, 190, 191, 192, 193, 194, 195, 196, 197, 198, 199, 219, 250, 251]
  }
}
```

`use_local_broker=true`: `api.cgi` löst Host/Port/Zugangsdaten bei jedem
Speichern frisch aus LoxBerrys eigenem `general.json` auf und schreibt die
aufgelösten Werte in `config.json` - der Go-Daemon selbst kennt LoxBerry/
`general.json` nicht, bleibt dadurch als reine Protokollbrücke einfach.

`enabled_observations`: Positivliste der Observation-IDs, die als
MQTT-State-Topics publiziert werden (Kommando-Topics sind davon unberührt,
die sind immer alle aktiv) - konfigurierbar in der Web-UI ("Daten"-Seite,
Checkbox-Liste mit vorausgewähltem Basis-Setup). Fehlt der Schlüssel
komplett (z.B. eine sehr alte `config.json`), publiziert der Daemon
sicherheitshalber alles statt versehentlich nichts.

**Passwörter** (`easee.password`, `mqtt.password`) liegen verschlüsselt vor
(AES-256-GCM, Go-Standardbibliothek). Der Schlüssel wird bei der ersten
Nutzung zufällig erzeugt und liegt lokal neben `config.json` (`secret.key`,
Dateimodus 600) - er ist nicht Teil dieses Repos und wird nicht gesichert.
Das schützt vor versehentlicher Weitergabe nur der `config.json` (Screenshot,
Support-Anfrage, Backup ohne den Schlüssel), nicht vor jemandem mit vollem
Zugriff auf den Config-Ordner selbst. Die Web-UI zeigt gespeicherte
Passwörter nie im Klartext an - ein leer gelassenes Passwortfeld behält beim
Speichern automatisch den bestehenden Wert.

## Web-UI

Single-Page-App (`webfrontend/htmlauth/index.cgi` + `api.cgi`). Seiten:

- **Übersicht**: Dienst-Status, konfigurierte Wallboxen
- **Konto & Wallboxen**: Login-Test, "Ladegeräte suchen"-Picker gegen die
  echte Easee-API, editierbare Charger-Tabelle (beliebig viele Einträge)
- **MQTT**: lokaler oder externer Broker
- **Daten**: Checkbox-Liste aller 170 Observation-IDs nach Kategorie
  gruppiert, mit Suche und "Basis-Setup"/"Alle"/"Keine"-Schnellauswahl -
  steuert, welche Werte als MQTT-State-Topics landen
- **Loxone-Import**: generiert fertige Copy-Paste-Werte für LoxBerrys
  "MQTT Gateway"-Plugin (siehe unten) - wahlweise gefiltert auf eine
  einzelne Wallbox
- **Status**: Dienst-Status, Rohlog
- **Debug**: Web-UI-eigenes Log (Passwörter/Tokens werden vor dem Loggen
  redigiert)

### Loxone-Integration über LoxBerrys "MQTT Gateway"-Plugin

Diese Bridge verbindet sich selbst mit MQTT - Loxone braucht dafür ein
separates, bereits existierendes LoxBerry-Plugin ("MQTT Gateway"), das
zwischen MQTT-Broker und Miniserver vermittelt. Ablauf (Details:
[MQTT -> Loxone](https://wiki.loxberry.de/konfiguration/widget_help/widget_mqtt/mqtt_gateway/mqtt_schritt_fur_schritt_mqtt_loxone),
[Loxone -> MQTT](https://wiki.loxberry.de/konfiguration/widget_help/widget_mqtt/mqtt_gateway/mqtt_schritt_fur_schritt_loxone_mqtt)):

1. Im MQTT-Gateway-Plugin (Tab **Abonnements**) die gewünschten Easee-Topics
   einmalig abonnieren (kein API-Zugriff dafür vorhanden - manueller
   Schritt, die "Loxone-Import"-Seite liefert nur die danach benötigten
   Werte).
2. Auf der "Loxone-Import"-Seite Protokoll wählen, je nachdem was im
   MQTT-Gateway-Plugin eingestellt ist (beide werden von LoxBerry
   gleichwertig unterstützt). Unterschied: bei UDP reicht ein einziger
   Virtueller UDP-Eingang für beliebig viele Werte; bei HTTP braucht jeder
   Wert einen eigenen Virtuellen Eingang mit eigenen Berechtigungen (siehe
   LoxBerry-Wiki).
3. Generierte Werte in Loxone Config einfügen:
   - **State-Topics (UDP)**: je Zeile ein *Virtueller UDP Eingang Befehl*
     unter einem gemeinsamen *Virtuellen UDP Eingang* (Port = Gateway
     UDP-In-Port, Standard 11884) - "Befehlserkennung" = generierter
     `MQTT:\i<topic>=\i\v`-String.
   - **State-Topics (HTTP)**: je Zeile ein *Virtueller Eingang*, dessen
     "Bezeichnung" exakt dem generierten Namen entsprechen muss (Topic mit
     `/` -> `_`) - "Als Digitaleingang verwenden" nicht aktivieren.
   - **Kommandos**: ein gemeinsamer *Virtueller Ausgang* (Adresse
     `/dev/udp/<loxberry>/<Gateway-UDP-In-Port>`), darunter je Zeile ein
     *Virtueller Ausgang Befehl* mit den generierten `publish <topic>
     <wert>`-Strings als "Befehl bei EIN"/"Befehl bei AUS".

Die flachen Einzelwert-Topics (ein Wert pro Topic, kein gebündeltes JSON)
passen dem Gateway-Muster besonders gut - die im Wiki beschriebene
JSON-Aufteilung entfällt komplett.

## Installation

1. Plugin als ZIP über "Plugin installieren" hochladen.
2. `postroot.sh` baut `easeemqtt` aus dem mitgelieferten `daemon/`-Quellcode
   (`go mod tidy && go build`, braucht Internetzugriff zum Go-Modul-Proxy),
   richtet den systemd-Dienst ein.
3. Unter "Konto & Wallboxen": Easee-Zugangsdaten eintragen, testen,
   Ladegeräte suchen und der Tabelle hinzufügen.
4. Unter "MQTT": lokalen Broker bestätigen oder externen Broker eintragen.
5. Speichern - der Dienst startet automatisch mit der neuen Konfiguration
   neu.

## Bekannte offene Punkte

- `current_t2..t5`/`in_volt_t*_t*` -> L1/L2/L3-Zuordnung ist nicht final
  verifiziert (siehe Hinweis oben) - Easees eigene Terminal-Benennung wird
  bewusst unverändert übernommen.
- Der `streams.easee.com`-SignalR-Endpunkt wurde in einem öffentlichen
  GitHub-Thread im Zusammenhang mit einem IP-Ratelimit ("Blackhole")
  genannt - der Daemon verwendet deshalb bewusst moderates Backoff statt
  aggressiver Reconnect-Versuche.
