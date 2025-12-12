package caddy

import (
	"testing"

	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/stretchr/testify/require"
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
	app := &FrankenPHPApp{}
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

	_, err = app.addModuleWorkers(module1.Workers...)
	require.NoError(t, err, "Expected no error when adding the first module workers")
	_, err = app.addModuleWorkers(module2.Workers...)
	require.NoError(t, err, "Expected no error when adding the second module workers")

	// Verify that both workers were added
	require.Len(t, app.Workers, 2, "Expected two workers in the app")
	require.Equal(t, "m#test-worker-1", app.Workers[0].Name, "First worker should have the correct name")
	require.Equal(t, "m#test-worker-2", app.Workers[1].Name, "Second worker should have the correct name")
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
	app := &FrankenPHPApp{}

	// Unmarshal the configuration
	err := module.UnmarshalCaddyfile(d)

	// Verify that no error was returned
	require.NoError(t, err, "Expected no error when configuring a worker with a custom name")

	// Verify that the worker was added to the module
	require.Len(t, module.Workers, 1, "Expected one worker to be added to the module")
	require.Equal(t, "../testdata/worker-with-env.php", module.Workers[0].FileName, "Worker should have the correct filename")

	// Verify that the worker was added to app.Workers with the m# prefix
	module.Workers, err = app.addModuleWorkers(module.Workers...)
	require.NoError(t, err, "Expected no error when adding the worker to the app")
	require.Equal(t, "m#custom-worker-name", module.Workers[0].Name, "Worker should have the custom name, prefixed with m#")
	require.Equal(t, "m#custom-worker-name", app.Workers[0].Name, "Worker should have the custom name, prefixed with m#")
}
