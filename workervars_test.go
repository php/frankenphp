package frankenphp_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bgWorker declares a background worker from testdata/bgworker publishing the
// vars namespace named like it, scoped to server when one is given
func bgWorker(name, file string, env map[string]string, server *frankenphp.Server, extra ...frankenphp.WorkerOption) frankenphp.Option {
	opts := []frankenphp.WorkerOption{frankenphp.WithWorkerBackground(), frankenphp.WithWorkerEnv(env), frankenphp.WithWorkerVars(name)}
	if server != nil {
		opts = append(opts, frankenphp.WithWorkerServerScope(server))
	}

	return frankenphp.WithWorkers(name, "testdata/bgworker/"+file, 1, append(opts, extra...)...)
}

func TestVarsRoundTrip(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), bgWorker("publisher", "publisher.php", nil, server), frankenphp.WithNumThreads(2))

	body := serverGet(t, server, "http://example.com/vars.php?ns=publisher")
	assert.JSONEq(t, `{"answer":42,"value":"default","nested":{"a":1,"list":[true,null,1.5,"x"]},"worker":"publisher"}`, body)

	// a second read is a fresh copy of the same snapshot
	assert.JSONEq(t, body, serverGet(t, server, "http://example.com/vars.php?ns=publisher"))
	assert.Contains(t, serverGet(t, server, "http://example.com/vars.php?ns=nope"), `no background worker publishes the vars namespace "nope"`)
}

// one worker publishes several namespaces, and readers address the
// namespace, never the worker
func TestVarsNamespaces(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t,
		frankenphp.WithServer(server),
		frankenphp.WithWorkers("config", "testdata/bgworker/multi-vars.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerServerScope(server),
			frankenphp.WithWorkerVars("redis", "feature_flags"),
		),
		frankenphp.WithNumThreads(2),
	)

	all := `{"vars":["redis","feature_flags"]}`
	assert.JSONEq(t, `{"namespace":"redis","all":`+all+`}`, serverGet(t, server, "http://example.com/vars.php?ns=redis"))
	assert.JSONEq(t, `{"namespace":"feature_flags","all":`+all+`}`, serverGet(t, server, "http://example.com/vars.php?ns=feature_flags"))
	assert.Contains(t, serverGet(t, server, "http://example.com/vars.php?ns=config"), "no background worker publishes")
}

// a namespace resolves within the server of the reader first, then among the
// global workers
func TestVarsScopedToServer(t *testing.T) {
	server1, _ := frankenphp.NewServer(testDataDir, frankenphp.WithServerName("one"))
	server2, _ := frankenphp.NewServer(testDataDir, frankenphp.WithServerName("two"))
	server3, _ := frankenphp.NewServer(testDataDir, frankenphp.WithServerName("three"))
	// worker names are global, the namespace is what the three share
	publisher := func(name, value string, server *frankenphp.Server) frankenphp.Option {
		opts := []frankenphp.WorkerOption{
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerVars("cfg"),
			frankenphp.WithWorkerEnv(map[string]string{"BG_PUBLISH_VALUE": value}),
		}
		if server != nil {
			opts = append(opts, frankenphp.WithWorkerServerScope(server))
		}

		return frankenphp.WithWorkers(name, "testdata/bgworker/publisher.php", 1, opts...)
	}
	initServers(t,
		frankenphp.WithServer(server1),
		frankenphp.WithServer(server2),
		frankenphp.WithServer(server3),
		publisher("cfg-one", "one", server1),
		publisher("cfg-two", "two", server2),
		publisher("cfg-global", "global", nil),
		frankenphp.WithNumThreads(3),
	)

	assert.Contains(t, serverGet(t, server1, "http://example.com/vars.php?ns=cfg"), `"value":"one"`)
	assert.Contains(t, serverGet(t, server2, "http://example.com/vars.php?ns=cfg"), `"value":"two"`)
	assert.Contains(t, serverGet(t, server3, "http://example.com/vars.php?ns=cfg"), `"value":"global"`)
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

	// one side sees the cycle and publishes the exception, the other reads it
	results := requireFileContentEventually(t, s1) + "\n" + requireFileContentEventually(t, s2)
	assert.Contains(t, results, "circular dependency")
	assert.Contains(t, results, `{"consumed":"RuntimeException: frankenphp_get_vars(): circular dependency`)
}

