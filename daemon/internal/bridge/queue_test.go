package bridge

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestQueueOrderAndCoalesce(t *testing.T) {
	q := newChargerQueue()
	var got []string
	add := func(key, name string) { q.push(&job{key: key, run: func() { got = append(got, name) }}) }
	add("", "start")
	add("set/dyn", "dyn=10")
	add("", "toggle")
	add("set/dyn", "dyn=12") // ersetzt dyn=10, Position bleibt
	add("", "toggle")        // cmd/* nie zusammenfassen
	for j := q.next(); j != nil; j = q.next() {
		j.run()
	}
	want := []string{"start", "dyn=12", "toggle", "toggle"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	add("set/dyn", "dyn=14") // nach dem Abarbeiten wieder neu einreihbar
	if j := q.next(); j == nil {
		t.Fatal("Job nach Abarbeiten nicht erneut einreihbar")
	}
}

// ── Mock fuer restCommander ────────────────────────────────────────────────

type call struct {
	charger, method string
	value           any
}

type mockRest struct {
	mu    sync.Mutex
	calls []call
	block map[string]chan struct{} // charger -> blockiert bis geschlossen
	fail  bool
}

func newMockRest() *mockRest { return &mockRest{block: map[string]chan struct{}{}} }

func (m *mockRest) rec(ctx context.Context, charger, method string, v any) error {
	m.mu.Lock()
	ch := m.block[charger]
	m.mu.Unlock()
	if ch != nil {
		select {
		case <-ch:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, call{charger, method, v})
	if m.fail {
		return context.DeadlineExceeded
	}
	return nil
}

func (m *mockRest) snapshot() []call {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]call(nil), m.calls...)
}

func (m *mockRest) StartCharging(ctx context.Context, id string) error {
	return m.rec(ctx, id, "start", nil)
}
func (m *mockRest) StopCharging(ctx context.Context, id string) error {
	return m.rec(ctx, id, "stop", nil)
}
func (m *mockRest) PauseCharging(ctx context.Context, id string) error {
	return m.rec(ctx, id, "pause", nil)
}
func (m *mockRest) ResumeCharging(ctx context.Context, id string) error {
	return m.rec(ctx, id, "resume", nil)
}
func (m *mockRest) ToggleCharging(ctx context.Context, id string) error {
	return m.rec(ctx, id, "toggle", nil)
}
func (m *mockRest) Reboot(ctx context.Context, id string) error { return m.rec(ctx, id, "reboot", nil) }
func (m *mockRest) OverrideSchedule(ctx context.Context, id string) error {
	return m.rec(ctx, id, "override", nil)
}
func (m *mockRest) SetDynamicCurrent(ctx context.Context, id string, a int) error {
	return m.rec(ctx, id, "dynamic", a)
}
func (m *mockRest) SetMaxCurrent(ctx context.Context, id string, a int) error {
	return m.rec(ctx, id, "max", a)
}
func (m *mockRest) SetEnabled(ctx context.Context, id string, v bool) error {
	return m.rec(ctx, id, "enabled", v)
}
func (m *mockRest) LockCablePermanently(ctx context.Context, id string, v bool) error {
	return m.rec(ctx, id, "cable_lock", v)
}
func (m *mockRest) SetSinglePhaseLimit(ctx context.Context, id string, v bool) error {
	return m.rec(ctx, id, "single_phase", v)
}
func (m *mockRest) SetPhaseMode(ctx context.Context, id string, v int) error {
	return m.rec(ctx, id, "phase_mode", v)
}
func (m *mockRest) SetSmartCharging(ctx context.Context, id string, v bool) error {
	return m.rec(ctx, id, "smart", v)
}
func (m *mockRest) SetLedStripBrightness(ctx context.Context, id string, v int) error {
	return m.rec(ctx, id, "led", v)
}
func (m *mockRest) SetIdleCurrentEnabled(ctx context.Context, id string, v bool) error {
	return m.rec(ctx, id, "idle", v)
}

// ── Fake fuer mqtt.Message ─────────────────────────────────────────────────

type fakeMsg struct {
	topic    string
	payload  string
	retained bool
}

