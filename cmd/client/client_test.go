package main

import (
	"testing"
)

func TestParseItems(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"empty", "", 0},
		{"single", "item1", 1},
		{"two items", "item1,item2", 2},
		{"three items", "a,b,c", 3},
		{"with spaces", "item1, item2, item3", 3},
		{"empty between commas", "a,,b", 3},
		{"trailing comma", "a,b,", 2}, // split doesn't include trailing empty string
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseItems(tt.input)
			if len(result) != tt.expected {
				t.Errorf("Expected %d items, got %d", tt.expected, len(result))
			}
		})
	}
}

func TestParseItemsContent(t *testing.T) {
	input := "hello,world,test"
	result := parseItems(input)

	expected := [][]byte{[]byte("hello"), []byte("world"), []byte("test")}
	if len(result) != len(expected) {
		t.Fatalf("Expected %d items, got %d", len(expected), len(result))
	}

	for i, exp := range expected {
		if string(result[i]) != string(exp) {
			t.Errorf("Item %d: expected %s, got %s", i, string(exp), string(result[i]))
		}
	}
}

func TestSplit(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		sep      rune
		expected []string
	}{
		{"empty string", "", ',', []string{}},
		{"no separator", "hello", ',', []string{"hello"}},
		{"single separator", "a,b", ',', []string{"a", "b"}},
		{"multiple separators", "a,b,c", ',', []string{"a", "b", "c"}},
		{"trailing separator", "a,b,", ',', []string{"a", "b"}}, // split doesn't include trailing empty string
		{"leading separator", ",a,b", ',', []string{"", "a", "b"}},
		{"consecutive separators", "a,,b", ',', []string{"a", "", "b"}},
		{"different separator", "a:b:c", ':', []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := split(tt.input, tt.sep)
			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d parts, got %d: %v", len(tt.expected), len(result), result)
				return
			}
			for i, exp := range tt.expected {
				if result[i] != exp {
					t.Errorf("Part %d: expected %q, got %q", i, exp, result[i])
				}
			}
		})
	}
}

func TestSplitWithUnicode(t *testing.T) {
	input := "你好，世界"
	result := split(input, '，')

	if len(result) != 2 {
		t.Errorf("Expected 2 parts, got %d: %v", len(result), result)
	}

	if result[0] != "你好" {
		t.Errorf("Expected first part 你好，got %s", result[0])
	}

	if result[1] != "世界" {
		t.Errorf("Expected second part 世界，got %s", result[1])
	}
}

func TestSplitLongString(t *testing.T) {
	// Create a long string with many separators
	input := ""
	for i := 0; i < 100; i++ {
		if i > 0 {
			input += ","
		}
		input += "item"
	}

	result := split(input, ',')

	if len(result) != 100 {
		t.Errorf("Expected 100 parts, got %d", len(result))
	}

	for i, part := range result {
		if part != "item" {
			t.Errorf("Part %d: expected 'item', got %q", i, part)
		}
	}
}
