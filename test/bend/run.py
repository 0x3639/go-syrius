#!/usr/bin/env python3
"""Bend proof/differential/fault gate. Python standard library only."""
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile
import uuid

HERE = Path(__file__).resolve().parent
ROOT = HERE.parent.parent
FAMILIES = ("policy", "pending", "journal", "lifecycle", "input", "recovery")

# Every edit is a semantic, type-correct fault. Single-hit replacements make
# source drift an error requiring review, never a silently skipped fault.
MODEL_FAULTS = (
    ("effect-sender", "policy", "sender && (destination", "True{} && (destination"),
    ("effect-token", "policy", "token && (present", "True{} && (present"),
    ("effect-data", "policy", "(amount && data)", "(amount && True{})"),
    ("publication-durability", "policy", "(prepared && durable)", "(prepared && True{})"),
    ("mainnet-consent", "policy", "Bool.not(is_mainnet) || consent", "True{}"),
    ("reconnect-generation", "policy", "current && (embedded", "True{} && (embedded"),
    ("funded-redeem", "policy", "zero && znn", "znn"),
    ("decimal-bound", "policy", "Nat.is_le(value, 18n)", "Nat.is_le(value, 19n)"),
    ("busy-slot-overwrite", "pending", "Reply{False{}, State{counter, True{}, busy}}", "Reply{True{}, State{1n+counter, True{}, busy}}"),
    ("cancel-during-publish", "pending", "Reply{False{}, State{counter, held, True{}}}", "Reply{True{}, State{counter, False{}, True{}}}"),
    ("stale-cleanup", "pending", "case State{counter, held, busy} False{}:\n      State{counter, held, busy}", "case State{counter, held, busy} False{}:\n      State{counter, False{}, busy}"),
    ("forget-unknown-outcome", "journal", "Reply{False{}, Signed{payload}}", "Reply{True{}, Absent{}}"),
    ("change-published-payload", "journal", "case Signed{payload} Publish{}:\n      Reply{True{}, Published{payload}}", "case Signed{payload} Publish{}:\n      Reply{True{}, Published{1n+payload}}"),
    ("lock-retains-session", "lifecycle", "State{False{}, consent, False{}}", "State{False{}, consent, fresh}"),
    ("unlock-revives-stale-work", "lifecycle", "State{True{}, consent, fresh}", "State{True{}, consent, True{}}"),
    ("foreign-token-metadata", "input", "matches && bounded", "True{} && bounded"),
    ("corrupt-storage-accepted", "input", "case Corrupt{}:\n      False{}", "case Corrupt{}:\n      True{}"),
    ("recovery-ack-unknown", "recovery", "case Uncertain{payload}:\n      Reply{False{}, False{}, State{Uncertain{payload}, consent}}", "case Uncertain{payload}:\n      Reply{True{}, False{}, State{Missing{}, consent}}"),
    ("recovery-lost-reply", "recovery", "case True{} False{} _:\n      Reply{False{}, True{}, State{Uncertain{payload}, True{}}}", "case True{} False{} _:\n      Reply{False{}, True{}, State{Missing{}, True{}}}"),
)
GO_FAULTS = (
    ("sender", "app/tx_service.go", "b.Address != e.from || ", "", "TestBendEffect"),
    ("recipient", "app/tx_service.go", "b.ToAddress != e.to || ", "", "TestBendEffect"),
    ("token", "app/tx_service.go", "b.TokenStandard != e.zts || ", "", "TestBendEffect"),
    ("amount", "app/tx_service.go", "b.Amount.Cmp(e.amount) != 0", "false", "TestBendEffect"),
    ("call-data", "app/tx_service.go", "!bytes.Equal(b.Data, e.data)", "!bytes.Equal(b.Data, b.Data)", "TestBendEffect"),
    ("mainnet-consent", "app/tx_service.go", "if !s.AllowMainnetSend {", "if false && !s.AllowMainnetSend {", "TestBendPolicy"),
    ("decimals", "app/decimals.go", "d > 18", "d > 19", "TestBendPolicy"),
    ("autolock-boundary", "app/wallet_service.go", "now.Sub(w.lastActivity) > time.Duration", "now.Sub(w.lastActivity) >= time.Duration", "TestBendPolicy"),
    ("overwrite-hold", "app/tx_service.go", "if t.pending != nil {", "if false && t.pending != nil {", "TestBendPending"),
    ("cancel-publishing", "app/tx_service.go", "if t.publishingHold != 0 && (holdId", "if false && t.publishingHold != 0 && (holdId", "TestBendPending"),
    ("ack-unresolved", "app/walletconnect_journal.go", "if rec.State != wcStatePublished {", "if false && rec.State != wcStatePublished {", "TestBendJournal"),
    ("signed-before-broadcast", "app/tx_service.go", "if err := t.wcJournal.put(wcJournalKey(wcID.Topic, wcID.ID), rec); err != nil {", "if err := t.wcJournal.put(wcJournalKey(wcID.Topic, wcID.ID), rec); false && err != nil {", "TestBendPublication"),
    ("reconnect-generation", "app/node_service_reconnect.go", "current := n.connGen == gen && n.mode", "current := n.mode", "TestBendReconnect"),
    ("receive-session", "app/tx_service.go", "!ok || t.wallet.sessionGen() != gen || addr != published.Address", "false && (!ok || t.wallet.sessionGen() != gen || addr != published.Address)", "TestBendLifecycle"),
    ("receive-final-consent", "app/tx_service.go", "if err := t.guardChain(published.ChainIdentifier); err != nil {", "if err := t.guardChain(published.ChainIdentifier); false && err != nil {", "TestBendLifecycle"),
    ("metadata-token-identity", "app/decimals.go", "if tok.TokenStandard != zts {", "if false && tok.TokenStandard != zts {", "TestBendMetadataIdentity"),
    ("journal-unknown-state", "app/walletconnect_journal.go", "if rec.State != wcStateSigned && rec.State != wcStatePublished {", "if false && rec.State != wcStateSigned && rec.State != wcStatePublished {", "TestBendCorruptJournal"),
    ("journal-foreign-owner", "app/walletconnect_journal.go", 'if (rec.Topic != "" && rec.Topic != topic) || (rec.RequestID != 0 && rec.RequestID != id) {', 'if false && ((rec.Topic != "" && rec.Topic != topic) || (rec.RequestID != 0 && rec.RequestID != id)) {', "TestBendCorruptJournal"),
    ("journal-duplicate-fields", "app/walletconnect_journal.go", "if seen[key] {", "if false && seen[key] {", "TestBendCorruptJournal"),
    ("journal-block-schema", "app/walletconnect_journal.go", "if err := json.Unmarshal(rec.BlockJSON, &block); err != nil || block.Hash.String() != rec.Hash {", "if err := json.Unmarshal(rec.BlockJSON, &block); false && (err != nil || block.Hash.String() != rec.Hash) {", "TestBendCorruptJournal"),
    ("recovery-changes-amount", "app/walletconnect.go", "if block.Hash != hash {", "block.Amount = big.NewInt(42)\n\tif block.Hash != hash {", "TestBendRecovery"),
)


