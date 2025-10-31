package tests

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	_ = os.Setenv("GH_TOKEN", "test-token")
	_ = os.Setenv("GITHUB_TOKEN", "test-token")
	os.Exit(m.Run())
}
