package hash

// Test data for TestGenerateRandomID
var GenerateRandomIDTests = []struct {
	name            string
	tableIdentifier string
	size            TableSize
	expectedPrefix  string
	expectedLength  int
	shouldError     bool
}{
	{
		name:            "Valid USERS table with Medium size",
		tableIdentifier: "USER",
		size:            Medium,
		expectedPrefix:  "USER",
		expectedLength:  12, // 4 chars + 8 digits
		shouldError:     false,
	},
	{
		name:            "Valid ORDR table with Small size",
		tableIdentifier: "ORDR",
		size:            Small,
		expectedPrefix:  "ORDR",
		expectedLength:  10, // 4 chars + 6 digits
		shouldError:     false,
	},
	{
		name:            "Valid PROD table with Large size",
		tableIdentifier: "PROD",
		size:            Large,
		expectedPrefix:  "PROD",
		expectedLength:  14, // 4 chars + 10 digits
		shouldError:     false,
	},
	{
		name:            "Valid CATE table with Tiny size",
		tableIdentifier: "CATE",
		size:            Tiny,
		expectedPrefix:  "CATE",
		expectedLength:  8, // 4 chars + 4 digits
		shouldError:     false,
	},
	{
		name:            "Valid INVT table with XLarge size",
		tableIdentifier: "INVT",
		size:            XLarge,
		expectedPrefix:  "INVT",
		expectedLength:  16, // 4 chars + 12 digits
		shouldError:     false,
	},
	{
		name:            "Invalid table identifier - too short",
		tableIdentifier: "ABC",
		size:            Medium,
		shouldError:     true,
	},
	{
		name:            "Invalid table identifier - too long",
		tableIdentifier: "ABCDE",
		size:            Medium,
		shouldError:     true,
	},
	{
		name:            "Table identifier with spaces",
		tableIdentifier: "US R",
		size:            Medium,
		expectedPrefix:  "USXR",
		expectedLength:  12,
		shouldError:     false,
	},
}

// Test data for TestGenerateRandomIDWithPattern
var GenerateRandomIDWithPatternTests = []struct {
	name            string
	tableIdentifier string
	size            TableSize
	pattern         string
	expectedPrefix  string
	expectedLength  int
	shouldError     bool
}{
	{
		name:            "Incremental pattern",
		tableIdentifier: "TEST",
		size:            Medium,
		pattern:         "incremental",
		expectedPrefix:  "TEST",
		expectedLength:  12,
		shouldError:     false,
	},
	{
		name:            "Timestamp pattern",
		tableIdentifier: "TEST",
		size:            Medium,
		pattern:         "timestamp",
		expectedPrefix:  "TEST",
		expectedLength:  12,
		shouldError:     false,
	},
	{
		name:            "Hash pattern",
		tableIdentifier: "TEST",
		size:            Medium,
		pattern:         "hash",
		expectedPrefix:  "TEST",
		expectedLength:  12,
		shouldError:     false,
	},
	{
		name:            "Invalid pattern",
		tableIdentifier: "TEST",
		size:            Medium,
		pattern:         "invalid",
		shouldError:     true,
	},
}

// Test data for TestGetModelTypeFromHash
var GetModelTypeFromHashTests = []struct {
	name     string
	hashID   string
	expected string
}{
	{
		name:     "Normal hash ID",
		hashID:   "USER12345678",
		expected: "USER",
	},
	{
		name:     "Short hash ID",
		hashID:   "ABC",
		expected: "ABC",
	},
	{
		name:     "Empty hash ID",
		hashID:   "",
		expected: "",
	},
	{
		name:     "Exact 4 character hash ID",
		hashID:   "TEST",
		expected: "TEST",
	},
}

// Test data for TestTableSizeFormats
var TableSizeFormatTests = []struct {
	size           TableSize
	expectedLength int
}{
	{Tiny, 8},    // 4 chars + 4 digits
	{Small, 10},  // 4 chars + 6 digits
	{Medium, 12}, // 4 chars + 8 digits
	{Large, 14},  // 4 chars + 10 digits
	{XLarge, 16}, // 4 chars + 12 digits
}

// Test data for TestSpaceReplacementInTableIdentifier
var SpaceReplacementTests = []struct {
	input    string
	expected string
}{
	{"US R", "USXR"},
	{"U R ", "UXRX"},
	{" US ", "XUSX"},
	{"   U", "XXXU"},
}

// Helper function to extract numeric part from ID
func extractNumericPart(id string) int {
	if len(id) <= 4 {
		return 0
	}

	numericStr := id[4:]
	var result int
	for _, char := range numericStr {
		if char >= '0' && char <= '9' {
			result = result*10 + int(char-'0')
		}
	}
	return result
}
