package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
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

// InitializeCountersFromDatabase initializes the counters from existing database records
// This prevents duplicate key violations when the service restarts
func (g *IDGenerator) InitializeCountersFromDatabase(tableIdentifier string, existingIDs []string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if len(existingIDs) == 0 {
		return
	}

	// Find the highest numeric part from existing IDs
	maxCounter := int64(0)
	prefix := tableIdentifier[:4] // Ensure we only look at the first 4 characters

	for _, id := range existingIDs {
		if len(id) >= 4 && id[:4] == prefix {
			// Extract numeric part after the prefix
			numericPart := id[4:]
			if counter, err := strconv.ParseInt(numericPart, 10, 64); err == nil {
				if counter > maxCounter {
					maxCounter = counter
				}
			}
		}
	}

	// Set the counter to the highest value found + 1
	if maxCounter > 0 {
		g.counters[tableIdentifier] = maxCounter
	}
}

// InitializeCountersFromDatabaseWithSize initializes counters considering the table size
func (g *IDGenerator) InitializeCountersFromDatabaseWithSize(tableIdentifier string, existingIDs []string, size TableSize) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if len(existingIDs) == 0 {
		return
	}

	// Find the highest numeric part from existing IDs
	maxCounter := int64(0)

	// Safely get prefix - handle any length table identifier
	prefix := tableIdentifier
	if len(tableIdentifier) >= 4 {
		prefix = tableIdentifier[:4]
	}

	// Create regex pattern based on table size
	var pattern string
	switch size {
	case Tiny:
		pattern = fmt.Sprintf("^%s(\\d{4})$", prefix)
	case Small:
		pattern = fmt.Sprintf("^%s(\\d{6})$", prefix)
	case Medium:
		pattern = fmt.Sprintf("^%s(\\d{8})$", prefix)
	case Large:
		pattern = fmt.Sprintf("^%s(\\d{10})$", prefix)
	case XLarge:
		pattern = fmt.Sprintf("^%s(\\d{12})$", prefix)
	default:
		pattern = fmt.Sprintf("^%s(\\d{8})$", prefix) // Default to medium
	}

	re := regexp.MustCompile(pattern)

	for _, id := range existingIDs {
		matches := re.FindStringSubmatch(id)
		if len(matches) == 2 {
			if counter, err := strconv.ParseInt(matches[1], 10, 64); err == nil {
				if counter > maxCounter {
					maxCounter = counter
				}
			}
		}
	}

	// Set the counter to the highest found value
	if maxCounter > 0 {
		g.counters[tableIdentifier] = maxCounter
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

// InitializeGlobalCountersFromDatabase initializes the global generator's counters from database
func InitializeGlobalCountersFromDatabase(tableIdentifier string, existingIDs []string, size TableSize) {
	globalGenerator.InitializeCountersFromDatabaseWithSize(tableIdentifier, existingIDs, size)
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

	result := fmt.Sprintf(format, counter)
	return result
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
