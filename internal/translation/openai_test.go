package translation

import (
	"testing"
)

func TestChunkHTML(t *testing.T) {
	testCases := []struct {
		name      string
		text      string
		chunkSize int
		expected  []string
	}{
		{
			name:      "Empty string",
			text:      "",
			chunkSize: 100,
			expected:  []string{""},
		},
		{
			name:      "Text smaller than chunk size",
			text:      "Hello, world!",
			chunkSize: 100,
			expected:  []string{"Hello, world!"},
		},
		{
			name:      "Text exactly chunk size",
			text:      "This is a test string of exactly 50 characters.",
			chunkSize: 50,
			expected:  []string{"This is a test string of exactly 50 characters."},
		},
		{
			name:      "Text larger than chunk size, split by word",
			text:      "This is a very long single paragraph that will need to be split in the middle of the text because it exceeds the chunk size.",
			chunkSize: 50,
			expected: []string{
				"This is a very long single paragraph that will need",
				"to be split in the middle of the text because it",
				"exceeds the chunk size.",
			},
		},
		{
			name:      "HTML tags should not be split",
			text:      "<p>This is a paragraph with <b>bold</b> text.</p>",
			chunkSize: 30,
			expected: []string{
				"<p>This is a paragraph with",
				"<b>bold</b> text.</p>",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			chunks := chunkHTML(tc.text, tc.chunkSize)

			if len(chunks) != len(tc.expected) {
				t.Fatalf("Expected %d chunks, but got %d. Chunks: %v", len(tc.expected), len(chunks), chunks)
			}

			for i, chunk := range chunks {
				if chunk != tc.expected[i] {
					t.Errorf("Chunk %d does not match.\nExpected: %q\nGot:      %q", i, tc.expected[i], chunk)
				}
			}
		})
	}
}
