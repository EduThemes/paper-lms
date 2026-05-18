package settings

import "testing"

// TestCatalog_MaxUploadSizeDefault locks the catalog default for
// quotas.max_upload_size_mb at 5120 MB (5 GB). Owner directive
// 2026-05-17: the framework BodyLimit is 5 GB and teachers should not
// be surprised by per-request rejections — the catalog default must
// match the framework safety net so the only way uploads get blocked
// is when an admin DELIBERATELY lowered the cap.
//
// If you're tempted to lower this default, raise the question in a
// product-level discussion first: the per-tenant override in the
// catalog already exists for accounts that need a tighter cap. Don't
// solve "we want a smaller default for x" by mutating the default for
// EVERYONE.
func TestCatalog_MaxUploadSizeDefault(t *testing.T) {
	def, ok := Find("quotas.max_upload_size_mb")
	if !ok {
		t.Fatal("catalog: quotas.max_upload_size_mb is missing")
	}
	if def.Default != "5120" {
		t.Fatalf("catalog: quotas.max_upload_size_mb default = %q, want %q (5 GB framework safety net)", def.Default, "5120")
	}
	if def.ValueType != TypeInt {
		t.Fatalf("catalog: quotas.max_upload_size_mb ValueType = %q, want %q", def.ValueType, TypeInt)
	}
	if !def.AllowsScope(ScopeInstance) || !def.AllowsScope(ScopeAccount) {
		t.Fatalf("catalog: quotas.max_upload_size_mb must allow both instance and account scope, got %v", def.Scopes)
	}
	if def.EnvFallback != "MAX_UPLOAD_SIZE_MB" {
		t.Fatalf("catalog: quotas.max_upload_size_mb EnvFallback = %q, want MAX_UPLOAD_SIZE_MB", def.EnvFallback)
	}
}

// TestCatalog_StorageBackendDropped locks the Wave 4 decision that
// `storage.backend` is NOT in the runtime catalog. It's a boot-time
// setting (the storage.Backend interface is constructed once at
// process start) and promoting it to the catalog created the false
// impression of hot-swap support. If you're adding it back, document
// the boot-time-vs-runtime contract first — see catalog.go's "File
// storage" section comment.
func TestCatalog_StorageBackendDropped(t *testing.T) {
	if _, ok := Find("storage.backend"); ok {
		t.Fatal("catalog: storage.backend must remain dropped (boot-only setting, read STORAGE_BACKEND env at boot — see cmd/server/main.go)")
	}
}
