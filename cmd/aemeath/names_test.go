package main

import "testing"

func TestParseName(t *testing.T) {
	tests := []struct {
		in                         string
		pascal, camel, file, route string
	}{
		{"Product", "Product", "product", "product", "products"},
		{"product", "Product", "product", "product", "products"},
		{"product-item", "ProductItem", "productItem", "productitem", "product-items"},
		{"product_item", "ProductItem", "productItem", "productitem", "product-items"},
		{"ProductItem", "ProductItem", "productItem", "productitem", "product-items"},
		{"APIKey", "ApiKey", "apiKey", "apikey", "api-keys"},
		{"category", "Category", "category", "category", "categories"},
		{"day", "Day", "day", "day", "days"},
		{"box", "Box", "box", "box", "boxes"},
		{"address", "Address", "address", "address", "addresses"},
		{"match", "Match", "match", "match", "matches"},
		{"user2", "User2", "user2", "user2", "user2s"},
	}
	for _, tc := range tests {
		got, err := parseName(tc.in)
		if err != nil {
			t.Errorf("parseName(%q): unexpected error: %v", tc.in, err)
			continue
		}
		want := Names{tc.pascal, tc.camel, tc.file, tc.route}
		if got != want {
			t.Errorf("parseName(%q) = %+v, want %+v", tc.in, got, want)
		}
	}
}

func TestParseNameRejectsUnsafeInput(t *testing.T) {
	for _, in := range []string{
		"", "   ", "../../etc/passwd", "a/b", `a\b`, "1product", "-product", "_x",
		"prod uct!", "product.go", "é", "名前", "a;b", "x\ny",
	} {
		if got, err := parseName(in); err == nil {
			t.Errorf("parseName(%q) = %+v, want an error", in, got)
		}
	}
}

func TestParseGoVersion(t *testing.T) {
	for in, want := range map[string]string{
		"go1.27.3": "1.27", "go1.22": "1.22", "go1.27rc1": "1.27", "devel go1.28-abc": "", "": "",
	} {
		if got := parseGoVersion(in); got != want {
			t.Errorf("parseGoVersion(%q) = %q, want %q", in, got, want)
		}
	}
}
