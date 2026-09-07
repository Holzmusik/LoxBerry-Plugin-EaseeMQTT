// Package bridge ist die reine Protokolluebersetzung Easee <-> MQTT - KEINE
// eigene Entscheidungslogik (Lastmanagement bleibt vollstaendig extern/in
// Loxone, siehe Plan-Datei). Ziel (Nutzer-Vorgabe 2026-09-07): ALLE
// Observation-Daten landen in MQTT, und die Wallboxen sind komplett
// steuerbar - siehe README fuer das vollstaendige Topic-Schema.
package bridge

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"

	"easeemqtt/internal/config"
	"easeemqtt/internal/easee"
)

// observationTopics ordnet JEDE bekannte Easee-SignalR-Observation-ID (siehe
// Plan-Datei/README) einem MQTT-State-Topic-Suffix zu - generiert aus der
// echten `ObservationID`-Enum in `evcc-io/evcc`s Produktionscode
// (charger/easee/signalr.go, verifiziert 2026-09-07, 170 IDs), NICHT von
// Hand abgetippt (Fehlerrisiko bei so vielen Eintraegen zu hoch - siehe
// scratchpad/gen_observation_map.py fuer den Generator). Ein paar bereits
// vor dieser Erweiterung genutzte, "sprechende" Namen wurden bewusst
// beibehalten (opmode/power_kw/session_energy_kwh/lifetime_energy_kwh/
// cable_locked/max_current_a/dynamic_current_a/error_code/current_t2..t5),
// weil Loxone-seitig schon gegen genau diese Topic-Namen getestet wurde -
// alle anderen IDs verwenden die kleingeschriebene Konstante aus der Easee-
// Enum direkt als Topic-Suffix (z.B. `derating_active`, `pilot_mode`,
// `in_volt_t1_t2`).
//
// current_t2..t5 (Rohstrom) und in_volt_t1_t2..t4_t5 (Rohspannung) werden
// bewusst NICHT auf L1/L2/L3 umbenannt: die exakte Zuordnung
// Terminal->Netzphase ist in dieser Session nicht verifiziert (siehe
// Plan-Datei/README) - Easees eigene T-Nummern-Benennung wird 1:1
// uebernommen, um keine falsche Phasenzuordnung vorzutaeuschen.
//
// 219 (fatal_error_code) ist NICHT Teil der oben genannten evcc-Enum-Liste,
// aber real bestaetigt (Nutzer-Mitschnitt 2026-09-07 lieferte dafuer einen
// Wert; Name aus pyeasee/const.py uebernommen).
var observationTopics = map[int]string{
	1:   "self_test_result",
	2:   "self_test_details",
	10:  "wifi_event",
	11:  "charger_offline_reason",
	13:  "easee_link_command_response",
	14:  "easee_link_data_received",
	15:  "local_pre_authorize_enabled",
	16:  "local_authorize_offline_enabled",
	17:  "allow_offline_tx_for_unknown_id",
	18:  "erratic_evmax_toggles",
	19:  "backplate_type",
	20:  "site_structure",
	21:  "detected_power_grid_type",
	22:  "circuit_max_current_p1",
	23:  "circuit_max_current_p2",
	24:  "circuit_max_current_p3",
	25:  "location",
	26:  "site_idstring",
	27:  "site_idnumeric",
	28:  "rfid_timeout_auth",
	30:  "lock_cable_permanently",
	31:  "is_enabled",
	32:  "temperature_monitor_state",
	33:  "circuit_sequence_number",
	34:  "single_phase_number",
	35:  "enable3_phases_deprecated",
	36:  "wi_fi_ssid",
	37:  "enable_idle_current",
	38:  "phase_mode",
	39:  "forced_three_phase_on_itwith_gnd_fault",
	40:  "led_strip_brightness",
	41:  "local_authorization_required",
	42:  "authorization_required",
	43:  "remote_start_required",
	44:  "smart_button_enabled",
	45:  "offline_charging_mode",
	46:  "ledmode",
	47:  "max_current_a",
	48:  "dynamic_current_a",
	50:  "max_current_offline_fallback_p1",
	51:  "max_current_offline_fallback_p2",
	52:  "max_current_offline_fallback_p3",
	54:  "release_cable_at_power_off",
	56:  "listen_to_control_pulse",
	57:  "control_pulse_rtt",
	60:  "charging_session_signed",
	62:  "charging_schedule",
	65:  "paired_equalizer",
	68:  "wi_fi_apenabled",
	69:  "paired_user_idtoken",
	70:  "circuit_total_allocated_phase_conductor_current_l1",
	71:  "circuit_total_allocated_phase_conductor_current_l2",
	72:  "circuit_total_allocated_phase_conductor_current_l3",
	73:  "circuit_total_phase_conductor_current_l1",
	74:  "circuit_total_phase_conductor_current_l2",
	75:  "circuit_total_phase_conductor_current_l3",
	76:  "number_of_cars_connected",
	77:  "number_of_cars_charging",
	78:  "number_of_cars_in_queue",
	79:  "number_of_cars_fully_charged",
	80:  "software_release",
	81:  "iccid",
	82:  "modem_fw_id",
	83:  "otaerror_code",
	84:  "mobile_network_operator",
	89:  "reboot_reason",
	90:  "power_pcbversion",
	91:  "com_pcbversion",
	96:  "reason_for_no_current",
	97:  "load_balancing_number_of_connected_chargers",
	98:  "udpnum_of_connected_nodes",
	99:  "local_connection",
	100: "pilot_mode",
	101: "car_connected_deprecated",
	102: "smart_charging",
	103: "cable_locked",
	104: "cable_rating",
	105: "pilot_high",
	106: "pilot_low",
	107: "back_plate_id",
	108: "user_idtoken_reversed",
	109: "opmode",
	110: "output_phase",
	111: "dynamic_circuit_current_p1",
	112: "dynamic_circuit_current_p2",
	113: "dynamic_circuit_current_p3",
	114: "output_current",
	115: "derated_current",
	116: "derating_active",
	117: "debug_string",
	118: "error_string",
	119: "error_code",
	120: "power_kw",
	121: "session_energy_kwh",
	122: "energy_per_hour",
	123: "legacy_ev_status",
	124: "lifetime_energy_kwh",
	125: "lifetime_relay_switches",
	126: "lifetime_hours",
	127: "dynamic_current_offline_fallback_depricated",
	128: "user_idtoken",
	129: "charging_session",
	130: "cell_rssi",
	131: "cell_rat",
	132: "wi_fi_rssi",
	133: "cell_address",
	134: "wi_fi_address",
	135: "wi_fi_type",
	136: "local_rssi",
	137: "master_back_plate_id",
	138: "local_tx_power",
	139: "local_state",
	140: "found_wi_fi",
	141: "current_connection",
	142: "cellular_interface_error_count",
	143: "cellular_interface_reset_count",
	144: "wifi_interface_error_count",
	145: "wifi_interface_reset_count",
	146: "local_node_type",
	147: "local_radio_channel",
	148: "local_short_address",
	149: "local_parent_addr_or_num_of_nodes",
	150: "temp_max",
	151: "temp_ambient_power_board",
	152: "temp_input_t2",
	153: "temp_input_t3",
	154: "temp_input_t4",
	155: "temp_input_t5",
	160: "temp_output_n",
	161: "temp_output_l1",
	162: "temp_output_l2",
	163: "temp_output_l3",
	170: "temp_ambient",
	171: "light_ambient",
	172: "int_rel_humidity",
	173: "back_plate_locked",
	174: "current_motor",
	175: "back_plate_hall_sensor",
	182: "current_t2",
	183: "current_t3",
	184: "current_t4",
	185: "current_t5",
	190: "in_volt_t1_t2",
	191: "in_volt_t1_t3",
	192: "in_volt_t1_t4",
	193: "in_volt_t1_t5",
	194: "in_volt_t2_t3",
	195: "in_volt_t2_t4",
	196: "in_volt_t2_t5",
	197: "in_volt_t3_t4",
	198: "in_volt_t3_t5",
	199: "in_volt_t4_t5",
	202: "out_volt_pin1_2",
	203: "out_volt_pin1_3",
	204: "out_volt_pin1_4",
	205: "out_volt_pin1_5",
	206: "out_volt_pin2_3",
	210: "volt_level33",
	211: "volt_level5",
	212: "volt_level12",
	219: "fatal_error_code",
	220: "lte_rsrp",
	221: "lte_sinr",
	222: "lte_rsrq",
	223: "charge_session_start",
	230: "eq_available_current_p1",
	231: "eq_available_current_p2",
	232: "eq_available_current_p3",
	250: "connected_to_cloud",
	251: "cloud_disconnect_reason",
}

