package bridge

import (
	"sync"
	"testing"
	"time"
)

func TestParseAmps(t *testing.T) {
	cases := map[string]int{"16": 16, "16.0": 16, "13.7": 13, "5.9999999": 6, " 8 ": 8, "-3": 0, "0": 0, "0.4": 0}
	for in, want := range cases {
		got, err := parseAmps(in)
		if err != nil || got != want {
			t.Errorf("parseAmps(%q) = %d,%v; want %d", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "abc", "NaN", "Inf"} {
		if _, err := parseAmps(bad); err == nil {
			t.Errorf("parseAmps(%q) sollte Fehler liefern", bad)
		}
	}
}

func TestThrottleDelayRules(t *testing.T) {
	th := newAmpThrottle(30 * time.Second)
	s := &throttleState{}
	t0 := time.Unix(1000, 0)
	if d := th.delay(s, 16, t0); d != 0 {
		t.Fatal("erster Wert: sofort")
	}
	s.has, s.lastAt, s.lastVal = true, t0, 10
	if d := th.delay(s, 16, t0.Add(5*time.Second)); d != 25*time.Second {
		t.Fatalf("Erhoehung nach 5 s: 25 s warten, got %v", d)
	}
	if d := th.delay(s, 16, t0.Add(31*time.Second)); d != 0 {
		t.Fatal("Erhoehung nach Ablauf: sofort")
	}
	if d := th.delay(s, 8, t0.Add(time.Second)); d != 0 {
		t.Fatal("Absenkung: sofort")
	}
	if d := th.delay(s, 0, t0.Add(time.Second)); d != 0 {
		t.Fatal("0: sofort")
	}
	if d := th.delay(s, 10, t0.Add(time.Second)); d != 0 {
		t.Fatal("unveraendert: sofort (sendGate entscheidet)")
	}
	if d := newAmpThrottle(0).delay(s, 16, t0.Add(time.Second)); d != 0 {
		t.Fatal("Intervall 0 = Drosselung aus")
	}
}

type sendLog struct {
	mu   sync.Mutex
	vals []int
}

func (l *sendLog) add(v int) { l.mu.Lock(); l.vals = append(l.vals, v); l.mu.Unlock() }
func (l *sendLog) get() []int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]int(nil), l.vals...)
}

func TestThrottleTrailingLatestValue(t *testing.T) {
	th := newAmpThrottle(100 * time.Millisecond)
	var log sendLog
	send := func(v int) { log.add(v); th.sent("k", v, time.Now()) }
	th.submit("k", 10, time.Now(), send) // sofort
	th.submit("k", 12, time.Now(), send) // gedrosselt
	th.submit("k", 14, time.Now(), send) // ersetzt 12
	if got := log.get(); len(got) != 1 || got[0] != 10 {
		t.Fatalf("sofort gesendet: %v", got)
	}
	time.Sleep(200 * time.Millisecond)
	if got := log.get(); len(got) != 2 || got[1] != 14 {
		t.Fatalf("nachgesendet: %v, erwartet [10 14]", got)
	}
}

func TestThrottleDecreaseCancelsPendingIncrease(t *testing.T) {
	th := newAmpThrottle(100 * time.Millisecond)
	var log sendLog
	send := func(v int) { log.add(v); th.sent("k", v, time.Now()) }
	th.submit("k", 10, time.Now(), send)
	th.submit("k", 16, time.Now(), send) // Erhoehung wartet
	th.submit("k", 6, time.Now(), send)  // Absenkung sofort, wartende Erhoehung verfaellt
	time.Sleep(200 * time.Millisecond)
	got := log.get()
	if len(got) != 2 || got[0] != 10 || got[1] != 6 {
		t.Fatalf("got %v, erwartet [10 6]", got)
	}
}

// Ende-zu-Ende ueber den Handler: Bruchteile werden abgerundet und als int
// gesendet, Erhoehungen gedrosselt, Absenkungen sofort.
func TestAmpsHandlerEndToEnd(t *testing.T) {
	m := newMockRest()
	cfg := testConfig()
	iv := 0 // Sekunden; unten direkt auf 100 ms gesetzt
	cfg.Easee.DynamicCurrentIntervalS = &iv
	b := New(cfg, m)
	b.amps = newAmpThrottle(100 * time.Millisecond)
	b.startWorkers()
	defer close(b.done)

	h := b.ampsHandler("EH1", "dyn", m.SetDynamicCurrent)
	topic := "easee/EH1/set/dynamic_current"
	h(nil, fakeMsg{topic: topic, payload: "10.8"}) // -> 10 sofort
	waitFor(t, func() bool { return len(m.snapshot()) == 1 })
	h(nil, fakeMsg{topic: topic, payload: "12.2"}) // Erhoehung, wartet
	h(nil, fakeMsg{topic: topic, payload: "13.9"}) // ersetzt -> 13
	time.Sleep(20 * time.Millisecond)
	if n := len(m.snapshot()); n != 1 {
		t.Fatalf("Erhoehung zu frueh gesendet (%d Calls)", n)
	}
	waitFor(t, func() bool { return len(m.snapshot()) == 2 })
	h(nil, fakeMsg{topic: topic, payload: "4"}) // Absenkung sofort
	waitFor(t, func() bool { return len(m.snapshot()) == 3 })
	calls := m.snapshot()
	want := []int{10, 13, 4}
	for i, w := range want {
		if calls[i].value != w {
			t.Fatalf("calls = %+v, erwartet amps %v", calls, want)
		}
	}
}
