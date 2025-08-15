package crypt

import (
	"fmt"
	"testing"
)

func TestVerifyChecksum(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected bool
	}{
		{
			name:     "Valid checksum",
			data:     []byte{0x01, 0x02, 0x03, 0x04, 0x06, 0x00, 0x00, 0x00}, // data + checksum
			expected: true,
		},
		{
			name:     "Invalid checksum",
			data:     []byte{0x01, 0x02, 0x03, 0x04, 0xFF, 0xFF, 0xFF, 0xFF}, // data + wrong checksum
			expected: false,
		},
		{
			name:     "Too short data",
			data:     []byte{0x01, 0x02},
			expected: false,
		},
		{
			name:     "Not multiple of 4",
			data:     []byte{0x01, 0x02, 0x03, 0x04, 0x05},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// For valid checksum test, calculate proper checksum
			if tt.expected && len(tt.data) >= 8 {
				testData := make([]byte, len(tt.data))
				copy(testData, tt.data)
				// Calculate proper checksum for first test
				AppendChecksum(testData)
				result := VerifyChecksum(testData)
				if result != tt.expected {
					t.Errorf("VerifyChecksum() = %v, want %v", result, tt.expected)
				}
			} else {
				result := VerifyChecksum(tt.data)
				if result != tt.expected {
					t.Errorf("VerifyChecksum() = %v, want %v", result, tt.expected)
				}
			}
		})
	}
}

func TestAppendChecksum(t *testing.T) {
	// Test data: 4 bytes data + 4 bytes space for checksum
	data := []byte{0x01, 0x02, 0x03, 0x04, 0x00, 0x00, 0x00, 0x00}

	// Append checksum
	AppendChecksum(data)

	// Verify the checksum we just appended
	if !VerifyChecksum(data) {
		t.Error("AppendChecksum() failed - checksum verification failed")
	}

	// Check that data portion wasn't modified
	if data[0] != 0x01 || data[1] != 0x02 || data[2] != 0x03 || data[3] != 0x04 {
		t.Error("AppendChecksum() modified the original data")
	}
}

func TestVerifyChecksumClient(t *testing.T) {
	// Test client packet structure: [data][checksum 4 bytes][padding 12 bytes]
	// Minimum size is 16 bytes (at least checksum + padding)

	tests := []struct {
		name     string
		dataSize int
		expected bool
	}{
		{
			name:     "Valid client packet 16 bytes",
			dataSize: 16, // 0 data + 4 checksum + 12 padding
			expected: true,
		},
		{
			name:     "Valid client packet 32 bytes",
			dataSize: 32, // 16 data + 4 checksum + 12 padding
			expected: true,
		},
		{
			name:     "Too short for client packet",
			dataSize: 8,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.dataSize < 16 {
				// Test invalid size
				data := make([]byte, tt.dataSize)
				result := VerifyChecksumClient(data)
				if result != tt.expected {
					t.Errorf("VerifyChecksumClient() = %v, want %v", result, tt.expected)
				}
			} else {
				// Create client packet: data + checksum + padding
				data := make([]byte, tt.dataSize)
				dataLen := tt.dataSize - 16 // Actual data length (excluding checksum + padding)

				// Fill data portion
				for i := 0; i < dataLen; i++ {
					data[i] = byte(i % 256)
				}

				// Calculate checksum for data portion
				var chksum uint32 = 0
				// Only process complete 4-byte blocks
				for i := 0; i+3 < dataLen; i += 4 {
					check := uint32(data[i]) & 0xff
					check |= (uint32(data[i+1]) << 8) & 0xff00
					check |= (uint32(data[i+2]) << 16) & 0xff0000
					check |= (uint32(data[i+3]) << 24) & 0xff000000
					chksum ^= check
				}

				// Place checksum after data
				checksumPos := dataLen
				data[checksumPos] = byte(chksum & 0xff)
				data[checksumPos+1] = byte((chksum >> 8) & 0xff)
				data[checksumPos+2] = byte((chksum >> 16) & 0xff)
				data[checksumPos+3] = byte((chksum >> 24) & 0xff)

				// Test verification
				result := VerifyChecksumClient(data)
				if result != tt.expected {
					t.Errorf("VerifyChecksumClient() = %v, want %v", result, tt.expected)
				}
			}
		})
	}
}

func TestChecksumRoundTrip(t *testing.T) {
	// Test various data sizes
	sizes := []int{8, 16, 32, 64}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			// Create test data (size-4 bytes data + 4 bytes for checksum)
			data := make([]byte, size)
			for i := 0; i < size-4; i++ {
				data[i] = byte(i % 256)
			}

			// Append checksum
			AppendChecksum(data)

			// Verify checksum
			if !VerifyChecksum(data) {
				t.Errorf("Checksum round-trip failed for size %d", size)
			}
		})
	}
}