// restCommander ist die Teilmenge von *easee.Client, die die Bridge fuer
// Kommandos braucht - als Interface, damit bridge_test.go (falls spaeter
// ergaenzt) ohne echten HTTP-Client testen kann. Alle Pfade/Feldnamen der
// dahinterliegenden Implementierung (easee/rest.go) sind gegen den echten
// pyeasee-Sourcecode verifiziert, siehe dort.
type restCommander interface {
	StartCharging(ctx context.Context, chargerID string) error
	StopCharging(ctx context.Context, chargerID string) error
	PauseCharging(ctx context.Context, chargerID string) error
	ResumeCharging(ctx context.Context, chargerID string) error
	ToggleCharging(ctx context.Context, chargerID string) error
	Reboot(ctx context.Context, chargerID string) error
	OverrideSchedule(ctx context.Context, chargerID string) error
	SetDynamicCurrent(ctx context.Context, chargerID string, amps float64) error
	SetMaxCurrent(ctx context.Context, chargerID string, amps int) error
	SetEnabled(ctx context.Context, chargerID string, enabled bool) error
	LockCablePermanently(ctx context.Context, chargerID string, lock bool) error
	SetSinglePhaseLimit(ctx context.Context, chargerID string, enabled bool) error
	SetPhaseMode(ctx context.Context, chargerID string, mode int) error
	SetSmartCharging(ctx context.Context, chargerID string, enabled bool) error
	SetLedStripBrightness(ctx context.Context, chargerID string, brightness int) error
	SetIdleCurrentEnabled(ctx context.Context, chargerID string, enabled bool) error
}

