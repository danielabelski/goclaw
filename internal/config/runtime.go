package config

import (
	"os"
	"strings"
	"sync"
	"sync/atomic"
)

var (
	dockerOnce   sync.Once
	dockerCached bool

	// inDockerForTest overrides Docker detection. Production code MUST never
	// set this; use SetInDockerForTest exclusively from *_test.go.
	inDockerForTest atomic.Pointer[bool]
)

// SetInDockerForTest forces InDocker to return v, bypassing the /.dockerenv
// probe. Returns a restore func that reinstates the previous override (or real
// detection when none was active). Test-only.
func SetInDockerForTest(v bool) func() {
	prev := inDockerForTest.Load()
	inDockerForTest.Store(&v)
	return func() { inDockerForTest.Store(prev) }
}

// InDocker returns true when running inside a Docker container.
// Result is cached after the first call.
func InDocker() bool {
	if override := inDockerForTest.Load(); override != nil {
		return *override
	}
	dockerOnce.Do(func() {
		_, err := os.Stat("/.dockerenv")
		dockerCached = err == nil
	})
	return dockerCached
}

// DockerLocalhost rewrites localhost or 127.0.0.1 in url to host.docker.internal
// when running inside Docker, so the container can reach host services.
// Returns the url unchanged when not in Docker or when it doesn't reference loopback.
func DockerLocalhost(url string) string {
	if !InDocker() {
		return url
	}
	if strings.Contains(url, "localhost") {
		return strings.Replace(url, "localhost", "host.docker.internal", 1)
	}
	if strings.Contains(url, "127.0.0.1") {
		return strings.Replace(url, "127.0.0.1", "host.docker.internal", 1)
	}
	return url
}
