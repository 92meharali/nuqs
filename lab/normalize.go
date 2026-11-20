// Command normalize validates and fixes the two timestamps in a Nuqs JSON report.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

func main() {
	if err := normalize(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func normalize(input io.Reader, output io.Writer) error {
	data, err := io.ReadAll(input)
	if err != nil {
		return err
	}
	// Preserve all report fields, including fields added in future releases.
	var report map[string]json.RawMessage
	if err := json.Unmarshal(data, &report); err != nil {
		return err
	}
	var started, finished time.Time
	if err := json.Unmarshal(report["started"], &started); err != nil {
		return fmt.Errorf("started: %w", err)
	}
	if err := json.Unmarshal(report["finished"], &finished); err != nil {
		return fmt.Errorf("finished: %w", err)
	}
	if started.IsZero() || finished.Before(started) {
		return errors.New("invalid report time interval")
	}
	report["started"] = json.RawMessage(`"2000-01-01T00:00:00Z"`)
	report["finished"] = json.RawMessage(`"2000-01-01T00:00:01Z"`)
	enc := json.NewEncoder(output)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}