type Bridge struct {
	cfg    *config.Config
	rest   restCommander
	client mqtt.Client
	prefix string

	// filter == nil bedeutet "keine Einschraenkung, alles publizieren" -
	// siehe config.MQTTConfig.EnabledObservations fuer die Nil/leer-
	// Unterscheidung. Ein leeres, aber nicht-nil Filter bedeutet bewusst
	// "nichts publizieren" (Nutzer hat in der UI alles abgewaehlt).
	filter        map[int]bool
	warnedUnknown map[int]bool
}

func New(cfg *config.Config, rest restCommander) *Bridge {
	var filter map[int]bool
	if cfg.MQTT.EnabledObservations != nil {
		filter = make(map[int]bool, len(cfg.MQTT.EnabledObservations))
		for _, id := range cfg.MQTT.EnabledObservations {
			filter[id] = true
		}
	}
	return &Bridge{
		cfg:           cfg,
		rest:          rest,
		prefix:        cfg.MQTT.TopicPrefix,
		filter:        filter,
		warnedUnknown: map[int]bool{},
	}
}

// Connect baut die MQTT-Verbindung auf (mit LWT auf <prefix>bridge/status)
// und abonniert die Kommando-Topics aller konfigurierten Charger.
func (b *Bridge) Connect() error {
	statusTopic := b.prefix + "bridge/status"

	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", b.cfg.MQTT.Host, b.cfg.MQTT.Port))
	opts.SetClientID(b.cfg.MQTT.ClientID)
	if b.cfg.MQTT.Username != "" {
		opts.SetUsername(b.cfg.MQTT.Username)
		opts.SetPassword(b.cfg.MQTT.Password)
	}
	opts.SetWill(statusTopic, "offline", 1, true)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(5 * time.Second)
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		log.Printf("mqtt: verbunden (%s:%d)", b.cfg.MQTT.Host, b.cfg.MQTT.Port)
		c.Publish(statusTopic, 1, true, "online")
		b.subscribeCommands(c)
	})
	opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		log.Printf("mqtt: Verbindung verloren: %v", err)
	})

	b.client = mqtt.NewClient(opts)
	token := b.client.Connect()
	token.Wait()
	return token.Error()
}

