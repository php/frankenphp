package caddy

import (
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/stretchr/testify/require"
	"path/filepath"
	"testing"
	"time"
)

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
	app := &FrankenPHPApp{}
	filename := "../testdata/worker-with-env.php"
	absFileName, _ := filepath.Abs(filename)
	names := make([]string, 6)
	for i := 0; i < 3; i++ {
		names[i] = app.createUniqueWorkerName(workerConfig{
			FileName: filename,
			Name:     "custom-worker-name",
		})
		names[i+3] = app.createUniqueWorkerName(workerConfig{
			FileName: filename,
		})
	}

	require.Equal(t, "custom-worker-name", names[0])
	require.Equal(t, "custom-worker-name_1", names[1])
	require.Equal(t, "custom-worker-name_2", names[2])
	require.Equal(t, absFileName, names[3])
	require.Equal(t, absFileName+"_1", names[4])
	require.Equal(t, absFileName+"_2", names[5])
}

func TestModuleWorkerWithPingConfiguration(t *testing.T) {
	configWithPing := `
	{
		php {
			worker ../testdata/worker-with-counter.php {
				ping 60s health
				ping minutely each message
			}
		}
	}`

	d := caddyfile.NewTestDispenser(configWithPing)
	module := &FrankenPHPModule{}

	err := module.UnmarshalCaddyfile(d)
	require.NoError(t, err)
	require.Len(t, module.Workers, 1)
	require.Equal(t, "../testdata/worker-with-counter.php", module.Workers[0].FileName)
	require.Len(t, module.Workers[0].Pings, 2)
	require.Equal(t, 60*time.Second, module.Workers[0].Pings[0].Interval)
	require.Equal(t, "health", module.Workers[0].Pings[0].Message)
	require.False(t, module.Workers[0].Pings[0].Aligned)
	require.False(t, module.Workers[0].Pings[0].Each)
	require.Equal(t, time.Minute, module.Workers[0].Pings[1].Interval)
	require.Equal(t, "message", module.Workers[0].Pings[1].Message)
	require.True(t, module.Workers[0].Pings[1].Aligned)
	require.True(t, module.Workers[0].Pings[1].Each)
}

func TestModuleWorkerWithInvalidPingConfiguration(t *testing.T) {
	tests := []struct {
		name   string
		config string
	}{
		{
			name: "missing message",
			config: `{
				php {
					worker {
						file ../testdata/worker-with-counter.php
						ping 60s
					}
				}
			}`,
		},
		{
			name: "invalid interval",
			config: `{
				php {
					worker {
						file ../testdata/worker-with-counter.php
						ping not-a-duration health
					}
				}
			}`,
		},
		{
			name: "each must come first",
			config: `{
				php {
					worker {
						file ../testdata/worker-with-counter.php
						ping 60s health each
					}
				}
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := caddyfile.NewTestDispenser(tt.config)
			module := &FrankenPHPModule{}
			err := module.UnmarshalCaddyfile(d)
			require.Error(t, err)
		})
	}
}