// the first tick of a run waits for every declared namespace, so readers
// never ask a ready worker for a snapshot it does not have
func TestVarsArePublishedBeforeTheFirstTick(t *testing.T) {
	t.Cleanup(frankenphp.Shutdown)

	err := frankenphp.Init(bgWorker("silent", "basic.php", nil, nil, frankenphp.WithWorkerMaxFailures(1)), frankenphp.WithNumThreads(1))
	require.Error(t, err)
}

func TestVarsRejectInvalidPublications(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "bad.txt")
	initServers(t, bgWorker("bad", "bad-vars.php", map[string]string{"BG_SENTINEL": sentinel}, nil), frankenphp.WithNumThreads(2))

	result := requireFileContentEventually(t, sentinel)
	assert.Contains(t, result, "ValueError")
	assert.Contains(t, result, "must be null, scalars, arrays or enums")
	assert.Contains(t, result, `LogicException: FrankenPHP\WorkerHandle::setVars(): this worker does not publish the vars namespace "undeclared"`)
}

// a reboot restarts the PHP engine, and readers keep getting the snapshot
// published before it until the worker publishes again: it must not point
// into the memory of the engine that is gone
func TestVarsSurviveAReboot(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	env := map[string]string{"BG_SENTINEL": filepath.Join(t.TempDir(), "published"), "BG_REPUBLISH_DELAY_MS": "2000"}
	initServers(t, frankenphp.WithServer(server), bgWorker("once", "republish.php", env, server), frankenphp.WithNumThreads(2))

	before := serverGet(t, server, "http://example.com/vars.php?ns=once")
	require.JSONEq(t, `{"answer":42,"nested":{"a":1,"list":[true,null,1.5,"x"]}}`, before)

	rebooted := make(chan struct{})
	go func() {
		frankenphp.RestartWorkers()
		close(rebooted)
	}()

	// the request threads come back while the worker still waits to publish
	time.Sleep(500 * time.Millisecond)
	assert.JSONEq(t, before, serverGet(t, server, "http://example.com/vars.php?ns=once"))

	<-rebooted
	assert.JSONEq(t, before, serverGet(t, server, "http://example.com/vars.php?ns=once"))
}

func TestValidateVarsNamespaces(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	publisher := func(name string, num int, extra ...frankenphp.WorkerOption) frankenphp.Option {
		return frankenphp.WithWorkers(name, "testdata/bgworker/publisher.php", num, append([]frankenphp.WorkerOption{frankenphp.WithWorkerBackground()}, extra...)...)
	}

	assert.ErrorContains(t, frankenphp.Validate(
		publisher("one", 1, frankenphp.WithWorkerVars("cfg")),
		publisher("two", 1, frankenphp.WithWorkerVars("other", "cfg")),
	), `two global workers cannot publish the same vars namespace: "cfg"`)

	// the same namespace in a server and globally: the server's comes first for its requests
	assert.NoError(t, frankenphp.Validate(
		frankenphp.WithServer(server),
		publisher("one", 1, frankenphp.WithWorkerVars("cfg")),
		publisher("two", 1, frankenphp.WithWorkerVars("cfg"), frankenphp.WithWorkerServerScope(server)),
	))

	assert.ErrorContains(t, frankenphp.Validate(
		publisher("one", 1, frankenphp.WithWorkerVars("cfg", "cfg")),
	), `declares the vars namespace "cfg" twice`)

	assert.ErrorContains(t, frankenphp.Validate(
		publisher("pool", 2, frankenphp.WithWorkerVars("cfg")),
	), "publishes vars, it runs a single thread")

	assert.ErrorContains(t, frankenphp.Validate(
		frankenphp.WithWorkers("http", "testdata/worker.php", 1, frankenphp.WithWorkerVars("cfg")),
	), "only background workers publish them")
}

// the rules are the ones Init() applies
func TestInitRefusesASharedVarsNamespace(t *testing.T) {
	t.Cleanup(frankenphp.Shutdown)

	err := frankenphp.Init(
		bgWorker("cfg", "publisher.php", nil, nil),
		frankenphp.WithWorkers("other", "testdata/bgworker/publisher.php", 1, frankenphp.WithWorkerBackground(), frankenphp.WithWorkerVars("cfg")),
		frankenphp.WithNumThreads(1),
	)
	require.ErrorContains(t, err, `two global workers cannot publish the same vars namespace: "cfg"`)
}