// subscribeCommands haengt fuer jeden konfigurierten Charger das komplette
// Kommando-Set ein - Ziel "komplett steuerbar" (Nutzer-Vorgabe 2026-09-07),
// nicht nur start/stop/pause/resume+dynamic_current wie in der ersten
// Version dieses Plugins.
func (b *Bridge) subscribeCommands(c mqtt.Client) {
	for _, ch := range b.cfg.Easee.Chargers {
		chargerID := ch.ID
		base := b.prefix + chargerID + "/"

		// Fixe, argumentlose Kommandos.
		c.Subscribe(base+"cmd/start", 1, b.cmdHandler(chargerID, "start", b.rest.StartCharging))
		c.Subscribe(base+"cmd/stop", 1, b.cmdHandler(chargerID, "stop", b.rest.StopCharging))
		c.Subscribe(base+"cmd/pause", 1, b.cmdHandler(chargerID, "pause", b.rest.PauseCharging))
		c.Subscribe(base+"cmd/resume", 1, b.cmdHandler(chargerID, "resume", b.rest.ResumeCharging))
		c.Subscribe(base+"cmd/toggle", 1, b.cmdHandler(chargerID, "toggle", b.rest.ToggleCharging))
		c.Subscribe(base+"cmd/reboot", 1, b.cmdHandler(chargerID, "reboot", b.rest.Reboot))
		c.Subscribe(base+"cmd/override_schedule", 1, b.cmdHandler(chargerID, "override_schedule", b.rest.OverrideSchedule))

		// Werte-Kommandos (Payload traegt den zu setzenden Wert).
		c.Subscribe(base+"set/dynamic_current", 1, b.floatHandler(chargerID, "set_dynamic_charger_current", b.rest.SetDynamicCurrent))
		c.Subscribe(base+"set/max_current", 1, b.intHandler(chargerID, "set_max_charger_current", b.rest.SetMaxCurrent))
		c.Subscribe(base+"set/enabled", 1, b.boolHandler(chargerID, "enable_charger", b.rest.SetEnabled))
		c.Subscribe(base+"set/cable_lock", 1, b.boolHandler(chargerID, "lock_cable_permanently", b.rest.LockCablePermanently))
		c.Subscribe(base+"set/single_phase_limit", 1, b.boolHandler(chargerID, "limit_to_single_phase", b.rest.SetSinglePhaseLimit))
		c.Subscribe(base+"set/phase_mode", 1, b.intHandler(chargerID, "phase_mode", b.rest.SetPhaseMode))
		c.Subscribe(base+"set/smart_charging", 1, b.boolHandler(chargerID, "smart_charging", b.rest.SetSmartCharging))
		c.Subscribe(base+"set/led_brightness", 1, b.intHandler(chargerID, "led_strip_brightness", b.rest.SetLedStripBrightness))
		c.Subscribe(base+"set/idle_current", 1, b.boolHandler(chargerID, "enable_idle_current", b.rest.SetIdleCurrentEnabled))
	}
}

func (b *Bridge) cmdHandler(chargerID, label string, fn func(ctx context.Context, chargerID string) error) mqtt.MessageHandler {
	return func(c mqtt.Client, msg mqtt.Message) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := fn(ctx, chargerID); err != nil {
			log.Printf("bridge: %s fuer %s fehlgeschlagen: %v", label, chargerID, err)
		} else {
			log.Printf("bridge: %s -> %s ausgefuehrt", label, chargerID)
		}
	}
}

