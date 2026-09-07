// Package config liest die einzige Config-Datei des Daemons (config.json).
// Bewusst EINE Datei, UI-editierbar UND direkt vom Daemon gelesen - anders als
// beim Schwester-Plugin KNXtoLOX (config.yaml/bridge.json-Split), das nur
// wegen YAML::Tiny-Unzuverlässigkeit bei verschachtelten Strukturen nötig war.
// JSON::PP (Perl) <-> encoding/json (Go) ist ein robustes, symmetrisches Paar,
// kein Split noetig.
package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// Charger ist ein einzelner, in der Web-UI ausgewaehlter Easee-Charger.
// Die Liste ist bewusst nicht auf eine feste Laenge begrenzt.
type Charger struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type EaseeConfig struct {
	Username string    `json:"username"`
	Password string    `json:"password"`
	Chargers []Charger `json:"chargers"`
}

// MQTTConfig traegt IMMER die tatsaechlich zu verwendenden Broker-Werte -
// ob sie vom lokalen LoxBerry-Mosquitto (general.json) oder manuell von
// einem externen Broker stammen, entscheidet ausschliesslich die Web-UI
// (api.cgi) beim Speichern; der Daemon selbst kennt kein general.json und
// keine LoxBerry-Spezifika, das haelt ihn als reine Protokollbruecke einfach.
type MQTTConfig struct {
	UseLocalBroker bool   `json:"use_local_broker"` // reine UI-Anzeige, vom Daemon ignoriert
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	TopicPrefix    string `json:"topic_prefix"`
	ClientID       string `json:"client_id"`

	// EnabledObservations ist eine Positivliste von Easee-Observation-IDs
	// (siehe bridge.go's observationTopics fuer alle 170 bekannten IDs), die
	// tatsaechlich als MQTT-State-Topics publiziert werden sollen - auf
	// Nutzerwunsch konfigurierbar (2026-09-07), damit man in Loxone/MQTT
	// Explorer nicht durch 170 grossteils irrelevante Diagnose-/Debug-Werte
	// browsen muss. Semantik bewusst ueber die JSON/Go-Nil-Unterscheidung
	// gesteuert: `nil` (Schluessel fehlt komplett in config.json) bedeutet
	// "keine Einschraenkung, alles publizieren" (Ruecken-kompatibler Fallback
	// falls diese Datei jemals ohne dieses Feld existiert); eine explizit
	// vorhandene, ggf. auch leere Liste ist eine echte Positivliste. Die
	// Web-UI schickt beim Speichern immer die volle Auswahl mit, der Daemon
	// selbst trifft keine eigene Annahme ueber "sinnvolle" IDs.
	EnabledObservations []int `json:"enabled_observations"`
}

type Config struct {
	Easee EaseeConfig `json:"easee"`
	MQTT  MQTTConfig  `json:"mqtt"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config.json konnte nicht gelesen werden (%s): %w", path, err)
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("config.json ist kein gueltiges JSON (%s): %w", path, err)
	}

	// Passwoerter liegen in config.json verschluesselt (siehe crypto.go) -
	// hier entschluesseln. Faellt Decrypt() auf einen Wert zurueck, der kein
	// gueltiges Chiffrat ist, wird er unveraendert als Klartext behandelt -
	// deckt den Uebergang von vor Einfuehrung dieser Verschluesselung bereits
	// bestehenden config.json-Dateien ab (dort steht noch echter Klartext),
	// ohne den Dienst dadurch zu brechen. Nach dem naechsten Speichern in der
	// Web-UI liegt der Wert dann verschluesselt vor.
	if plain, ok := Decrypt(path, c.Easee.Password); ok {
		c.Easee.Password = plain
	}
	if plain, ok := Decrypt(path, c.MQTT.Password); ok {
		c.MQTT.Password = plain
	}

	if c.Easee.Username == "" || c.Easee.Password == "" {
		return nil, fmt.Errorf("config.json: easee.username/password sind leer - bitte in der Web-UI konfigurieren")
	}
	if len(c.Easee.Chargers) == 0 {
		return nil, fmt.Errorf("config.json: easee.chargers ist leer - bitte mindestens einen Charger in der Web-UI hinzufuegen")
	}
	if c.MQTT.Host == "" {
		return nil, fmt.Errorf("config.json: mqtt.host ist leer - lokalen Broker aktivieren oder externen Broker eintragen")
	}
	if c.MQTT.Port == 0 {
		c.MQTT.Port = 1883
	}
	if c.MQTT.TopicPrefix == "" {
		c.MQTT.TopicPrefix = "easee/"
	}
	if c.MQTT.ClientID == "" {
		c.MQTT.ClientID = "easeemqtt"
	}
	return &c, nil
}
