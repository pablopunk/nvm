const REQUIRED_PI_JOBS = [
  'pi-integration-tests',
  'electron-smoke',
  'packaged-dependency-contracts',
  'packaged-diagnostics',
  'pi-packaged-macos',
];

function evaluatePiCiGate(input) {
  if (input.classificationResult !== 'success')
    return {
      success: false,
      reason: 'Pi change classification did not succeed.',
    };
  if (input.checkResult !== 'success')
    return {
      success: false,
      reason: 'The required check job did not succeed.',
    };
  if (input.relevant !== 'true' && input.relevant !== 'false')
    return {
      success: false,
      reason: 'Pi classification output must be true or false.',
    };
  if (input.relevant === 'false')
    return {
      success: true,
      reason: 'Unrelated changes were classified successfully.',
    };
  for (const job of REQUIRED_PI_JOBS) {
    if (input[job] !== 'success')
      return {
        success: false,
        reason: `Required Pi coverage job ${job} concluded ${input[job] || 'missing'}.`,
      };
  }
  return {
    success: true,
    reason: 'All required Pi compatibility jobs succeeded.',
  };
}

if (require.main === module) {
  const result = evaluatePiCiGate({
    classificationResult: process.env.PI_CLASSIFICATION_RESULT,
    relevant: process.env.PI_RELEVANT,
    checkResult: process.env.PI_CHECK_RESULT,
    ...Object.fromEntries(
      REQUIRED_PI_JOBS.map((job) => [
        job,
        process.env[`PI_JOB_${job.replaceAll('-', '_')}`],
      ]),
    ),
  });
  console.log(result.reason);
  if (!result.success) process.exitCode = 1;
}

module.exports = { REQUIRED_PI_JOBS, evaluatePiCiGate };