// parseBool akzeptiert die gaengigen Loxone-MQTT-Output-Payload-Formen
// grosszuegig (numerisch "1"/"0" UND "true"/"false"/"on"/"off"), damit die
// Wahl des Loxone-seitigen MQTT-Bausteins nicht auf ein bestimmtes
// Payload-Format festgelegt ist.
func parseBool(payload string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(payload)) {
	case "1", "true", "on":
		return true, nil
	case "0", "false", "off":
		return false, nil
	default:
		return false, fmt.Errorf("unbekannter Bool-Payload %q", payload)
	}
}

func (b *Bridge) boolHandler(chargerID, label string, fn func(ctx context.Context, chargerID string, v bool) error) mqtt.MessageHandler {
	return func(c mqtt.Client, msg mqtt.Message) {
		v, err := parseBool(string(msg.Payload()))
		if err != nil {
			log.Printf("bridge: %s: %v", msg.Topic(), err)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := fn(ctx, chargerID, v); err != nil {
			log.Printf("bridge: %s fuer %s fehlgeschlagen: %v", label, chargerID, err)
		} else {
			log.Printf("bridge: %s fuer %s auf %v gesetzt", label, chargerID, v)
		}
	}
}

func (b *Bridge) intHandler(chargerID, label string, fn func(ctx context.Context, chargerID string, v int) error) mqtt.MessageHandler {
	return func(c mqtt.Client, msg mqtt.Message) {
		v, err := strconv.Atoi(strings.TrimSpace(string(msg.Payload())))
		if err != nil {
			log.Printf("bridge: %s: Payload %q ist keine Ganzzahl: %v", msg.Topic(), string(msg.Payload()), err)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := fn(ctx, chargerID, v); err != nil {
			log.Printf("bridge: %s fuer %s fehlgeschlagen: %v", label, chargerID, err)
		} else {
			log.Printf("bridge: %s fuer %s auf %d gesetzt", label, chargerID, v)
		}
	}
}

func (b *Bridge) floatHandler(chargerID, label string, fn func(ctx context.Context, chargerID string, v float64) error) mqtt.MessageHandler {
	return func(c mqtt.Client, msg mqtt.Message) {
		v, err := strconv.ParseFloat(strings.TrimSpace(string(msg.Payload())), 64)
		if err != nil {
			log.Printf("bridge: %s: Payload %q ist keine Zahl: %v", msg.Topic(), string(msg.Payload()), err)
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := fn(ctx, chargerID, v); err != nil {
			log.Printf("bridge: %s fuer %s fehlgeschlagen: %v", label, chargerID, err)
		} else {
			log.Printf("bridge: %s fuer %s auf %.1f gesetzt", label, chargerID, v)
		}
	}
}

// HandleObservation wird von main.go als onObservation-Callback an
// easee.NewHub() uebergeben - reine 1:1-Uebersetzung Observation -> retained
// MQTT-State-Topic, keine eigene Logik/Filterung ausser der ID->Topic-Tabelle.
// Dank der vollstaendigen observationTopics-Tabelle sollte hier im
// Normalfall keine ID mehr als "unbekannt" auftauchen.
func (b *Bridge) HandleObservation(obs easee.Observation) {
	suffix, known := observationTopics[obs.ID]
	if !known {
		if !b.warnedUnknown[obs.ID] {
			b.warnedUnknown[obs.ID] = true
			log.Printf("bridge: unbekannte Observation-ID %d (Charger %s, Wert %q) - wird ignoriert", obs.ID, obs.ChargerID, obs.Value)
		}
		return
	}
	// Nutzer-konfigurierte Positivliste (siehe config.MQTTConfig.
	// EnabledObservations) - bewusst OHNE Log-Meldung, das ist beabsichtigte
	// Filterung, kein Fehlerfall wie bei "unbekannter" ID oben.
	if b.filter != nil && !b.filter[obs.ID] {
		return
	}
	if b.client == nil || !b.client.IsConnected() {
		return
	}
	topic := b.prefix + obs.ChargerID + "/state/" + suffix
	b.client.Publish(topic, 0, true, obs.Value)
}

func (b *Bridge) Disconnect() {
	if b.client != nil && b.client.IsConnected() {
		b.client.Disconnect(250)
	}
}
