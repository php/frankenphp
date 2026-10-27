package caddy

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/dunglas/frankenphp"
	"github.com/stretchr/testify/require"
)

// options collected while provisioning a module, like the Mercure hub, must reach frankenphp
func TestWorkerOptionsKeepProvisionedOptions(t *testing.T) {
	wc := workerConfig{FileName: "../testdata/worker-with-env.php"}
	opts, err := wc.toWorkerOptions()
	require.NoError(t, err)
	base := len(opts)

	wc.options = append(wc.options, frankenphp.WithWorkerMaxThreads(3))
	opts, err = wc.toWorkerOptions()

	require.NoError(t, err)
	require.Len(t, opts, base+1, "worker options set during provisioning must be forwarded")
}

// negative counts are runtime sentinels (auto, default) and must not be reachable from the Caddyfile
func TestAppRejectsNegativeCounts(t *testing.T) {
	for _, directive := range []string{"num_threads -1", "max_threads -1", "max_requests -1", "worker index.php -1", "worker {\n num -1\n }", "worker {\n max_threads -1\n }"} {
		t.Run(directive, func(t *testing.T) {
			d := caddyfile.NewTestDispenser("frankenphp {\n" + directive + "\n}")

			require.ErrorContains(t, (&FrankenPHPApp{}).UnmarshalCaddyfile(d), "must not be negative")
		})
	}
}

func TestModuleRequestBodyTimeout(t *testing.T) {
	d := caddyfile.NewTestDispenser(`
	{
		php {
			request_body_timeout 5s
		}
	}`)
	module := &FrankenPHPModule{}

	require.NoError(t, module.UnmarshalCaddyfile(d))
	require.NotNil(t, module.RequestBodyTimeout)
	require.Equal(t, caddy.Duration(5*time.Second), *module.RequestBodyTimeout)
}

func TestModuleRequestBodyTimeoutDisabled(t *testing.T) {
	d := caddyfile.NewTestDispenser(`
	{
		php {
			request_body_timeout 0
		}
	}`)
	module := &FrankenPHPModule{}

	require.NoError(t, module.UnmarshalCaddyfile(d))
	require.NotNil(t, module.RequestBodyTimeout)
	require.Equal(t, caddy.Duration(0), *module.RequestBodyTimeout)
}

func TestModuleWorkerDuplicateFilenamesFail(t *testing.T) {
	// Create a test configuration with duplicate worker filenames
	configWithDuplicateFilenames := `
	{
		php {
			worker {
				file worker-with-env.php
				num 1
			}
			worker {
				file worker-with-env.php
				num 2
			}
		}
	}`

	// Parse the configuration
	d := caddyfile.NewTestDispenser(configWithDuplicateFilenames)
	module := &FrankenPHPModule{}

	// Unmarshal the configuration
	err := module.UnmarshalCaddyfile(d)

	// Verify that an error was returned
	require.Error(t, err, "Expected an error when two workers in the same module have the same filename")
	require.Contains(t, err.Error(), "must not have duplicate filenames", "Error message should mention duplicate filenames")
}

// two global background workers may share a script, like their php_server
// counterparts and the Go API: they are not matched by path
func TestGlobalBackgroundWorkersShareAFilename(t *testing.T) {
	d := caddyfile.NewTestDispenser(`
	{
		frankenphp {
			worker {
				name first
				file ../testdata/worker-with-env.php
				num 1
				background
			}
			worker {
				name second
				file ../testdata/worker-with-env.php
				num 1
				background
			}
		}
	}`)
	app := &FrankenPHPApp{}

	require.NoError(t, app.UnmarshalCaddyfile(d))
	require.Len(t, app.Workers, 2)
}

func TestGlobalWorkerDuplicateFilenamesFail(t *testing.T) {
	d := caddyfile.NewTestDispenser(`
	{
		frankenphp {
			worker ../testdata/worker-with-env.php
			worker ../testdata/worker-with-env.php
		}
	}`)
	app := &FrankenPHPApp{}

	require.ErrorContains(t, app.UnmarshalCaddyfile(d), "must not have duplicate filenames")
}

func TestModuleWorkersWithDifferentFilenames(t *testing.T) {
	// Create a test configuration with different worker filenames
	configWithDifferentFilenames := `
	{
		php {
			worker ../testdata/worker-with-env.php
			worker ../testdata/worker-with-counter.php
		}
	}`

	// Parse the configuration
	d := caddyfile.NewTestDispenser(configWithDifferentFilenames)
	module := &FrankenPHPModule{}

	// Unmarshal the configuration
	err := module.UnmarshalCaddyfile(d)

	// Verify that no error was returned
	require.NoError(t, err, "Expected no error when two workers in the same module have different filenames")

	// Verify that both workers were added to the module
	require.Len(t, module.Workers, 2, "Expected two workers to be added to the module")
	require.Equal(t, "../testdata/worker-with-env.php", module.Workers[0].FileName, "First worker should have the correct filename")
	require.Equal(t, "../testdata/worker-with-counter.php", module.Workers[1].FileName, "Second worker should have the correct filename")
}

