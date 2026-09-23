package testext

import (
	"os"
	"os/exec"
	"testing"
)

func TestRegisterExtension(t *testing.T) {
	const childEnv = "FRANKENPHP_TEST_EXTENSION_CHILD"
	if os.Getenv(childEnv) == "1" {
		testRegisterExtension(t)
		return
	}

	// go test omits a Windows process's exit status once it has printed any
	// output. Run the native extension test in a child so a crash reports both
	// its exit status and the last reached stage instead of just "unknown".
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.v", "-test.run=^TestRegisterExtension$", "-test.timeout=1m")
	cmd.Env = append(os.Environ(), childEnv+"=1")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("extension test subprocess failed: %v\n%s", err, output)
	}
	t.Logf("extension test subprocess:\n%s", output)
}
