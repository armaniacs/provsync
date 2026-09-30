package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"llm-sync/internal/syncer"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "llm-sync:", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("llm-sync", flag.ContinueOnError)
	fs.SetOutput(out)

	source := fs.String("source", filepath.Join(home, ".config", "kilo", "kilo.jsonc"), "source kilo config (JSONC)")
	target := fs.String("target", filepath.Join(home, ".config", "opencode", "opencode.json"), "target opencode config (JSON)")
	write := fs.Bool("write", false, "write changes to target (default: preview only)")
	backup := fs.Bool("backup", true, "create .bak before writing")
	providers := fs.String("providers", "vs_inoue,sakura,vs", "comma-separated provider keys to port")
	if err := fs.Parse(args); err != nil {
		return err
	}

	keys := splitKeys(*providers)
	if len(keys) == 0 {
		return fmt.Errorf("no providers specified")
	}

	srcBytes, err := os.ReadFile(*source)
	if err != nil {
		return fmt.Errorf("read source: %w", err)
	}
	var src map[string]any
	if err := json.Unmarshal(syncer.StripJSONC(srcBytes), &src); err != nil {
		return fmt.Errorf("parse source: %w", err)
	}

	tgtBytes, err := os.ReadFile(*target)
	if err != nil {
		return fmt.Errorf("read target: %w", err)
	}
	var tgt map[string]any
	if err := json.Unmarshal(tgtBytes, &tgt); err != nil {
		return fmt.Errorf("parse target: %w", err)
	}

	merged, err := syncer.Merge(src, tgt, keys)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	for _, k := range keys {
		fmt.Fprintf(out, "%s: %s\n", k, summarize(merged, k))
	}

	if !*write {
		fmt.Fprintln(out, "(preview only; use --write to apply)")
		return nil
	}

	if *backup {
		if err := copyFile(*target, *target+".bak"); err != nil {
			return fmt.Errorf("backup: %w", err)
		}
	}
	if err := atomicWrite(*target, data); err != nil {
		return fmt.Errorf("write target: %w", err)
	}
	fmt.Fprintf(out, "wrote %s\n", *target)
	return nil
}

func summarize(cfg map[string]any, key string) string {
	providers, _ := cfg["provider"].(map[string]any)
	entry, _ := providers[key].(map[string]any)
	models, _ := entry["models"].(map[string]any)
	return fmt.Sprintf("置換 (models %d件)", len(models))
}

func splitKeys(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, info.Mode().Perm())
}

func atomicWrite(path string, data []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".llm-sync-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
