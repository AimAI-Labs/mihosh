package profile

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

var ErrStaleRaw = errors.New("subscription snapshot belongs to a different source")

const unknownRawOrigin = "unknown"

type rawSourceBinding struct {
	Origin string `json:"origin"`
	Digest string `json:"sha256"`
}

func rawOrigin(source SubSource) string {
	identity := fmt.Sprintf("%d:%q:%q", source.Kind, strings.TrimSpace(source.URL), strings.TrimSpace(source.Path))
	return fmt.Sprintf("%x", sha256.Sum256([]byte(identity)))
}

func rawBinding(origin string, raw []byte) rawSourceBinding {
	return rawSourceBinding{Origin: origin, Digest: fmt.Sprintf("%x", sha256.Sum256(raw))}
}

func readRawOrigin(path string, raw []byte) (string, error) {
	data, err := os.ReadFile(path + ".source.json")
	if os.IsNotExist(err) {
		return "", nil // Snapshots created by older versions have no origin record.
	}
	if err != nil {
		return "", err
	}
	var bindings []rawSourceBinding
	if err := json.Unmarshal(data, &bindings); err != nil {
		return unknownRawOrigin, nil
	}
	digest := rawBinding("", raw).Digest
	for _, binding := range bindings {
		if binding.Digest == digest && binding.Origin != "" {
			return binding.Origin, nil
		}
	}
	return unknownRawOrigin, nil
}

func writeRawBindings(path string, bindings ...rawSourceBinding) error {
	data, err := json.Marshal(bindings)
	if err != nil {
		return err
	}
	return atomicWrite(path+".source.json", data, 0600)
}

// BindLegacyRawSource records the old origin before changing subscription metadata.
func BindLegacyRawSource(p Profile) error {
	path, err := rawStoragePath(p.UID)
	if err != nil {
		return err
	}
	origin := rawOrigin(p.Source)
	bind := func(path string) error {
		if _, err := os.Stat(path + ".source.json"); !os.IsNotExist(err) {
			return err
		}
		raw, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		return writeRawBindings(path, rawBinding(origin, raw))
	}
	if err := bind(path); err != nil {
		return err
	}
	dir, err := snapshotDir(SnapshotRaw, p.UID)
	if err != nil {
		return err
	}
	snapshots, err := ListSnapshots(SnapshotRaw, p.UID)
	if err != nil {
		return err
	}
	for _, snapshot := range snapshots {
		if err := bind(filepath.Join(dir, snapshot.ID)); err != nil {
			return err
		}
	}
	return nil
}

// ReadRawForProfile accepts legacy data and verifies recorded origins before an apply.
func ReadRawForProfile(p Profile) (*yaml.Node, error) {
	raw, err := readVerifiedRaw(p)
	if err != nil {
		return nil, err
	}
	return decodeRaw(raw)
}

// VerifyRawSource checks the origin of the cached subscription bytes.
func VerifyRawSource(p Profile) error {
	_, err := readVerifiedRaw(p)
	return err
}

func readVerifiedRaw(p Profile) ([]byte, error) {
	path, err := rawStoragePath(p.UID)
	if err != nil {
		return nil, err
	}
	raw, err := readRawAt(path)
	if err != nil {
		return nil, err
	}
	if err := verifyRawOrigin(path, raw, p.Source); err != nil {
		return nil, err
	}
	return raw, nil
}

func verifyRawOrigin(path string, raw []byte, source SubSource) error {
	origin, err := readRawOrigin(path, raw)
	if err != nil {
		return err
	}
	if origin != "" && origin != rawOrigin(source) {
		return ErrStaleRaw
	}
	return nil
}

func WriteRawForSource(p Profile, data []byte) error {
	return writeRaw(p.UID, data, rawOrigin(p.Source))
}

// RestoreRawSnapshot publishes the requested raw data and returns an undo
// operation. Call both under the same configuration lock as subscription writes.
func RestoreRawSnapshot(p Profile, id string) (func() error, error) {
	data, err := ReadSnapshot(SnapshotRaw, p.UID, id)
	if err != nil {
		return nil, err
	}
	dir, err := snapshotDir(SnapshotRaw, p.UID)
	if err != nil {
		return nil, err
	}
	if err := verifyRawOrigin(filepath.Join(dir, id), data, p.Source); err != nil {
		return nil, err
	}
	path, err := rawStoragePath(p.UID)
	if err != nil {
		return nil, err
	}
	restore, err := backupRawFiles(path)
	if err != nil {
		return nil, err
	}
	if err := WriteRawForSource(p, data); err != nil {
		return nil, errors.Join(err, restore())
	}
	return func() error {
		current, err := readRawAt(path)
		if err != nil {
			return err
		}
		origin, err := readRawOrigin(path, current)
		if err != nil {
			return err
		}
		if !bytes.Equal(current, data) || origin != rawOrigin(p.Source) {
			return errors.New("subscription snapshot changed during rollback")
		}
		return restore()
	}, nil
}

func backupRawFiles(path string) (func() error, error) {
	files := [...]struct {
		path    string
		data    []byte
		existed bool
	}{{path: path}, {path: path + ".source.json"}}
	for i := range files {
		data, err := os.ReadFile(files[i].path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		files[i].data, files[i].existed = data, true
	}
	return func() error {
		var restored error
		for _, file := range files {
			if file.existed {
				restored = errors.Join(restored, atomicWrite(file.path, file.data, 0600))
			} else if err := os.Remove(file.path); err != nil && !os.IsNotExist(err) {
				restored = errors.Join(restored, err)
			}
		}
		return restored
	}, nil
}
