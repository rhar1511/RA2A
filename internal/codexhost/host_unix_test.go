//go:build !windows

package codexhost

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

func TestFinalizeManagedExitSkipsReapedLogWhenGroupAlreadyGone(t *testing.T) {
	root := t.TempDir()
	ownerPath := filepath.Join(root, "owner.json")
	socketPath := filepath.Join(root, "ra2a.sock")

	command := exec.Command("sh", "-c", "exit 0")
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	leaderPid := command.Process.Pid
	if err := writeOwnerRecord(ownerPath, ownerRecord{PID: leaderPid, SocketPath: socketPath}); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	waitErr := command.Wait()
	finalizeManagedExit(command, &output, ownerPath, socketPath, waitErr, nil, false)

	logs := output.String()
	if !strings.Contains(logs, "event=managed_codex_host_exited pid="+strconv.Itoa(leaderPid)+" status=clean") {
		t.Fatalf("finalize logs = %q", logs)
	}
	if strings.Contains(logs, "managed_codex_host_reaped") {
		t.Fatalf("reaped logged for an already-gone group: %q", logs)
	}
	if _, err := os.Stat(ownerPath); !os.IsNotExist(err) {
		t.Fatalf("owner record still exists: %v", err)
	}
}
