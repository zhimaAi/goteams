// Copyright © 2026 Wuhan Sesame Small Customer Service Network Technology Co., Ltd.

package common

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Device identity salts. They are GoTeams-specific constants: never reuse ChatWiki prefixes,
// and never fall back to a fixed MD5 constant shared by all machines.
const (
	deviceSaltMain    = `goteams-device-v1`
	deviceSaltDev     = `goteams-device-dev-v1`
	machineIDSalt     = `goteams-machine-id-v1`
	productUUIDSalt   = `goteams-product-uuid-v1`
	boardSerialSalt   = `goteams-board-serial-v1`
	deviceCodeBodyLen = 16 // 15 hex payload chars + 1 check char
)

// deviceCheckAlphabet is the 31-char unambiguous alphabet used for the device code check char.
const deviceCheckAlphabet = `0123456789ABCDEFGHJKMNPQRSTVWXYZ`

// Linux proc/sys sources of stable host identity.
const (
	pathMachineID   = `/etc/machine-id`
	pathProductUUID = `/sys/class/dmi/id/product_uuid`
	pathBoardSerial = `/sys/class/dmi/id/board_serial`
)

// Device identity errors (mapped to config_error states by callers).
var (
	ErrDeviceIdentityMissing = errors.New(`device stable identity missing`)
	ErrDeviceInstanceMissing = errors.New(`device instance id missing`)
	ErrDeviceCodeFormat      = errors.New(`device code format invalid`)
)

// DeviceInfo holds the raw stable identifiers that feed the device code and fingerprints.
// Raw values never leave the process: only salted SHA-256 digests are displayed or submitted.
type DeviceInfo struct {
	MachineID   string
	ProductUUID string
	BoardSerial string
	InstanceID  string
	// DevMode marks the goteams-device-dev-v1 algorithm used by every IsDev run.
	DevMode bool
}

// DeviceFingerprints holds the salted SHA-256 digests submitted with online trial requests.
// Only non-empty components are hashed; the combined digest joins available components in a
// fixed order so a host that later exposes more identifiers still keeps its old component rows.
type DeviceFingerprints struct {
	MachineIDHash   string `json:"machine_id_hash,omitempty"`
	ProductUUIDHash string `json:"product_uuid_hash,omitempty"`
	BoardSerialHash string `json:"board_serial_hash,omitempty"`
	Combined        string `json:"hardware_fingerprint_hash,omitempty"`
}

// LoadDeviceInfo reads stable host identity plus the persistent instance id.
// Production requires the instance id file (created once by the install tool, never rebuilt
// by the app) and at least one stable host identifier; dev runs fall back to a dev algorithm.
func LoadDeviceInfo(instanceIDFile string, devMode bool) (*DeviceInfo, error) {
	info := &DeviceInfo{DevMode: devMode}
	if devMode {
		hostname, err := os.Hostname()
		if err != nil || strings.TrimSpace(hostname) == `` {
			return nil, fmt.Errorf(`read hostname: %w`, err)
		}
		info.MachineID = strings.ToLower(strings.TrimSpace(hostname))
		instanceID, err := loadOrCreateInstanceID(instanceIDFile)
		if err != nil {
			return nil, err
		}
		info.InstanceID = instanceID
		return info, nil
	}

	info.MachineID = readStableFile(pathMachineID, strings.ToLower)
	info.ProductUUID = readStableFile(pathProductUUID, strings.ToUpper)
	info.BoardSerial = readStableFile(pathBoardSerial, strings.ToUpper)
	instanceBytes, err := os.ReadFile(instanceIDFile)
	if err != nil {
		return nil, fmt.Errorf(`%w: %s`, ErrDeviceInstanceMissing, instanceIDFile)
	}
	info.InstanceID = strings.ToLower(strings.TrimSpace(string(instanceBytes)))
	if info.InstanceID == `` {
		return nil, ErrDeviceInstanceMissing
	}
	if info.MachineID == `` && info.ProductUUID == `` && info.BoardSerial == `` {
		return nil, ErrDeviceIdentityMissing
	}
	return info, nil
}

// loadOrCreateInstanceID reads the instance id file, creating it once on dev machines only.
func loadOrCreateInstanceID(instanceIDFile string) (string, error) {
	if data, err := os.ReadFile(instanceIDFile); err == nil {
		instanceID := strings.TrimSpace(string(data))
		if instanceID != `` {
			return instanceID, nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(instanceIDFile), 0o755); err != nil {
		return ``, fmt.Errorf(`create instance id dir: %w`, err)
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return ``, fmt.Errorf(`generate instance id: %w`, err)
	}
	instanceID := hex.EncodeToString(buf)
	if err := os.WriteFile(instanceIDFile, []byte(instanceID+`\n`), 0o600); err != nil {
		return ``, fmt.Errorf(`write instance id: %w`, err)
	}
	return instanceID, nil
}

// readStableFile reads a host identity source, normalizes it and returns an empty string
// when the file is missing or empty (containers often lack DMI entries).
func readStableFile(path string, normalize func(string) string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ``
	}
	value := normalize(strings.TrimSpace(strings.Trim(string(data), "\x00")))
	if value == `` {
		return ``
	}
	return value
}

