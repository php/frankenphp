package frankenphp_test

import (
	"path/filepath"
	"strings"
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
	assert.Contains(t, serverGet(t, server, "http://example.com/vars.php?ns=nope"), `RuntimeException: frankenphp_get_vars(): no background worker publishes the vars namespace "nope"`)

	// a name nobody declares may come from the request: the message shows its start
	long := serverGet(t, server, "http://example.com/vars.php?ns="+strings.Repeat("x", 5000))
	assert.Contains(t, long, `the vars namespace "`+strings.Repeat("x", 64)+`"...`)
	assert.Less(t, len(long), 300)
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

// between two requests, a worker serving any server still reads the vars of
// the one it just served
func TestVarsBetweenTwoRequests(t *testing.T) {
	server1, _ := frankenphp.NewServer(testDataDir, frankenphp.WithServerName("one"))
	server2, _ := frankenphp.NewServer(testDataDir, frankenphp.WithServerName("two"))
	publisher := func(name, value string, server *frankenphp.Server) frankenphp.Option {
		return frankenphp.WithWorkers(name, "testdata/bgworker/publisher.php", 1,
			frankenphp.WithWorkerBackground(),
			frankenphp.WithWorkerVars("cfg"),
			frankenphp.WithWorkerEnv(map[string]string{"BG_PUBLISH_VALUE": value}),
			frankenphp.WithWorkerServerScope(server),
		)
	}
	initServers(t,
		frankenphp.WithServer(server1),
		frankenphp.WithServer(server2),
		publisher("cfg-one", "one", server1),
		publisher("cfg-two", "two", server2),
		frankenphp.WithWorkers("reader", testDataDir+"vars-between-requests.php", 1),
		frankenphp.WithNumThreads(4),
	)

	url := "http://example.com/vars-between-requests.php"
	assert.JSONEq(t, `{"request":"one","after":null}`, serverGet(t, server1, url))
	assert.JSONEq(t, `{"request":"two","after":"one"}`, serverGet(t, server2, url))
	assert.JSONEq(t, `{"request":"one","after":"two"}`, serverGet(t, server1, url))
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

// a worker reading its own namespace before its first tick would wait for
// itself: refused with a message saying so
func TestVarsOwnReadBeforeTheFirstTick(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "self.txt")
	initServers(t, bgWorker("self", "consumer.php", map[string]string{"BG_CONSUME": "self", "BG_SENTINEL": sentinel}, nil), frankenphp.WithNumThreads(2))

	assert.Contains(t, requireFileContentEventually(t, sentinel), `background worker "self" reads its own vars before its first tick`)
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
	assert.Contains(t, results, `{"consumed":"LogicException: frankenphp_get_vars(): circular dependency`)
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
	assert.Contains(t, result, `ValueError: FrankenPHP\WorkerHandle::setVars(): values must be null, scalars, arrays or enums`)
	assert.Contains(t, result, `ValueError: FrankenPHP\WorkerHandle::setVars(): values must fit in memory_limit once copied`)
	assert.Contains(t, result, `LogicException: FrankenPHP\WorkerHandle::setVars(): this worker does not publish the vars namespace "undeclared"`)
}

// a snapshot fits in the memory_limit of its publisher, not always in the one
// of its readers: their copy is refused before it starts
func TestVarsLargerThanTheMemoryLimitOfTheReader(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), bgWorker("big", "publisher.php", map[string]string{"BG_PUBLISH_BLOB_MB": "32"}, server), frankenphp.WithNumThreads(2))

	body := serverGet(t, server, "http://example.com/vars-over-limit.php")
	assert.Contains(t, body, "RuntimeException: FrankenPHP: the values to copy take 33")
	assert.Contains(t, body, "more than what is left of memory_limit")
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
	), `two global workers cannot publish the same vars namespace: "cfg", declared by "one" and "two"`)

	// one publisher per server
	other, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	assert.NoError(t, frankenphp.Validate(
		frankenphp.WithServer(server),
		frankenphp.WithServer(other),
		publisher("one", 1, frankenphp.WithWorkerVars("cfg"), frankenphp.WithWorkerServerScope(server)),
		publisher("two", 1, frankenphp.WithWorkerVars("cfg"), frankenphp.WithWorkerServerScope(other)),
	))

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
	), `background worker "pool" publishes vars, so it runs a single thread, got num 2`)

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

// a bailout while copying a snapshot, here in an autoloader its enum
// triggers, lets go of it: the next readers are served, by a publisher that
// keeps publishing. memory_limit would do too, but not where the Zend
// allocator is off, as under the sanitizers.
func TestVarsBailoutInTheCopy(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), bgWorker("enum", "enum-vars.php", nil, server), frankenphp.WithNumThreads(2))

	assert.Contains(t, serverGet(t, server, "http://example.com/vars-fatal.php?fatal=1"), "Cannot redeclare")

	first := serverGet(t, server, "http://example.com/vars-fatal.php")
	require.Eventually(t, func() bool {
		return serverGet(t, server, "http://example.com/vars-fatal.php") != first
	}, 5*time.Second, 25*time.Millisecond, "the publisher is stuck")
}

