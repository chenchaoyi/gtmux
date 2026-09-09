package hq

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

// A charter edit that does not bump hqPlaybookVersion reaches nobody.
//
// `gtmux hq` re-seeds an existing home only when the SHIPPED version is newer than the
// installed one. Change the text and leave the number alone and the new charter ships,
// installs, and never replaces the AGENTS.md of any home that already has one — the very
// people the change was written for. CLAUDE.md says so in bold, and the tests said
// nothing: they assert that particular phrases are PRESENT and that the version is at
// least N, so an edit that adds a rule, or rewrites one, or deletes one those tests do
// not name, passes with the number untouched. Verified by editing the English charter
// and running the package: green.
//
// So the text is fingerprinted, and the fingerprint is filed under the version that
// carries it. Edit the charter and this fails with the new value to paste in.
//
// The boundary, stated: nothing here can force the number UP. Someone can satisfy this
// by overwriting the fingerprint for the current version, which is the right move while
// that version is still unreleased and being iterated on. What it removes is doing it by
// accident — the omission is now a line in the diff somebody chose to write.
func TestChangingTheCharterBumpsTheVersion(t *testing.T) {
	sum := charterFingerprint()
	want, known := playbookFingerprints[hqPlaybookVersion]
	if !known {
		t.Fatalf("hqPlaybookVersion is %d and no fingerprint is filed for it.\n"+
			"  Add it to playbookFingerprints in hq.go:  %d: %q,",
			hqPlaybookVersion, hqPlaybookVersion, sum)
	}
	if sum != want {
		t.Fatalf("the seeded charter text changed but hqPlaybookVersion is still %d.\n"+
			"  An existing HQ home re-seeds only when the shipped version is NEWER than the\n"+
			"  installed one, so this edit reaches nobody who already has a home.\n"+
			"    → bump hqPlaybookVersion, then file the new fingerprint:  %d: %q,\n"+
			"  If %d has not shipped and you are still iterating on it, replace its\n"+
			"  fingerprint instead — deliberately.",
			hqPlaybookVersion, hqPlaybookVersion+1, sum, hqPlaybookVersion)
	}
}

// charterFingerprint hashes both language halves, because both are the seeded charter:
// a home seeded in Chinese gets hqInstructionsZH, and an edit to only that half is just
// as invisible to an existing home as an edit to only the English one.
func charterFingerprint() string {
	h := sha256.New()
	h.Write([]byte(hqInstructionsEN))
	h.Write([]byte(hqInstructionsZH))
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// Both halves must actually reach the fingerprint — a hash over one language would let
// the other drift silently, which is the same defect one language further in.
func TestTheFingerprintCoversBothLanguages(t *testing.T) {
	base := charterFingerprint()
	for name, one := range map[string]string{"EN": hqInstructionsEN, "ZH": hqInstructionsZH} {
		h := sha256.New()
		h.Write([]byte(one))
		if hex.EncodeToString(h.Sum(nil))[:16] == base {
			t.Errorf("the fingerprint is the hash of %s alone — the other language could "+
				"change without anyone hearing about it", name)
		}
	}
}