func TestModuleWorkersDifferentNamesSucceed(t *testing.T) {
	// Create a test configuration with a worker name
	configWithWorkerName1 := `
	{
		php_server {
			worker {
				name test-worker-1
				file ../testdata/worker-with-env.php
				num 1
			}
		}
	}`

	// Parse the first configuration
	d1 := caddyfile.NewTestDispenser(configWithWorkerName1)
	module1 := &FrankenPHPModule{}

	// Unmarshal the first configuration
	err := module1.UnmarshalCaddyfile(d1)
	require.NoError(t, err, "First module should be configured without errors")

	// Create a second test configuration with a different worker name
	configWithWorkerName2 := `
	{
		php_server {
			worker {
				name test-worker-2
				file ../testdata/worker-with-env.php
				num 1
			}
		}
	}`

	// Parse the second configuration
	d2 := caddyfile.NewTestDispenser(configWithWorkerName2)
	module2 := &FrankenPHPModule{}

	// Unmarshal the second configuration
	err = module2.UnmarshalCaddyfile(d2)

	// Verify that no error was returned
	require.NoError(t, err, "Expected no error when two workers have different names")
}

func TestModuleWorkerWithEnvironmentVariables(t *testing.T) {
	// Create a test configuration with environment variables
	configWithEnv := `
	{
		php {
			worker {
				file ../testdata/worker-with-env.php
				num 1
				env APP_ENV production
				env DEBUG true
			}
		}
	}`

	// Parse the configuration
	d := caddyfile.NewTestDispenser(configWithEnv)
	module := &FrankenPHPModule{}

	// Unmarshal the configuration
	err := module.UnmarshalCaddyfile(d)

	// Verify that no error was returned
	require.NoError(t, err, "Expected no error when configuring a worker with environment variables")

	// Verify that the worker was added to the module
	require.Len(t, module.Workers, 1, "Expected one worker to be added to the module")
	require.Equal(t, "../testdata/worker-with-env.php", module.Workers[0].FileName, "Worker should have the correct filename")

	// Verify that the environment variables were set correctly
	require.Len(t, module.Workers[0].Env, 2, "Expected two environment variables")
	require.Equal(t, "production", module.Workers[0].Env["APP_ENV"], "APP_ENV should be set to production")
	require.Equal(t, "true", module.Workers[0].Env["DEBUG"], "DEBUG should be set to true")
}

func TestModuleWorkerWithWatchConfiguration(t *testing.T) {
	// Create a test configuration with watch directories
	configWithWatch := `
	{
		php {
			worker {
				file ../testdata/worker-with-env.php
				num 1
				watch
				watch ./src/**/*.php
				watch ./config/**/*.yaml
			}
		}
	}`

	// Parse the configuration
	d := caddyfile.NewTestDispenser(configWithWatch)
	module := &FrankenPHPModule{}

	// Unmarshal the configuration
	err := module.UnmarshalCaddyfile(d)

	// Verify that no error was returned
	require.NoError(t, err, "Expected no error when configuring a worker with watch directories")

	// Verify that the worker was added to the module
	require.Len(t, module.Workers, 1, "Expected one worker to be added to the module")
	require.Equal(t, "../testdata/worker-with-env.php", module.Workers[0].FileName, "Worker should have the correct filename")

	// Verify that the watch directories were set correctly
	require.Len(t, module.Workers[0].Watch, 3, "Expected three watch patterns")
	require.Equal(t, defaultWatchPattern, module.Workers[0].Watch[0], "First watch pattern should be the default")
	require.Equal(t, "./src/**/*.php", module.Workers[0].Watch[1], "Second watch pattern should match the configuration")
	require.Equal(t, "./config/**/*.yaml", module.Workers[0].Watch[2], "Third watch pattern should match the configuration")
}

func TestModuleWorkerWithCustomName(t *testing.T) {
	// Create a test configuration with a custom worker name
	configWithCustomName := `
	{
		php {
			worker {
				file ../testdata/worker-with-env.php
				num 1
				name custom-worker-name
			}
		}
	}`

	// Parse the configuration
	d := caddyfile.NewTestDispenser(configWithCustomName)
	module := &FrankenPHPModule{}

	// Unmarshal the configuration
	err := module.UnmarshalCaddyfile(d)

	// Verify that no error was returned
	require.NoError(t, err, "Expected no error when configuring a worker with a custom name")

	// Verify that the worker was added to the module
	require.Len(t, module.Workers, 1, "Expected one worker to be added to the module")
	require.Equal(t, "../testdata/worker-with-env.php", module.Workers[0].FileName, "Worker should have the correct filename")
}

func TestCreateUniqueWorkerNames(t *testing.T) {
	usedWorkerNames := map[string]bool{}
	filename := "../testdata/worker-with-env.php"
	absFileName, _ := filepath.Abs(filename)
	names := make([]string, 6)
	for i := range 3 {
		names[i] = createUniqueWorkerName(usedWorkerNames, workerConfig{
			FileName: filename,
			Name:     "custom-worker-name",
		}, "")
		names[i+3] = createUniqueWorkerName(usedWorkerNames, workerConfig{
			FileName: filename,
		}, "")
	}

	require.Equal(t, "custom-worker-name", names[0])
	require.Equal(t, "custom-worker-name_1", names[1])
	require.Equal(t, "custom-worker-name_2", names[2])
	require.Equal(t, absFileName, names[3])
	require.Equal(t, absFileName+"_1", names[4])
	require.Equal(t, absFileName+"_2", names[5])
}

