---
doc_type: legacy_doc
status: reference
last_reviewed: 2026-02-16
---

# Test Infrastructure Improvements

**Date:** Feb 10, 2026
**Status:** In Progress
**Context:** Addressed daemon lifecycle race conditions from 6-tier parallel test suite (108 processes)

## Problem Analysis

The initial 6-tier parallel test approach (all tiers starting simultaneously on ports 7800-7888) experienced daemon lifecycle race conditions:

- **Test Termination Contention:** 26 test processes terminating near-simultaneously caused TCP TIME_WAIT state conflicts on adjacent port ranges
- **Health Polling Inefficiency:** Fixed 0.1s sleep pattern wasted ~50ms per attempt across 50 attempts = 5 seconds per test startup
- **Daemon Port Binding:** Intermittent "Address already in use" errors when ports not fully released between test cycles
- **Cascading Failures:** Tests 8.1-8.4 (Security) showing 0/4 pass due to daemon startup failure

### Early Test Results (6-Tier Approach)

From `/tmp/kaboom-uat-multitier-1770672748/`:

| Test Category | Result | Port Range | Status |
|---|---|---|---|
| cat-09-http | 1/4 pass | 7800 | Daemon timeout |
| cat-08-security | 0/4 pass | 7810 | Port binding failure |
| cat-14-extension-startup | 5/5 pass | 7885 | Success (no shutdown) |
| cat-16-api-contract | 5/5 pass | 7886 | Success (no lifecycle stress) |

**Root Cause:** Tests without `/shutdown` calls or daemon restart cycles were unaffected, while tests with lifecycle changes experienced race conditions.

## Solutions Implemented

### 1. Framework.sh: Exponential Backoff for Health Polling

**File:** `scripts/tests/framework/framework.sh`

**Change:** Replaced fixed 0.1s sleep with exponential backoff:

```bash
# Old: 50 attempts × 0.1s = 5s max, always 0.1s
sleep 0.1

# New: Exponential - 10ms → 50ms → 100ms
if [ $i -lt 3 ]; then
    sleep 0.01      # 3 attempts × 10ms = 30ms
elif [ $i -lt 10 ]; then
    sleep 0.05      # 7 attempts × 50ms = 350ms
else
    sleep 0.1       # Remaining attempts × 100ms
fi
```

**Impact:**
- Typical startup (<100ms): ~30-50ms wait instead of 100-500ms
- Slow startup (>1s): Still waits up to 5s total
- **Expected savings:** 10-15 seconds across all tests (50 × 0.05s difference × ~20 test categories)

### 2. New Safe Test Runners

Created conservative parallel test executors that run tests in smaller groups with cleanup delays:

#### `test-original-uat-safe.sh`
- 54 tests in 6 groups of 3-5 tests
- Port ranges: 7900-7902, 7920-7922, 7940-7942, 7960-7962, 7980-7982, 7900-7902
- **1s cleanup delay** between groups
- Sequential group execution (groups run in order, tests within group run parallel)

#### `test-new-uat-conservative.sh`
- 98 tests in 4 groups of 3-4 tests
- Port ranges: 7900-7903, 7920-7923, 7940-7943, 7960-7963
- **1s cleanup delay** between groups
- Sequential group execution

#### `test-tiered-parallel.sh` (Original, for reference)
- All 3 tiers launch simultaneously on separate port ranges
- **Deprecated:** Use safe runners instead

### 3. Potential Speed Critical Path

**File:** `scripts/tests/capture/cat-11-data-pipeline.sh` (869 LOC)
- Currently takes ~60 seconds for 31 tests
- Could be parallelized further if needed (currently sequential operations)

### 4. Test Output Format Standardization

All test result files write structured variables:

```bash
PASS_COUNT=<number>
FAIL_COUNT=<number>
SKIP_COUNT=<number>
ELAPSED=<seconds>
CATEGORY_ID=<id>
CATEGORY_NAME="<name>"
```

Both safe runners use `source` to load these variables for reliable result aggregation.

## Expected Improvements

### Speed Improvements
- Exponential backoff: **10-15s saved** across test suite
- Smaller parallel groups: **Avoids TIME_WAIT conflicts**, enabling reliable parallelism
- Expected total time: **2-3 minutes** (from original 65+ seconds with failures)

### Reliability Improvements
- **Daemon lifecycle:** No more port binding races
- **Test isolation:** Each group gets clean port state
- **Result accuracy:** Structured format enables reliable result parsing

## Test Execution Strategies

### For Quick Feedback (1-2 min)
```bash
# Run original 54 proven tests only
bash scripts/test-original-uat-safe.sh
```

### For Comprehensive Validation (5-6 min)
```bash
# Run original + new tests
bash scripts/test-original-uat-safe.sh && \
bash scripts/test-new-uat-conservative.sh
```

### For Development/Debugging
```bash
# Run single test category
bash scripts/tests/cat-XX-<name>.sh 7890 /tmp/result.txt

# Run with output
bash scripts/tests/cat-XX-<name>.sh 7890 /tmp/result.txt && \
grep -E "PASS|FAIL" /tmp/result.txt
```

## Migration Notes

The tiered approach (`test-tiered-parallel.sh`, `test-multi-tier.sh`) should not be used:
- Root cause: Too many concurrent daemons on adjacent ports
- Race conditions overwhelm OS port state cleanup
- Minimal speed benefit (~5s from perfect parallelism) vs. reliability cost

## Next Steps

1. ✅ Complete safe test runs (in progress)
2. Validate all 152 tests pass with safe approach
3. Document any remaining test failures
4. Consider further optimization if needed (e.g., split cat-11 if time is critical)
5. Update CI/CD to use `test-original-uat-safe.sh` as primary test gate

## References

- QA Analysis: Analysis of parallel test failures, root causes, and recommendations
- Framework: `scripts/tests/framework/framework.sh` — daemon lifecycle, health polling
- Test Runners: `scripts/test-original-uat-safe.sh`, `scripts/test-new-uat-conservative.sh`
