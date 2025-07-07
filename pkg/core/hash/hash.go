package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// TableSize represents the size category of a table
type TableSize string

const (
	Tiny   TableSize = "tiny"   // 4 digits
	Small  TableSize = "small"  // 6 digits
	Medium TableSize = "medium" // 8 digits
	Large  TableSize = "large"  // 10 digits
	XLarge TableSize = "xlarge" // 12 digits
)

// IDGenerator manages ID generation for different tables.
// It maintains a thread-safe map of counters for each table to generate unique incremental IDs.
// The counters map stores the last used ID value for each table identifier.
// The mutex (mu) ensures thread-safe access to the counters when multiple goroutines
// generate IDs concurrently.
type IDGenerator struct {
	counters map[string]int64 // Maps table identifiers to their current counter value
	mu       sync.RWMutex     // Protects concurrent access to the counters map
}

var (
	globalGenerator = NewIDGenerator()
)

// NewIDGenerator creates a new ID generator
func NewIDGenerator() *IDGenerator {
	return &IDGenerator{
		counters: make(map[string]int64),
	}
}

// GenerateRandomID generates a unique ID for a model based on table identifier and size
func GenerateRandomID(tableIdentifier string, size TableSize) (string, error) {
	return globalGenerator.GenerateID(tableIdentifier, size, "incremental")
}

// GenerateRandomIDWithPattern generates a unique ID with a specific pattern
func GenerateRandomIDWithPattern(tableIdentifier string, size TableSize, pattern string) (string, error) {
	return globalGenerator.GenerateID(tableIdentifier, size, pattern)
}

// GenerateID generates a unique ID based on table identifier, size, and pattern
func (g *IDGenerator) GenerateID(tableIdentifier string, size TableSize, pattern string) (string, error) {
	if len(tableIdentifier) != 4 {
		return "", fmt.Errorf("table identifier must be exactly 4 characters")
	}

	// Convert to uppercase
	tableID := fmt.Sprintf("%4s", tableIdentifier)
	for i := 0; i < 4; i++ {
		if tableID[i] == ' ' {
			tableID = tableID[:i] + "X" + tableID[i+1:]
		}
	}

	var numericPart string
	switch pattern {
	case "incremental":
		numericPart = g.generateIncrementalID(tableIdentifier, size)
	case "timestamp":
		numericPart = g.generateTimestampID(size)
	case "hash":
		numericPart = g.generateHashID(tableIdentifier, size)
	default:
		return "", fmt.Errorf("unsupported pattern: %s", pattern)
	}

	return tableID + numericPart, nil
}

// generateIncrementalID generates an incremental numeric ID
func (g *IDGenerator) generateIncrementalID(tableIdentifier string, size TableSize) string {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.counters[tableIdentifier]++
	counter := g.counters[tableIdentifier]

	var format string
	switch size {
	case Tiny:
		format = "%04d"
	case Small:
		format = "%06d"
	case Medium:
		format = "%08d"
	case Large:
		format = "%010d"
	case XLarge:
		format = "%012d"
	default:
		format = "%08d" // Default to medium
	}

	return fmt.Sprintf(format, counter)
}

// generateTimestampID generates a timestamp-based numeric ID
func (g *IDGenerator) generateTimestampID(size TableSize) string {
	g.mu.Lock()
	defer g.mu.Unlock()

	now := time.Now()
	timestamp := now.UnixNano()

	// Add counter to ensure uniqueness even for timestamps in same nanosecond
	g.counters["timestamp"]++
	counter := g.counters["timestamp"]

	// Combine timestamp and counter
	combined := timestamp*1000 + counter

	var format string
	var modulus int64
	switch size {
	case Tiny:
		format = "%04d"
		modulus = 10000
	case Small:
		format = "%06d"
		modulus = 1000000
	case Medium:
		format = "%08d"
		modulus = 100000000
	case Large:
		format = "%010d"
		modulus = 10000000000
	case XLarge:
		format = "%012d"
		modulus = 1000000000000
	default:
		format = "%08d"
		modulus = 100000000
	}

	// Get last N digits based on size
	numericPart := combined % modulus
	if numericPart < 0 {
		numericPart += modulus
	}

	return fmt.Sprintf(format, numericPart)
}

// generateHashID generates a hash-based numeric ID.
// It creates a SHA-256 hash of the table identifier and current timestamp,
// then converts the beginning of the hash to numeric digits.
// The resulting numeric string is padded or truncated to match the specified size.
func (g *IDGenerator) generateHashID(tableIdentifier string, size TableSize) string {
	data := fmt.Sprintf("%s:%d:%d", tableIdentifier, time.Now().UnixNano(), rand.Int63())
	hash := sha256.Sum256([]byte(data))
	hashHex := hex.EncodeToString(hash[:])

	var length int
	switch size {
	case Tiny:
		length = 4
	case Small:
		length = 6
	case Medium:
		length = 8
	case Large:
		length = 10
	case XLarge:
		length = 12
	default:
		length = 8
	}

	// Convert first 'length' characters of hash to numeric
	numericPart := ""
	for i := 0; i < length && i < len(hashHex); i++ {
		// Convert hex character to numeric (0-9)
		val := int(hashHex[i] - '0')
		if val < 0 || val > 9 {
			val = int(hashHex[i]-'a'+10) % 10
		}
		numericPart += fmt.Sprintf("%d", val)
	}

	// Pad with zeros if needed
	for len(numericPart) < length {
		numericPart = "0" + numericPart
	}

	return numericPart[:length]
}

// GetModelTypeFromHash extracts the model type from a hash ID.
// The model type is represented by the first 4 characters of the hash ID,
// which corresponds to the table identifier used during ID generation.
// If the hash ID is shorter than 4 characters, returns the entire hash ID.
func GetModelTypeFromHash(hashID string) string {
	if len(hashID) >= 4 {
		return hashID[:4]
	}
	return hashID
}
