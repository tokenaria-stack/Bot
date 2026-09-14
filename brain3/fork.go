package brain3

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ForkResult struct {
	PreDigest  string
	PostDigest string
	Preflight  string
	Audit      AuditResult
	Probe      BinaryProbeResult
	Verdict    string
	VerdictWhy string
	Text       string
}

func RunTPStopFork(ds Dataset, val Validation, python, repo, work string) (ForkResult, error) {
	var z ForkResult
	spec := BinaryProbeSpec1()
	pre, err := spec.DigestHex(ds, val)
	if err != nil {
		return z, err
	}
	z.PreDigest = pre
	census, err := ResolveOuterFolds(ds, val)
	if err != nil {
		return z, err
	}
	geoms, err := PreflightBinaryGeometry(ds, census, spec)
	if err != nil {
		return z, err
	}
	z.Preflight = FormatBinaryPreflight(geoms)
	audit, err := RunInformationAudit(ds, geoms)
	if err != nil {
		return z, err
	}
	z.Audit = audit
	post, err := spec.DigestHex(ds, val)
	if err != nil {
		return z, err
	}
	z.PostDigest = post
	if post != pre {
		return z, fmt.Errorf("brain3: SPEC_MUTATED_AFTER_AUDIT")
	}
	probe, err := RunBinaryProbe(ds, val, spec, geoms, python, repo, work)
	if err != nil {
		return z, err
	}
	z.Probe = probe
	z.Verdict, z.VerdictWhy = describeFork(probe)
	z.Text = FormatFork(z, ds, val)
	return z, nil
}

func FormatFork(z ForkResult, ds Dataset, val Validation) string {
	var b strings.Builder
	b.WriteString("TP-STOP-INFORMATION-FORK-1\n")
	b.WriteString("A. FROZEN PROBE SPEC (declared before audit)\n")
	b.WriteString(fmt.Sprintf("digest=%s population=resolved_only schema=[STOP_FIRST, TP_FIRST] loss=Logloss depth=6 lr=0.03 l2=3 max=1000 span=35040 min_inner=1000 H=72\n", z.PreDigest))
	b.WriteString(fmt.Sprintf("dataset=%s validation=%s split=%s width=%d\n", ds.ContentHex, val.PlanHex, val.SplitHex, ds.Width))
	b.WriteString("B. PREFLIGHT\n")
	b.WriteString(z.Preflight)
	b.WriteString("C. INFORMATION AUDIT\n")
	b.WriteString(z.Audit.Text)
	b.WriteString("D. BINARY PORTABLE: classCount=2 margin+sigmoid; 3-class walker unchanged; Python≈Go checked per fold.\n")
	b.WriteString(z.Probe.Text)
	b.WriteString("F. optional coverage grid omitted (logloss owns the probe verdict).\n")
	b.WriteString("G. VERDICT\n")
	b.WriteString(z.Verdict + "\n" + z.VerdictWhy + "\n")
	b.WriteString(fmt.Sprintf("post_audit_spec_digest=%s (unchanged)\n", z.PostDigest))
	b.WriteString("CORRECTIONS: inner exclusive end = NextBarOpen(last RESOLVED train At) because SplitCausalTail requires that contract;\n")
	b.WriteString("binary Y maps dataset TP=0/STOP=1 onto STOP=0/TP=1; TIMEOUT dropped after frozen outer membership;\n")
	b.WriteString("no coverage grid (prompt allowed omit); no a*q; no 2026.\n")
	return b.String()
}

func writeForkArtifacts(root string, z ForkResult) error {
	dir := filepath.Join(root, "research", "brain3")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "tp-stop-information-fork-1.txt"), []byte(z.Text), 0o644)
}