def replace_once(source, old, new):
    if source.count(old) != 1:
        raise RuntimeError(f"fault target must occur exactly once, found {source.count(old)}: {old!r}")
    return source.replace(old, new, 1)


def proof_passed(code, output):
    return code == 0 and "ALL PROOFS CHECK" in output and "SOME PROOFS FAIL" not in output


def bounded_setting(name, default, maximum):
    raw = os.environ.get(name, str(default))
    if not re.fullmatch(r"[1-9][0-9]{0,18}", raw) or int(raw) > maximum:
        raise RuntimeError(f"{name} must be 1..{maximum} without leading zeros")
    return raw


def classify_go_fault(code, output, test):
    # A mismatch printed before a later crash is still a broken experiment.
    errors = ("panic:", "fatal error:", "[build failed]", "[setup failed]", "Bend harness", "signal:", "test timed out", "DATA RACE")
    if code != 1 or any(s in output for s in errors):
        return "ERROR" if code != 0 else "MISSED"
    if "BEND_MISMATCH " in output and re.search(r"^--- FAIL: " + re.escape(test) + r" \(", output, re.M):
        return "caught"
    return "ERROR"


class Gate:
    def __init__(self):
        depth = os.environ.get("BEND_DEPTH", "3")
        if depth not in ("1", "2", "3", "4"):
            raise RuntimeError("BEND_DEPTH must be 1..4 without leading zeros")
        verdict = os.environ.get("VERDICT", "1")
        if verdict not in ("0", "1"):
            raise RuntimeError("VERDICT must be 0 or 1")
        self.verdict = verdict == "1"
        seed = bounded_setting("BEND_SEED", 1, 2**63 - 1)
        random_count = bounded_setting("BEND_RANDOM", 40, 1000)
        self.fuzz_seconds = bounded_setting("BEND_FUZZ_SECONDS", 10, 300)
        output_root = Path(os.environ.get("OUT", str(HERE / ".scratch" / "out"))).resolve()
        output_root.mkdir(parents=True, exist_ok=True)
        self.out = Path(tempfile.mkdtemp(prefix="run-", dir=output_root))
        (HERE / ".scratch").mkdir(exist_ok=True)
        # Public test sources must be traversable by the fixed, non-root
        # checker UID on Linux. Keep logs/Go overlays private inside it.
        (HERE / ".scratch").chmod(0o755)
        self.image = os.environ.get("BEND_IMAGE", "go-syrius-bend:2.0.40")
        self.verdict_image = os.environ.get("BEND_VERDICT_IMAGE", "go-syrius-bend:2.0.40-verdict")
        self.env = dict(os.environ, GOWORK="off", GOTOOLCHAIN="auto", BEND_IMAGE=self.image, BEND_DEPTH=depth, BEND_SEED=seed, BEND_RANDOM=random_count)
        (self.out / "00-settings.log").write_text(f"BEND_DEPTH={depth}\nBEND_SEED={seed}\nBEND_RANDOM={random_count}\nBEND_FUZZ_SECONDS={self.fuzz_seconds}\n")
        print(f"Bend cases: depth={depth}, seed={seed}, random histories={random_count}; fuzz={self.fuzz_seconds}s per target", flush=True)
        self.index = 0

    def command(self, name, args, timeout=600):
        self.index += 1
        path = self.out / f"{self.index:02d}-{name}.log"
        try:
            result = subprocess.run(args, cwd=ROOT, env=self.env, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True, timeout=timeout)
        except subprocess.TimeoutExpired as exc:
            raw = exc.stdout or b""
            path.write_text(raw.decode(errors="replace") if isinstance(raw, bytes) else raw)
            raise RuntimeError(f"{name} timed out; see {path}") from exc
        path.write_text(result.stdout)
        return result.returncode, result.stdout

    def required(self, name, args, timeout=600):
        code, output = self.command(name, args, timeout)
        if code:
            print(output[-6000:])
            raise RuntimeError(f"{name} failed ({code}); logs: {self.out}")
        return output

    def bend(self, name, path, verdict=False):
        container = "syrius-bend-" + uuid.uuid4().hex
        try:
            return self.command(name, ["docker", "run", "--rm", "--name", container,
                "--network", "none", "--read-only", "--cap-drop", "ALL",
                "--security-opt", "no-new-privileges", "--pids-limit", "256", "--memory", "2g",
                "--tmpfs", "/tmp:rw,nosuid,size=256m", "-v", f"{HERE}:/work:ro",
                self.verdict_image if verdict else self.image, path,
                "--verdict" if verdict else "--check-only"], 300)
        finally:
            # Also stop a container if the docker client timed out or was interrupted.
            subprocess.run(["docker", "rm", "-f", container], stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL, timeout=15)

    def build(self):
        for name in ("docker", "go"):
            if not shutil.which(name):
                raise RuntimeError(f"{name} is required; nothing will run on a host Node/Bend fallback")
        self.required("docker", ["docker", "info", "--format", "{{.ServerVersion}}"])
        # Rebuild by default so a changed Dockerfile cannot silently reuse a stale tag.
        if "BEND_IMAGE" not in os.environ:
            self.required("image", ["docker", "build", "-t", self.image, "-f", str(HERE / "Dockerfile"), str(HERE)], 1200)
        if self.verdict and "BEND_VERDICT_IMAGE" not in os.environ:
            self.required("verdict-image", ["docker", "build", "--build-arg", f"BASE={self.image}", "-t", self.verdict_image, "-f", str(HERE / "Dockerfile.verdict"), str(HERE)], 1800)

    def proofs(self):
        count = 0
        for family in FAMILIES:
            laws = re.findall(r"^law (\w+):", (HERE / family / "LAWS.bend").read_text(), re.M)
            proofs = re.findall(r"^def Laws\.(\w+)\(", (HERE / family / "PROOF.bend").read_text(), re.M)
            if not laws or len(laws) != len(set(laws)) or sorted(laws) != sorted(proofs):
                raise RuntimeError(f"{family}: laws/proof definitions do not match")
            count += len(laws)
            for verdict in ([False, True] if self.verdict else [False]):
                code, output = self.bend(f"{family}-{'verdict' if verdict else 'proof'}", f"{family}/PROOF.bend", verdict)
                if not proof_passed(code, output):
                    raise RuntimeError(f"{family}: proof check failed: {output}")
        print(f"PASS: {count} laws" + ("; independent verdict checked" if self.verdict else "; VERDICT SKIPPED (partial check)"), flush=True)

    def model_faults(self):
        for name, family, old, new in MODEL_FAULTS:
            with tempfile.TemporaryDirectory(prefix="mutant-", dir=HERE / ".scratch") as directory:
                dst = Path(directory)
                dst.chmod(0o755)
                for src in (HERE / family).glob("*.bend"):
                    shutil.copyfile(src, dst / src.name)
                    (dst / src.name).chmod(0o644)
                model = dst / "model.bend"
                model.write_text(replace_once(model.read_text(), old, new))
                relative = dst.relative_to(HERE).as_posix()
                code, output = self.bend(name + "-typecheck", f"{relative}/model.bend")
                if not proof_passed(code, output):
                    raise RuntimeError(f"{name}: malformed mutant (not a caught law): {output}")
                code, output = self.bend(name, f"{relative}/PROOF.bend")
                # Bend can return 0 even when a law fails: inspect its verdict.
                if code not in (0, 1) or "SOME PROOFS FAIL" not in output or "ALL PROOFS CHECK" in output or "Location: Laws." not in output:
                    raise RuntimeError(f"{name}: mutant missed or checker error: {output}")
            print(f"caught model fault: {name}", flush=True)

    def wallet(self):
        output = self.required("wallet", ["go", "test", "-tags", "bendrules", "./app", "-run", "^TestBend", "-count=1", "-v", "-timeout", "15m"], 960)
        tests = [test for path in sorted((ROOT / "app").glob("bend*_test.go"))
                 for test in re.findall(r"^func (TestBend\w+)\(", path.read_text(), re.M)]
        if not tests or any(not re.search(r"^--- PASS: " + t + r" \(", output, re.M) for t in tests):
            raise RuntimeError("not every Bend comparison ran and passed")
        for line in output.splitlines():
            if "compared " in line or line.startswith("--- PASS:"):
                print(line.strip(), flush=True)

    def fuzz(self):
        for test in ("FuzzWCJournalDecode", "FuzzWalletConnectIntent"):
            output = self.required(test, ["go", "test", "./app", "-run", "^$", "-fuzz", "^" + test + "$", "-fuzztime", self.fuzz_seconds + "s", "-parallel", "2", "-timeout", "10m"], 660)
            if not re.search(r"^PASS$", output, re.M):
                raise RuntimeError(f"{test}: fuzz target did not pass")
            print(f"PASS: {test} ({self.fuzz_seconds}s)", flush=True)

    def go_faults(self):
        for name, file, old, new, test in GO_FAULTS:
            with tempfile.TemporaryDirectory(prefix="fault-", dir=HERE / ".scratch") as directory:
                dst = Path(directory)
                src = ROOT / file
                mutant = dst / src.name
                mutant.write_text(replace_once(src.read_text(), old, new))
                overlay = dst / "overlay.json"
                overlay.write_text(json.dumps({"Replace": {str(src): str(mutant)}}))
                code, output = self.command("wallet-fault-" + name, ["go", "test", "-overlay", str(overlay), "-tags", "bendrules", "./app", "-run", "^" + test + "$", "-count=1", "-v", "-timeout", "5m"], 360)
                result = classify_go_fault(code, output, test)
                if result != "caught":
                    raise RuntimeError(f"{name}: {result}; see logs in {self.out}\n{output[-2500:]}")
            print(f"caught wallet fault: {name}", flush=True)


def main():
    if len(sys.argv) != 1:
        raise RuntimeError("usage: test/bend/run.sh (BEND_DEPTH=1..4, VERDICT=0|1, OUT=directory)")
    gate = Gate()
    gate.required("runner-tests", [sys.executable, "-m", "unittest", "discover", "-s", "test/bend", "-p", "test_runner.py", "-v"])
    gate.build()
    gate.proofs()
    gate.model_faults()
    gate.wallet()
    gate.go_faults()
    gate.fuzz()
    print(f"{'PASS' if gate.verdict else 'PARTIAL PASS (no independent verdict)'}: logs in {gate.out}")


if __name__ == "__main__":
    try:
        main()
    except (RuntimeError, OSError, subprocess.SubprocessError) as exc:
        print(f"FAIL: {exc}", file=sys.stderr)
        sys.exit(1)
