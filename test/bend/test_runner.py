"""The gate must never count a broken experiment as detecting a wallet fault."""
import unittest
import os
from pathlib import Path
import shutil
import tempfile
from unittest.mock import patch
from run import Gate, HERE, MODEL_FAULTS, bounded_setting, classify_go_fault, proof_passed, replace_once

class RunnerTests(unittest.TestCase):
    def test_public_mutants_remain_readable_with_private_umask(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            shutil.copytree(HERE / 'policy', root / 'policy')
            old_mask = os.umask(0o077)
            try:
                with patch('run.HERE', root), patch('run.MODEL_FAULTS', MODEL_FAULTS[:1]), patch.dict('os.environ', {'OUT': str(root / 'logs')}):
                    gate = Gate()
                    def check_source(_name, path, verdict=False):
                        target = root / path
                        for parent in (root / '.scratch', target.parent):
                            self.assertEqual(parent.stat().st_mode & 0o777, 0o755)
                        for source in target.parent.glob('*.bend'):
                            self.assertEqual(source.stat().st_mode & 0o777, 0o644)
                        return (0, 'ALL PROOFS CHECK' if target.name == 'model.bend' else 'SOME PROOFS FAIL\nLocation: Laws.effect_requires_sender')
                    with patch.object(gate, 'bend', side_effect=check_source):
                        gate.model_faults()
                    self.assertEqual(list((root / '.scratch').glob('mutant-*')), [])
            finally:
                os.umask(old_mask)

    def test_seed_and_budget_validation(self):
        with patch.dict('os.environ', {}, clear=True):
            self.assertEqual(bounded_setting('BEND_SEED', 1, 2**63 - 1), '1')
        for value in ('0', '-1', '01', '1.5', ' 1', '1\n', 'x', str(2**63)):
            with patch.dict('os.environ', {'BEND_SEED': value}):
                with self.assertRaises(RuntimeError):
                    bounded_setting('BEND_SEED', 1, 2**63 - 1)
        with patch.dict('os.environ', {'BEND_SEED': str(2**63 - 1)}):
            self.assertEqual(bounded_setting('BEND_SEED', 1, 2**63 - 1), str(2**63 - 1))

    def test_fault_requires_named_failure_and_mismatch(self):
        good = 'BEND_MISMATCH sender\n--- FAIL: TestBendEffect (0.2s)\nFAIL\n'
        self.assertEqual(classify_go_fault(1, good, 'TestBendEffect'), 'caught')
        for text in ('--- FAIL: TestBendEffect (0.2s)', 'BEND_MISMATCH sender', good.replace('TestBendEffect', 'TestOther')):
            self.assertEqual(classify_go_fault(1, text, 'TestBendEffect'), 'ERROR')
        for error in ('panic:', 'fatal error:', '[build failed]', '[setup failed]', 'Bend harness', 'signal:', 'test timed out', 'DATA RACE'):
            self.assertEqual(classify_go_fault(1, good + error, 'TestBendEffect'), 'ERROR')
        self.assertEqual(classify_go_fault(-9, good, 'TestBendEffect'), 'ERROR')
        self.assertEqual(classify_go_fault(0, good, 'TestBendEffect'), 'MISSED')

    def test_proof_success_is_explicit(self):
        self.assertTrue(proof_passed(0, 'ALL PROOFS CHECK'))
        for code, text in ((1, 'ALL PROOFS CHECK'), (0, ''), (0, 'SOME PROOFS FAIL'), (0, 'ALL PROOFS CHECK\nSOME PROOFS FAIL')):
            self.assertFalse(proof_passed(code, text))

    def test_mutation_target_drift_is_an_error(self):
        self.assertEqual(replace_once('abc', 'b', 'x'), 'axc')
        for source in ('abc', 'bb'):
            with self.assertRaises(RuntimeError):
                replace_once(source, 'b' if source == 'bb' else 'z', 'x')
