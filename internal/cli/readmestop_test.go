package cli

import (
	"os"
	"strings"
	"testing"
)

// v0.4.4 made dev stop the server on Windows with Ctrl+Break, so its shutdown
// code runs (TestDevStopLetsTheServerShutDown proves it on every system CI
// runs). The README still said that on Windows the process is killed at once.
func TestReadmeDescribesHowDevStopsTheServer(t *testing.T) {
	b, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	readme := string(b)
	if strings.Contains(readme, "killed at once") {
		t.Error("the README says dev kills the server at once on Windows")
	}
	if !strings.Contains(readme, "Ctrl+Break") {
		t.Error("the README does not say how dev stops the server on Windows")
	}
}
