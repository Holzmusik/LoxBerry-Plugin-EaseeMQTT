package bridge

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
)

func captureLog(t *testing.T, fn func()) string {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	fn()
	return buf.String()
}

func TestObservationDebugLog(t *testing.T) {
	cfg := testConfig()
	cfg.MQTT.EnabledObservations = []int{48, 128}
	b := New(cfg, nil)

	debug := false
	b.DebugEnabled = func() bool { return debug }
	if out := captureLog(t, func() { b.HandleObservation(obs("EH1", 120, "3.6")) }); out != "" {
		t.Fatalf("ohne Debug-Level darf nichts geloggt werden: %q", out)
	}

	debug = true
	out := captureLog(t, func() {
		b.HandleObservation(obs("EH1", 120, "3.6"))  // nicht ausgewaehlt
		b.HandleObservation(obs("EH1", 48, "16"))    // ausgewaehlt, aber kein MQTT-Client
		b.HandleObservation(obs("EH1", 128, "ABCD")) // personenbezogen
	})
	for _, want := range []string{
		`obs: EH1 id=120 power_kw = "3.6" (nicht ausgewaehlt)`,
		`obs: EH1 id=48 dynamic_current_a = "16" (MQTT getrennt, verworfen)`,
		`obs: EH1 id=128 user_idtoken = "<maskiert>"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Log enthaelt nicht %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "ABCD") {
		t.Error("personenbezogener Wert im Log")
	}
}
