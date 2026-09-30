package report

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"fluxguard/protocol"
)

const maxSpool = 400

func flushSpool(ctx context.Context, cfg Config, client *http.Client) error {
	if cfg.SpoolFile == "" {
		return nil
	}
	items, err := readSpool(cfg.SpoolFile)
	if err != nil || len(items) == 0 {
		return err
	}
	var keep []protocol.Request
	var failed error
	for _, item := range items {
		err := postBatch(ctx, client, cfg.ServerURL+protocol.IngestPath, cfg.AgentToken, item)
		if err != nil {
			keep = append(keep, item)
			failed = errors.Join(failed, err)
		}
	}
	if err := writeSpool(cfg.SpoolFile, keep); err != nil {
		return errors.Join(failed, err)
	}
	return failed
}

func appendSpool(path string, payload protocol.Request) error {
	items, err := readSpool(path)
	if err != nil {
		return err
	}
	items = append(items, payload)
	if len(items) > maxSpool {
		items = items[len(items)-maxSpool:]
	}
	return writeSpool(path, items)
}

func readSpool(path string) ([]protocol.Request, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()
	var out []protocol.Request
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var item protocol.Request
		if err := json.Unmarshal(line, &item); err != nil {
			continue
		}
		out = append(out, item)
	}
	return out, scanner.Err()
}

func writeSpool(path string, items []protocol.Request) error {
	if len(items) == 0 {
		err := os.Remove(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf []byte
	for _, item := range items {
		line, err := json.Marshal(item)
		if err != nil {
			return err
		}
		buf = append(buf, line...)
		buf = append(buf, '\n')
	}
	return os.WriteFile(path, buf, 0o644)
}
