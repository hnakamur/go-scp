package scp

import (
	"path/filepath"
	"testing"
)

func TestRegulatePath(t *testing.T) {

	destinationFromWindows := "/home/ubuntu/destination"
	cleaned := filepath.Clean(destinationFromWindows) // `\home\ubuntu\destination`

	expected := destinationFromWindows

	if real := realPath(cleaned); real != expected {
		t.Errorf("%q.realPath() = %q , expected = %q", destinationFromWindows, real, expected)
	}
}
