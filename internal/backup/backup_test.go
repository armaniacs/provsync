package backup

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecordAndRestore(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.json")
	if err := os.WriteFile(path, []byte("orig"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := New(filepath.Join(dir, "state"))

	op, err := st.Record("push opencode", []string{path})
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if err := os.WriteFile(path, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}

	found, err := st.Find("")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if found.ID != op.ID {
		t.Fatalf("find returned %s, want %s", found.ID, op.ID)
	}
	if _, err := st.Restore(found); err != nil {
		t.Fatalf("restore: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "orig" {
		t.Errorf("content = %q, want orig", data)
	}

	ops, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	if ops[0].Command != "undo "+op.ID {
		t.Errorf("newest op = %q, want undo of original", ops[0].Command)
	}
	if !ops[1].Undone {
		t.Error("original operation should be marked undone")
	}
}

func TestRestoreRemovesCreatedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	st := New(filepath.Join(dir, "state"))

	op, err := st.Record("pull kilocode", []string{path})
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	if err := os.WriteFile(path, []byte("created"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Restore(op); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("created file should be removed, stat err = %v", err)
	}
}

func TestRetention(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.json")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := New(filepath.Join(dir, "state"))
	for i := 0; i < MaxOperations+5; i++ {
		if _, err := st.Record("op", []string{path}); err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
	}
	ops, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != MaxOperations {
		t.Errorf("len = %d, want %d", len(ops), MaxOperations)
	}
}

func TestSetMax(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.json")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := New(filepath.Join(dir, "state"))
	st.SetMax(5)
	for i := 0; i < 8; i++ {
		if _, err := st.Record("op", []string{path}); err != nil {
			t.Fatal(err)
		}
	}
	ops, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 5 {
		t.Errorf("len = %d, want 5", len(ops))
	}
}

func TestPrune(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.json")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(dir, "state")
	st := New(stateDir)
	for i := 0; i < 6; i++ {
		if _, err := st.Record("op", []string{path}); err != nil {
			t.Fatal(err)
		}
	}
	removed, err := st.Prune(2)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if removed != 4 {
		t.Errorf("removed = %d, want 4", removed)
	}
	ops, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 2 {
		t.Errorf("len = %d, want 2", len(ops))
	}
	entries, err := os.ReadDir(filepath.Join(stateDir, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Errorf("backup dirs = %d, want 2", len(entries))
	}
	// 冪等: 再実行では何も消えない
	removed, err = st.Prune(2)
	if err != nil {
		t.Fatalf("prune 2nd: %v", err)
	}
	if removed != 0 {
		t.Errorf("2nd removed = %d, want 0", removed)
	}
}

func TestFindUnknownIDErrors(t *testing.T) {
	st := New(t.TempDir())
	if _, err := st.Find("nope"); err == nil {
		t.Error("expected error for unknown id")
	}
}

func TestPruneRemovesBackupDirs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.json")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(dir, "state")
	st := New(stateDir)
	for i := 0; i < MaxOperations+3; i++ {
		if _, err := st.Record("op", []string{path}); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(stateDir, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != MaxOperations {
		t.Errorf("backup dirs = %d, want %d", len(entries), MaxOperations)
	}
}

func TestRestoreDetectsTamperedBackup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.json")
	if err := os.WriteFile(path, []byte("orig"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := New(filepath.Join(dir, "state"))
	op, err := st.Record("op", []string{path})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(op.Files[0].BackupPath, []byte("tampered"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Restore(op); err == nil {
		t.Error("expected hash mismatch error")
	}
	// 復元に失敗しても、redo 用の記録はマニフェストから到達可能であること。
	ops, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) == 0 || ops[0].Command != "undo "+op.ID {
		t.Errorf("redo record not indexed after failed restore: %+v", ops)
	}
}

func TestRecordMarkerIsNotUndoTarget(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a.json")
	if err := os.WriteFile(path, []byte("orig"), 0o644); err != nil {
		t.Fatal(err)
	}
	st := New(filepath.Join(dir, "state"))
	if _, err := st.Record("push opencode", []string{path}); err != nil {
		t.Fatal(err)
	}
	marker, err := st.RecordMarker("push opencode")
	if err != nil {
		t.Fatal(err)
	}
	if marker.NoBackup != true || len(marker.Files) != 0 {
		t.Errorf("marker = %+v", marker)
	}

	found, err := st.Find("")
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if found.NoBackup {
		t.Error("bare undo must skip no-backup markers")
	}
	if found.Command != "push opencode" || len(found.Files) != 1 {
		t.Errorf("unexpected undo target: %+v", found)
	}

	ops, err := st.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 2 || !ops[0].NoBackup {
		t.Errorf("list should show marker first: %+v", ops)
	}
}
