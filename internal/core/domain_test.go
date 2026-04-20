package core

import "testing"

func TestValidTransport(t *testing.T) {
	cases := map[TransportMode]bool{
		"":             true,
		TransportAsync: true,
		TransportLive:  true,
		TransportBoth:  true,
		"weird":        false,
	}

	for in, want := range cases {
		if got := ValidTransport(in); got != want {
			t.Errorf("ValidTransport(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestTransportOrDefault(t *testing.T) {
	if got := TransportOrDefault(""); got != TransportAsync {
		t.Errorf("TransportOrDefault(\"\") = %q, want %q", got, TransportAsync)
	}

	if got := TransportOrDefault(TransportLive); got != TransportLive {
		t.Errorf("TransportOrDefault(%q) = %q, want %q", TransportLive, got, TransportLive)
	}
}
