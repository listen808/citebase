package ai

import "testing"

// P0 增量缓存：指纹必须同时绑定内容与 embedding 身份。
// 内容没变 → 指纹不变（跳过重索引的前提）；
// 换 embedding 模型/供应商 → 指纹必变（触发全量重建，防止新旧维度混存）。
func TestDocSyncFingerprint(t *testing.T) {
	s := &VectorSyncer{embIdentity: "openai:text-embedding-3-small"}

	fp1 := s.docSyncFingerprint("hello world")
	fp2 := s.docSyncFingerprint("hello world")
	if fp1 != fp2 {
		t.Fatalf("same content and identity should produce same fingerprint: %s vs %s", fp1, fp2)
	}

	if fp := s.docSyncFingerprint("hello worlds"); fp == fp1 {
		t.Fatalf("different content should produce different fingerprint")
	}

	other := &VectorSyncer{embIdentity: "openai:text-embedding-3-large"}
	if fp := other.docSyncFingerprint("hello world"); fp == fp1 {
		t.Fatalf("different emb identity should produce different fingerprint")
	}

	empty := &VectorSyncer{}
	if fp := empty.docSyncFingerprint("hello world"); fp == fp1 {
		t.Fatalf("empty identity must not collide with populated identity")
	}
}