func TestCreateUniqueWorkerNamesQualifiedByServer(t *testing.T) {
	usedWorkerNames := map[string]bool{}
	wc := workerConfig{FileName: "../testdata/worker-with-env.php", Name: "queue"}

	require.Equal(t, "queue", createUniqueWorkerName(usedWorkerNames, wc, "one.example.com"))
	// on collision, the name is qualified with the server name
	require.Equal(t, "two.example.com:queue", createUniqueWorkerName(usedWorkerNames, wc, "two.example.com"))
	// when the qualified name is also taken, fall back to the numeric postfix
	require.Equal(t, "queue_1", createUniqueWorkerName(usedWorkerNames, wc, "two.example.com"))
	// workers without a server keep the numeric postfix behavior
	require.Equal(t, "queue_2", createUniqueWorkerName(usedWorkerNames, wc, ""))
}

func TestWorkerBackgroundConfig(t *testing.T) {
	d := caddyfile.NewTestDispenser(`
	{
		php_server {
			worker {
				name jobs
				file ../testdata/worker-with-env.php
				num 2
				background
			}
		}
	}`)
	module := &FrankenPHPModule{}

	require.NoError(t, module.UnmarshalCaddyfile(d))
	require.Len(t, module.Workers, 1)
	require.True(t, module.Workers[0].Background)
	require.Equal(t, "jobs", module.Workers[0].Name)
}

// the name is generated when the workers are collected, like for an HTTP worker
func TestWorkerBackgroundNameIsOptional(t *testing.T) {
	d := caddyfile.NewTestDispenser(`
	{
		php_server {
			worker {
				file ../testdata/worker-with-env.php
				background
			}
		}
	}`)
	module := &FrankenPHPModule{}

	require.NoError(t, module.UnmarshalCaddyfile(d))
	require.True(t, module.Workers[0].Background)
	require.Empty(t, module.Workers[0].Name)
}

func TestWorkerBackgroundVars(t *testing.T) {
	d := caddyfile.NewTestDispenser(`
	{
		php_server {
			worker {
				file ../testdata/worker-with-env.php
				background
				vars redis feature_flags
			}
		}
	}`)
	module := &FrankenPHPModule{}

	require.NoError(t, module.UnmarshalCaddyfile(d))
	require.Equal(t, []string{"redis", "feature_flags"}, module.Workers[0].Vars)
}

func TestWorkerVarsRequireBackground(t *testing.T) {
	d := caddyfile.NewTestDispenser(`
	{
		php_server {
			worker {
				file ../testdata/worker-with-env.php
				vars redis
			}
		}
	}`)
	module := &FrankenPHPModule{}

	require.ErrorContains(t, module.UnmarshalCaddyfile(d), `"vars" is only supported for background workers`)
}

func TestWorkerBackgroundWithoutNumParses(t *testing.T) {
	d := caddyfile.NewTestDispenser(`
	{
		php_server {
			worker {
				name jobs
				file ../testdata/worker-with-env.php
				background
			}
		}
	}`)
	module := &FrankenPHPModule{}

	// num is optional, it defaults to one thread when the workers start
	require.NoError(t, module.UnmarshalCaddyfile(d))
}

func TestWorkerBootTimeout(t *testing.T) {
	parse := func(t *testing.T, directives string) (*FrankenPHPModule, error) {
		t.Helper()
		module := &FrankenPHPModule{}
		err := module.UnmarshalCaddyfile(caddyfile.NewTestDispenser(`
	{
		php_server {
			worker {
				file ../testdata/worker-with-env.php
				` + directives + `
			}
		}
	}`))

		return module, err
	}

	module, err := parse(t, "background\nboot_timeout 5s")
	require.NoError(t, err)
	require.Equal(t, caddy.Duration(5*time.Second), *module.Workers[0].BootTimeout)

	// zero waits forever
	module, err = parse(t, "background\nboot_timeout 0")
	require.NoError(t, err)
	require.Equal(t, caddy.Duration(0), *module.Workers[0].BootTimeout)

	_, err = parse(t, "boot_timeout 5s")
	require.ErrorContains(t, err, `"boot_timeout" is only supported for background workers`)

	_, err = parse(t, "background\nboot_timeout -1s")
	require.ErrorContains(t, err, "boot_timeout must be >= 0")
}

func TestWorkerBackgroundRejectsMatch(t *testing.T) {
	d := caddyfile.NewTestDispenser(`
	{
		php_server {
			worker {
				name jobs
				file ../testdata/worker-with-env.php
				match /jobs/*
				background
			}
		}
	}`)
	module := &FrankenPHPModule{}

	err := module.UnmarshalCaddyfile(d)
	require.ErrorContains(t, err, `"match" is not supported for background workers`)
}
