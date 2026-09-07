package config

import "testing"

func TestDockerLocalhostRewritesLoopbackInDocker(t *testing.T) {
	restore := SetInDockerForTest(true)
	defer restore()

	cases := []struct {
		name string
		in   string
		want string
	}{
		{"localhost", "http://localhost:11434/v1", "http://host.docker.internal:11434/v1"},
		{"loopback ip", "http://127.0.0.1:11434/v1", "http://host.docker.internal:11434/v1"},
		{"non-loopback unchanged", "https://api.openai.com/v1", "https://api.openai.com/v1"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := DockerLocalhost(tc.in); got != tc.want {
				t.Fatalf("DockerLocalhost(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestDockerLocalhostPassthroughOutsideDocker(t *testing.T) {
	restore := SetInDockerForTest(false)
	defer restore()

	in := "http://localhost:11434/v1"
	if got := DockerLocalhost(in); got != in {
		t.Fatalf("DockerLocalhost(%q) = %q, want unchanged outside Docker", in, got)
	}
}