// ComputeDeviceFingerprints derives the salted digests used for trial identity dedup.
func ComputeDeviceFingerprints(info *DeviceInfo) DeviceFingerprints {
	var prints DeviceFingerprints
	components := make([]string, 0, 3)
	if info.MachineID != `` {
		prints.MachineIDHash = sha256Hex(machineIDSalt + `|` + info.MachineID)
		components = append(components, prints.MachineIDHash)
	}
	if info.ProductUUID != `` {
		prints.ProductUUIDHash = sha256Hex(productUUIDSalt + `|` + info.ProductUUID)
		components = append(components, prints.ProductUUIDHash)
	}
	if info.BoardSerial != `` {
		prints.BoardSerialHash = sha256Hex(boardSerialSalt + `|` + info.BoardSerial)
		components = append(components, prints.BoardSerialHash)
	}
	if len(components) > 0 {
		prints.Combined = sha256Hex(strings.Join(components, `|`))
	}
	return prints
}

// GenerateDeviceCode builds the display device code from the stable identifiers.
// Production digest: SHA-256("goteams-device-v1|" + machine_id + "|" + product_uuid + "|"
// + board_serial + "|" + instance_id). Missing components contribute an empty string,
// which stays deterministic per host.
func GenerateDeviceCode(info *DeviceInfo) (string, error) {
	if info.InstanceID == `` {
		return ``, ErrDeviceInstanceMissing
	}
	var digestSource string
	if info.DevMode {
		digestSource = deviceSaltDev + `|` + info.MachineID + `|` + info.InstanceID
	} else {
		if info.MachineID == `` && info.ProductUUID == `` && info.BoardSerial == `` {
			return ``, ErrDeviceIdentityMissing
		}
		digestSource = deviceSaltMain + `|` + info.MachineID + `|` + info.ProductUUID + `|` + info.BoardSerial + `|` + info.InstanceID
	}
	digest := sha256.Sum256([]byte(digestSource))
	return encodeDeviceCode(digest[:]), nil
}

// encodeDeviceCode renders the digest as GTDEV1-XXXX-XXXX-XXXX-XXXX with a trailing check char.
func encodeDeviceCode(digest []byte) string {
	payload := strings.ToUpper(hex.EncodeToString(digest))[:deviceCodeBodyLen-1]
	check := deviceCheckAlphabet[checksumByte(payload)]
	return formatDeviceCodeBody(payload + string(check))
}

// checksumByte sums the payload byte values modulo the alphabet size.
func checksumByte(payload string) int {
	sum := 0
	for i := 0; i < len(payload); i++ {
		sum += int(payload[i])
	}
	return sum % len(deviceCheckAlphabet)
}

// formatDeviceCodeBody inserts the GTDEV1 prefix and dashes into a 16-char body.
func formatDeviceCodeBody(body string) string {
	return defineDevicePrefix() + `-` + body[0:4] + `-` + body[4:8] + `-` + body[8:12] + `-` + body[12:16]
}

// defineDevicePrefix returns the GTDEV1 prefix constant.
func defineDevicePrefix() string {
	return `GTDEV1`
}

// NormalizeDeviceCode validates and canonicalizes a user/operator supplied device code:
// uppercase, strip whitespace and dashes, verify length, hex payload and check char.
// It returns the canonical dashed form stored by both the terminal and the operations backend.
func NormalizeDeviceCode(raw string) (string, error) {
	value := strings.ToUpper(strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\r', '\n', '-':
			return -1
		}
		return r
	}, strings.TrimSpace(raw)))
	if !strings.HasPrefix(value, `GTDEV1`) {
		return ``, ErrDeviceCodeFormat
	}
	body := strings.TrimPrefix(value, `GTDEV1`)
	if len(body) != deviceCodeBodyLen {
		return ``, ErrDeviceCodeFormat
	}
	payload, check := body[:deviceCodeBodyLen-1], body[deviceCodeBodyLen-1]
	if _, err := strconv.ParseInt(payload, 16, 64); err != nil {
		return ``, ErrDeviceCodeFormat
	}
	if string(deviceCheckAlphabet[checksumByte(payload)]) != string(check) {
		return ``, ErrDeviceCodeFormat
	}
	return formatDeviceCodeBody(body), nil
}

// sha256Hex returns the lowercase hex SHA-256 of the input.
func sha256Hex(input string) string {
	sum := sha256.Sum256([]byte(input))
	return hex.EncodeToString(sum[:])
}
