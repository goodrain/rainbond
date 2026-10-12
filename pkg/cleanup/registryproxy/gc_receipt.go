//go:build linux || darwin

package registryproxy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"strings"

	coordination "github.com/goodrain/rainbond/pkg/cleanup"
	"golang.org/x/sys/unix"
)

// GCReceipt is executor evidence on the verified data volume, not an execution
// permission. A trusted launcher must also verify the executor's provenance.
type GCReceipt struct {
	Protocol    int                 `json:"protocol"`
	RequestHash string              `json:"request_hash"`
	Outcome     string              `json:"outcome"`
	Before      StorageMeasurement  `json:"before"`
	After       *StorageMeasurement `json:"after,omitempty"`
}

type gcReceiptJournal struct {
	directory int
	name      string
	record    GCReceipt
}

func gcReceiptKey(binding coordination.StorageRegistration, request coordination.CoordinationRequest) (string, string, error) {
	fingerprint, err := binding.Fingerprint()
	if err != nil || request.StorageID != binding.StorageID || request.Generation != binding.Generation || request.Kind != "gc" || request.Scope != "*" || request.Target != "" {
		return "", "", ErrStorageIdentity
	}
	for _, value := range []struct {
		text  string
		limit int
	}{{request.OperationID, 64}, {request.Owner, 128}, {request.Fingerprint, 128}} {
		if value.text == "" || len(value.text) > value.limit || strings.ContainsAny(value.text, "\x00\r\n") {
			return "", "", ErrStorageIdentity
		}
	}
	raw, _ := json.Marshal(request)
	identity := sha256.Sum256([]byte(request.OperationID))
	requestHash := sha256.Sum256(append([]byte(fingerprint+"\n"), raw...))
	return "gc-" + hex.EncodeToString(identity[:]), hex.EncodeToString(requestHash[:]), nil
}

func publishGCReceipt(directory int, name string, value GCReceipt) error {
	raw, err := json.Marshal(value)
	if err != nil || len(raw) > 16384 {
		return ErrStorageIdentity
	}
	nonce, err := coordination.NewActivationRevision()
	if err != nil {
		return err
	}
	temporary := "gc-" + nonce + ".tmp"
	fd, err := unix.Openat(directory, temporary, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return ErrStorageIdentity
	}
	defer unix.Unlinkat(directory, temporary, 0)
	file := os.NewFile(uintptr(fd), "gc-receipt")
	defer file.Close()
	if _, err := file.Write(raw); err != nil {
		return ErrStorageIdentity
	}
	if file.Chmod(0444) != nil || file.Sync() != nil {
		return ErrStorageIdentity
	}
	if err := unix.Linkat(directory, temporary, directory, name, 0); err != nil {
		return coordination.ErrCoordinationUncertain
	}
	if unix.Fsync(directory) != nil {
		return ErrStorageIdentity
	}
	return nil
}

func reserveGCReceipt(rootFD int, binding coordination.StorageRegistration, request coordination.CoordinationRequest, before StorageMeasurement) (*gcReceiptJournal, error) {
	name, hash, err := gcReceiptKey(binding, request)
	if err != nil {
		return nil, err
	}
	fingerprint, _ := binding.Fingerprint()
	if !coordination.IsValidStorageMeasurement(before) || before.BindingFingerprint != fingerprint || before.StorageID != binding.StorageID || before.Generation != binding.Generation {
		return nil, ErrStorageIdentity
	}
	directory, err := unix.Openat(rootFD, identityDirectory, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, ErrStorageIdentity
	}
	if readIdentity(directory, binding) != nil {
		unix.Close(directory)
		return nil, ErrStorageIdentity
	}
	journal := &gcReceiptJournal{directory: directory, name: name, record: GCReceipt{Protocol: 1, RequestHash: hash, Outcome: "admitted", Before: before}}
	if err := publishGCReceipt(directory, name+".begin", journal.record); err != nil {
		journal.Close()
		return nil, err
	}
	return journal, nil
}
func (j *gcReceiptJournal) Close() { unix.Close(j.directory) }
func (j *gcReceiptJournal) Complete(outcome string, after *StorageMeasurement) error {
	if outcome != "succeeded" && outcome != "failed" && outcome != "unknown" {
		return ErrStorageIdentity
	}
	result := j.record
	result.Outcome = outcome
	result.After = after
	return publishGCReceipt(j.directory, j.name+".result", result)
}

func readGCReceiptFile(directory int, name string) (GCReceipt, error) {
	var result GCReceipt
	fd, err := unix.Openat(directory, name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return result, coordination.ErrCoordinationUncertain
	}
	file := os.NewFile(uintptr(fd), "gc-receipt")
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 16384 {
		return result, ErrStorageIdentity
	}
	raw, err := io.ReadAll(io.LimitReader(file, 16385))
	if err != nil || len(raw) > 16384 {
		return result, ErrStorageIdentity
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&result) != nil || decoder.Decode(&struct{}{}) != io.EOF || result.Protocol != 1 {
		return GCReceipt{}, ErrStorageIdentity
	}
	return result, nil
}

// ReadGCReceipt only reads evidence for the original immutable operation.
func ReadGCReceipt(root string, binding coordination.StorageRegistration, request coordination.CoordinationRequest) (GCReceipt, error) {
	denied := GCReceipt{}
	name, hash, err := gcReceiptKey(binding, request)
	if err != nil {
		return denied, err
	}
	fd, err := openIdentityRoot(root)
	if err != nil {
		return denied, err
	}
	defer unix.Close(fd)
	directory, err := unix.Openat(fd, identityDirectory, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return denied, ErrStorageIdentity
	}
	defer unix.Close(directory)
	if readIdentity(directory, binding) != nil {
		return denied, ErrStorageIdentity
	}
	begin, err := readGCReceiptFile(directory, name+".begin")
	if err != nil {
		return denied, err
	}
	result, err := readGCReceiptFile(directory, name+".result")
	if err != nil {
		return denied, err
	}
	fingerprint, _ := binding.Fingerprint()
	if begin.RequestHash != hash || result.RequestHash != hash || begin.Outcome != "admitted" || !reflect.DeepEqual(begin.Before, result.Before) || !coordination.IsValidStorageMeasurement(result.Before) || result.Before.BindingFingerprint != fingerprint || result.Before.StorageID != binding.StorageID || result.Before.Generation != binding.Generation {
		return denied, ErrStorageIdentity
	}
	if result.Outcome != "succeeded" && result.Outcome != "failed" && result.Outcome != "unknown" {
		return denied, ErrStorageIdentity
	}
	if result.After != nil && (!coordination.IsValidStorageMeasurement(*result.After) || result.After.StorageID != binding.StorageID || result.After.Generation != binding.Generation || result.After.BindingFingerprint != fingerprint || result.After.FilesystemID != result.Before.FilesystemID || result.After.ObservedAt.Before(result.Before.ObservedAt)) {
		return denied, ErrStorageIdentity
	}
	return result, nil
}