func (f fakeMsg) Duplicate() bool   { return false }
func (f fakeMsg) Qos() byte         { return 1 }
func (f fakeMsg) Retained() bool    { return f.retained }
func (f fakeMsg) Topic() string     { return f.topic }
func (f fakeMsg) MessageID() uint16 { return 0 }
func (f fakeMsg) Payload() []byte   { return []byte(f.payload) }
func (f fakeMsg) Ack()              {}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("Bedingung nicht rechtzeitig erfuellt")
}

// Punkt 2: ein haengender Cloud-Call fuer EH1 darf EH2 nicht blockieren, und
// der paho-Handler selbst kehrt sofort zurueck.
func TestHangingCallDoesNotBlockOtherCharger(t *testing.T) {
	m := newMockRest()
	release := make(chan struct{})
	m.block["EH1"] = release
	b := New(testConfig(), m)
	b.startWorkers()
	defer close(b.done)

	h1 := b.cmdHandler("EH1", "start", m.StartCharging)
	h2 := b.cmdHandler("EH2", "start", m.StartCharging)

	returned := make(chan struct{})
	go func() {
		h1(nil, fakeMsg{topic: "easee/EH1/cmd/start"})
		h2(nil, fakeMsg{topic: "easee/EH2/cmd/start"})
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("Handler blockiert")
	}
	waitFor(t, func() bool { return len(m.snapshot()) == 1 })
	if c := m.snapshot()[0]; c.charger != "EH2" {
		t.Fatalf("erster Call = %+v, erwartet EH2", c)
	}
	close(release)
	waitFor(t, func() bool { return len(m.snapshot()) == 2 })
}

// Waehrend ein Call haengt, eintreffende Sollwerte werden zusammengefasst:
// nur der letzte geht raus.
func TestLatestValueWinsWhileBusy(t *testing.T) {
	m := newMockRest()
	release := make(chan struct{})
	m.block["EH1"] = release
	b := New(testConfig(), m)
	b.startWorkers()
	defer close(b.done)

	start := b.cmdHandler("EH1", "start", m.StartCharging)
	led := b.intHandler("EH1", "led", m.SetLedStripBrightness)
	start(nil, fakeMsg{topic: "easee/EH1/cmd/start"}) // haengt
	for _, v := range []string{"10", "20", "30"} {
		led(nil, fakeMsg{topic: "easee/EH1/set/led_brightness", payload: v})
	}
	m.mu.Lock()
	delete(m.block, "EH1")
	m.mu.Unlock()
	close(release)
	waitFor(t, func() bool { return len(m.snapshot()) == 2 })
	time.Sleep(50 * time.Millisecond)
	calls := m.snapshot()
	if len(calls) != 2 || calls[1].method != "led" || calls[1].value != 30 {
		t.Fatalf("calls = %+v, erwartet start + led=30", calls)
	}
}

func TestRetainedCmdIgnored(t *testing.T) {
	m := newMockRest()
	b := New(testConfig(), m)
	b.startWorkers()
	defer close(b.done)
	b.cmdHandler("EH1", "reboot", m.Reboot)(nil, fakeMsg{topic: "easee/EH1/cmd/reboot", retained: true})
	b.cmdHandler("EH1", "start", m.StartCharging)(nil, fakeMsg{topic: "easee/EH1/cmd/start"})
	waitFor(t, func() bool { return len(m.snapshot()) == 1 })
	time.Sleep(50 * time.Millisecond)
	if calls := m.snapshot(); len(calls) != 1 || calls[0].method != "start" {
		t.Fatalf("calls = %+v, retained reboot haette ignoriert werden muessen", calls)
	}
}

func TestSignalRStateWithoutMQTT(t *testing.T) {
	b := New(testConfig(), newMockRest())
	b.HandleSignalRState(true) // ohne MQTT-Client: nur merken, kein Panic
	if b.signalrState != "connected" {
		t.Fatalf("state = %q", b.signalrState)
	}
	b.HandleSignalRState(false)
	if b.signalrState != "disconnected" {
		t.Fatalf("state = %q", b.signalrState)
	}
}