// an autoloader may keep the class name it is given: a request string, not
// the one of the snapshot, which the next publication frees
func TestVarsEnumClassNameOutlivesTheSnapshot(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), bgWorker("enum", "enum-vars.php", nil, server), frankenphp.WithNumThreads(2))

	assert.Equal(t, "Hearts kept:EnumSuit", serverGet(t, server, "http://example.com/vars-enum.php"))
}

// an HTTP worker reading vars in its bootstrap gives up once the start
// failed, here because the publisher never publishes, instead of keeping
// Init() waiting
func TestVarsWaitEndsWhenTheStartFails(t *testing.T) {
	t.Cleanup(frankenphp.Shutdown)
	done := make(chan error, 1)
	go func() {
		done <- frankenphp.Init(
			bgWorker("silent", "basic.php", nil, nil, frankenphp.WithWorkerMaxFailures(1)),
			frankenphp.WithWorkers("reader", "testdata/vars-at-boot.php", 1),
			frankenphp.WithNumThreads(3),
		)
	}()

	select {
	case err := <-done:
		require.ErrorContains(t, err, "keeps crashing")
	case <-time.After(10 * time.Second):
		t.Fatal("Init() did not return")
	}
}

// a worker whose boot fails because the start did is not run again, even one
// that never gives up restarting: Init() would wait for it for ever
func TestStartFailureStopsTheWorkersStillBooting(t *testing.T) {
	t.Cleanup(frankenphp.Shutdown)
	done := make(chan error, 1)
	go func() {
		done <- frankenphp.Init(
			bgWorker("silent", "basic.php", nil, nil, frankenphp.WithWorkerMaxFailures(1)),
			frankenphp.WithWorkers("reader", "testdata/vars-at-boot.php", 1,
				frankenphp.WithWorkerEnv(map[string]string{"VARS_UNCAUGHT": "1"}),
				frankenphp.WithWorkerMaxFailures(-1),
			),
			frankenphp.WithNumThreads(3),
		)
	}()

	select {
	case err := <-done:
		require.ErrorContains(t, err, "keeps crashing")
	case <-time.After(10 * time.Second):
		t.Fatal("Init() did not return")
	}
}

// the namespaces of a run go with it, even when the next run has no workers
func TestVarsOwnersEndWithTheRun(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), bgWorker("publisher", "publisher.php", nil, server), frankenphp.WithNumThreads(2))
	require.Contains(t, serverGet(t, server, "http://example.com/vars.php?ns=publisher"), `"answer":42`)
	frankenphp.Shutdown()

	initServers(t, frankenphp.WithServer(server), frankenphp.WithNumThreads(1))
	assert.Contains(t, serverGet(t, server, "http://example.com/vars.php?ns=publisher"), `no background worker publishes the vars namespace "publisher"`)
}

// a worker of a server falls back to the global namespace like a request does
func TestVarsServerWorkerReadsAGlobalNamespace(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "consumer.json")
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t,
		frankenphp.WithServer(server),
		bgWorker("publisher", "publisher.php", map[string]string{"BG_PUBLISH_VALUE": "global"}, nil),
		bgWorker("consumer", "consumer.php", map[string]string{"BG_CONSUME": "publisher", "BG_SENTINEL": sentinel}, server),
		frankenphp.WithNumThreads(3),
	)

	assert.Contains(t, requireFileContentEventually(t, sentinel), `"value":"global"`)
}

// an autoloader may suspend the fiber reading vars in the middle of the
// copy: each read lets go of the snapshot it holds, whatever the order the
// fibers resume in
func TestVarsReadsInterleavedByFibers(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t,
		frankenphp.WithServer(server),
		bgWorker("enum-a", "enum-vars.php", nil, server),
		bgWorker("enum-b", "enum-vars.php", map[string]string{"BG_ENUM": "EnumRank::Ace"}, server),
		frankenphp.WithNumThreads(3),
	)

	for range 5 {
		require.Equal(t, "Hearts Ace", serverGet(t, server, "http://example.com/vars-fibers.php"))
	}
}

// a snapshot written against another version of an enum fails the read with
// an exception, whatever the reader's class has instead of the case
func TestVarsEnumCaseMissingInTheReader(t *testing.T) {
	server, err := frankenphp.NewServer(testDataDir)
	require.NoError(t, err)
	initServers(t, frankenphp.WithServer(server), bgWorker("enum", "enum-vars.php", nil, server), frankenphp.WithNumThreads(2))

	assert.Equal(t, `LogicException: persistent_zval: enum case "EnumSuit::Hearts" not found`, serverGet(t, server, "http://example.com/vars-enum-mismatch.php"))
	assert.Equal(t, `LogicException: persistent_zval: enum case "EnumSuit::Hearts" not found`, serverGet(t, server, "http://example.com/vars-enum-mismatch.php?constant=1"))
}
