package userservice

import "testing"

func TestGSS015StorageRejectsRuntimeOnlyMask(t *testing.T) {
	base := "LoadState=masked\nActiveState=inactive\nSubState=dead\nJob=\nUnitFileState="
	for _, tc := range []struct{ value, want string }{
		{"masked", ""},
		{"masked-runtime", "stop and disable"},
	} {
		err := stoppedSystemdUnit(func(string, ...string) ([]byte, error) {
			return []byte(base + tc.value), nil
		}, "com.gtmux.serve.service")
		wantStorageManagerResult(t, err, tc.want)
	}
}
