// vm-restore-copy executes only a backend-generated fixed-mount plan.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/goodrain/rainbond/pkg/vmrestore/copyio"
)

func persist(path string, result copyio.Report) ([]byte, error) {
	data, err := json.Marshal(result)
	if err != nil || len(data) > copyio.MaxReportBytes {
		return nil, copyio.ErrInvalidPlan
	}
	temporary := path + ".tmp"
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	defer os.Remove(temporary)
	_, err = file.Write(data)
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if err := os.Rename(temporary, path); err != nil {
		return nil, err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return nil, err
	}
	return data, nil
}

type runtimePaths struct {
	plan        string
	volumes     string
	work        string
	termination string
}

// executeWithPaths receives only trusted runtime paths, never plan-provided paths.
func executeWithPaths(parent context.Context, paths runtimePaths, output io.Writer) int {
	input, err := os.Open(paths.plan)
	if err != nil {
		return 1
	}
	defer input.Close()
	body, err := io.ReadAll(io.LimitReader(input, copyio.MaxPlanBytes+1))
	if err != nil || len(body) > copyio.MaxPlanBytes {
		return 1
	}
	var plan copyio.Plan
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&plan) != nil || decoder.Decode(new(interface{})) != io.EOF || plan.Validate() != nil {
		return 1
	}
	ctx, cancel := context.WithTimeout(parent, time.Duration(plan.DeadlineSeconds)*time.Second)
	defer cancel()
	if err := os.MkdirAll(paths.work, 0700); err != nil {
		return 1
	}
	var last []byte
	save := func(report copyio.Report) error {
		data, err := persist(filepath.Join(paths.work, "report.json"), report)
		if err == nil {
			last = data
		}
		return err
	}
	result, runErr := copyio.Run(ctx, plan, paths.volumes, save)
	// Full bounded report is persisted first, then emitted as exactly one JSON line.
	// The Region reader saves this typed payload before evaluating any assertion.
	if len(last) == 0 {
		data, err := persist(filepath.Join(paths.work, "report.json"), result)
		if err == nil {
			last = data
		}
	}
	if len(last) == 0 {
		return 1
	}
	if n, err := output.Write(append(last, '\n')); err != nil || n != len(last)+1 {
		return 1
	}
	hash := sha256.Sum256(last)
	summary := copyio.Summary{Protocol: copyio.Protocol, OperationID: plan.OperationID, PlanHash: plan.PlanHash, OK: result.OK && runErr == nil, ReportSHA256: hex.EncodeToString(hash[:]), ReportBytes: len(last), DiskCount: len(result.Disks), ErrorCode: result.ErrorCode}
	payload, err := json.Marshal(summary)
	if err != nil || len(payload) > 3500 {
		return 1
	}
	if err := os.WriteFile(paths.termination, payload, 0600); err != nil {
		return 1
	}
	if runErr != nil || !result.OK {
		return 1
	}
	return 0
}
func execute() int {
	parent, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	return executeWithPaths(parent, runtimePaths{plan: "/plan/plan.json", volumes: "/volumes", work: "/work", termination: "/dev/termination-log"}, os.Stdout)
}
func main() { os.Exit(execute()) }
