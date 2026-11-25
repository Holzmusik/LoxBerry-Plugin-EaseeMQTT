package bridge

import (
	"testing"
	"time"
)

func TestNormalize(t *testing.T) {
	cases := []struct {
		kind valueKind
		in   string
		want string
		ok   bool
	}{
		{kindBool, "1", "true", true},
		{kindBool, "True", "true", true},
		{kindBool, "false", "false", true},
		{kindBool, "0", "false", true},
		{kindBool, "x", "", false},
		{kindInt, "16", "16", true},
		{kindInt, "16.0", "16", true},
		{kindInt, " 32 ", "32", true},
		{kindInt, "abc", "", false},
		{kindFloat, "16", "16.00", true},
		{kindFloat, "16.0", "16.00", true},
		{kindFloat, "6.5", "6.50", true},
		{kindFloat, "NaN", "", false},
	}
	for _, c := range cases {
		got, ok := normalize(c.kind, c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("normalize(%v, %q) = %q,%v; want %q,%v", c.kind, c.in, got, ok, c.want, c.ok)
		}
	}
}

const k = "easee/EH1/set/dynamic_current"

func TestGateFirstAndChange(t *testing.T) {
	g := newSendGate()
	t0 := time.Unix(1000, 0)
	if ok, _ := g.shouldSend(k, "16", t0); !ok {
		t.Fatal("erster Wert muss gesendet werden")
	}
	g.markSent(k, "16", t0)
	if ok, _ := g.shouldSend(k, "16", t0.Add(time.Second)); ok {
		t.Fatal("unveraenderter Wert darf nicht erneut gesendet werden")
	}
	if ok, _ := g.shouldSend(k, "10", t0.Add(time.Second)); !ok {
		t.Fatal("geaenderter Wert muss gesendet werden")
	}
}

func TestGateFailedCallIsNotMarked(t *testing.T) {
	g := newSendGate()
	t0 := time.Unix(1000, 0)
	g.shouldSend(k, "16", t0) // Call schlaegt fehl -> kein markSent
	if ok, _ := g.shouldSend(k, "16", t0.Add(time.Second)); !ok {
		t.Fatal("nach fehlgeschlagenem Call muss derselbe Wert erneut gesendet werden")
	}
}

// Der eigentliche Bug: Wallbox-Neustart setzt Dynamic Current zurueck,
// Loxone schickt denselben Sollwert -> muss wieder durchgehen.
func TestGateObservedDiffers(t *testing.T) {
	g := newSendGate()
	t0 := time.Unix(1000, 0)
	g.markSent(k, "16", t0)
	g.observe(k, "16")
	if ok, _ := g.shouldSend(k, "16", t0.Add(2*time.Minute)); ok {
		t.Fatal("Ist = Soll: nicht senden")
	}
	g.observe(k, "32") // Wallbox nach Neustart
	if ok, _ := g.shouldSend(k, "16", t0.Add(30*time.Second)); ok {
		t.Fatal("innerhalb der Karenzzeit nach dem Senden nicht erneut senden")
	}
	ok, reason := g.shouldSend(k, "16", t0.Add(2*time.Minute))
	if !ok {
		t.Fatal("Ist != Soll nach Karenzzeit: senden")
	}
	if reason != "Wallbox meldet 32" {
		t.Errorf("reason = %q", reason)
	}
}

func TestGateTTL(t *testing.T) {
	g := newSendGate()
	t0 := time.Unix(1000, 0)
	g.markSent(k, "16", t0)
	if ok, _ := g.shouldSend(k, "16", t0.Add(9*time.Minute)); ok {
		t.Fatal("vor Ablauf der TTL nicht senden")
	}
	if ok, _ := g.shouldSend(k, "16", t0.Add(10*time.Minute)); !ok {
		t.Fatal("nach Ablauf der TTL erneut senden")
	}
}

func TestObservationFeedsGate(t *testing.T) {
	b := New(testConfig(), nil)
	b.HandleObservation(obs("EH1", 48, "32.0"))
	b.HandleObservation(obs("EH1", 31, "False"))
	b.gate.mu.Lock()
	defer b.gate.mu.Unlock()
	if got := b.gate.observed["easee/EH1/set/dynamic_current"]; got != "32" { // "32.0" -> ganzzahlig
		t.Errorf("dynamic_current beobachtet = %q", got)
	}
	if got := b.gate.observed["easee/EH1/set/enabled"]; got != "false" {
		t.Errorf("enabled beobachtet = %q", got)
	}
}
