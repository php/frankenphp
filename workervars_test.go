package frankenphp_test

import (
	"path/filepath"
	"testing"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bgWorker declares a background worker from testdata/bgworker, scoped to
// server when one is given
func bgWorker(name, file string, env map[string]string, server *frankenphp.Server) frankenphp.Option {
	opts := []frankenphp.WorkerOption{frankenphp.WithWorkerBackground(), frankenphp.WithWorkerEnv(env)}
	if server != nil {
		opts = append(opts, frankenphp.WithWorkerServerScope(server))
	}

	return frankenphp.WithWorkers(name, "testdata/bgworker/"+file, 1, opts...)
}

func TestVarsRoundTrip(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), bgWorker("publisher", "publisher.php", nil, server), frankenphp.WithNumThreads(2))

	body := serverGet(t, server, "http://example.com/vars.php?name=publisher")
	assert.JSONEq(t, `{"answer":42,"value":"default","nested":{"a":1,"list":[true,null,1.5,"x"]},"worker":"publisher"}`, body)

	// a second read is a fresh copy of the same snapshot
	assert.JSONEq(t, body, serverGet(t, server, "http://example.com/vars.php?name=publisher"))
	assert.Contains(t, serverGet(t, server, "http://example.com/vars.php?name=nope"), "unknown background worker")
}

// a worker booting before its dependency has published blocks in
// frankenphp_get_vars() until the dependency is ready
func TestVarsBlockUntilReady(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "consumer.json")
	initServers(t,
		bgWorker("consumer", "consumer.php", map[string]string{"BG_CONSUME": "publisher", "BG_SENTINEL": sentinel}, nil),
		bgWorker("publisher", "publisher.php", map[string]string{"BG_PUBLISH_DELAY_MS": "500"}, nil),
		frankenphp.WithNumThreads(3),
	)

	assert.Contains(t, requireFileContentEventually(t, sentinel), `"answer":42`)
}

func TestVarsCycleIsRefused(t *testing.T) {
	tmp := t.TempDir()
	s1, s2 := filepath.Join(tmp, "c1.txt"), filepath.Join(tmp, "c2.txt")
	initServers(t,
		bgWorker("c1", "consumer.php", map[string]string{"BG_CONSUME": "c2", "BG_SENTINEL": s1}, nil),
		bgWorker("c2", "consumer.php", map[string]string{"BG_CONSUME": "c1", "BG_SENTINEL": s2}, nil),
		frankenphp.WithNumThreads(3),
	)

	// one side sees the cycle, the other then reads a ready worker that never published
	results := requireFileContentEventually(t, s1) + "\n" + requireFileContentEventually(t, s2)
	assert.Contains(t, results, "circular dependency")
	assert.Contains(t, results, "has not published any vars yet")
}

func TestVarsNotPublished(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), bgWorker("silent", "basic.php", nil, server), frankenphp.WithNumThreads(2))

	assert.Contains(t, serverGet(t, server, "http://example.com/vars.php?name=silent"), "has not published any vars yet")
}

func TestVarsRejectInvalidValues(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "bad.txt")
	initServers(t, bgWorker("bad", "bad-vars.php", map[string]string{"BG_SENTINEL": sentinel}, nil), frankenphp.WithNumThreads(2))

	result := requireFileContentEventually(t, sentinel)
	assert.Contains(t, result, "ValueError")
	assert.Contains(t, result, "must be null, scalars, arrays or enums")
}

// a reboot restarts the PHP engine, and a snapshot published before it is
// still served: it must not point into the memory of the engine that is gone
func TestVarsSurviveAReboot(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	sentinel := filepath.Join(t.TempDir(), "published")
	initServers(t, frankenphp.WithServer(server), bgWorker("once", "publish-once.php", map[string]string{"BG_SENTINEL": sentinel}, server), frankenphp.WithNumThreads(2))

	before := serverGet(t, server, "http://example.com/vars.php?name=once")
	require.JSONEq(t, `{"answer":42,"nested":{"a":1,"list":[true,null,1.5,"x"]}}`, before)

	frankenphp.RestartWorkers()

	assert.JSONEq(t, before, serverGet(t, server, "http://example.com/vars.php?name=once"))
}
