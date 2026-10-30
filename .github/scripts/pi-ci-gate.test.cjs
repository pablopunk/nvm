const assert = require('node:assert/strict');
const test = require('node:test');
const { evaluatePiCiGate, REQUIRED_PI_JOBS } = require('./pi-ci-gate.cjs');

function successfulInput(overrides = {}) {
  return {
    classificationResult: 'success',
    relevant: 'true',
    checkResult: 'success',
    ...Object.fromEntries(REQUIRED_PI_JOBS.map((job) => [job, 'success'])),
    ...overrides,
  };
}

test('Pi gate requires every same-run compatibility job for relevant changes', function relevantChanges() {
  assert.equal(evaluatePiCiGate(successfulInput()).success, true);
  for (const job of REQUIRED_PI_JOBS) {
    assert.equal(
      evaluatePiCiGate(successfulInput({ [job]: 'skipped' })).success,
      false,
      `${job} cannot be skipped for a relevant Pi change`,
    );
    assert.equal(
      evaluatePiCiGate(successfulInput({ [job]: undefined })).success,
      false,
      `${job} must be present for a relevant Pi change`,
    );
  }
});

test('Pi gate fails closed on classifier, check, malformed output, and unavailable job results', function failClosed() {
  for (const input of [
    successfulInput({ classificationResult: 'failure' }),
    successfulInput({ checkResult: 'failure' }),
    successfulInput({ relevant: '' }),
    successfulInput({ relevant: 'maybe' }),
    successfulInput({ 'pi-packaged-macos': 'cancelled' }),
    successfulInput({ 'pi-packaged-macos': 'neutral' }),
  ])
    assert.equal(evaluatePiCiGate(input).success, false);
});

test('unrelated changes may omit Pi jobs only after successful false classification', function unrelatedChanges() {
  assert.equal(
    evaluatePiCiGate(
      successfulInput({
        relevant: 'false',
        'pi-integration-tests': 'skipped',
        'packaged-dependency-contracts': 'skipped',
        'packaged-diagnostics': 'skipped',
        'pi-packaged-macos': 'skipped',
      }),
    ).success,
    true,
  );
  assert.equal(
    evaluatePiCiGate(
      successfulInput({ relevant: 'false', classificationResult: 'skipped' }),
    ).success,
    false,
  );
});
