# Claude Code Active Testing Orchestrator

Think deeply about actively running, troubleshooting, and iterating on the Claude Code test suite. You will execute tests, debug failures, research solutions, modify tests as needed, and ensure comprehensive validation of Claude Code automation capabilities.

## 🚀 ACTIVE TESTING MISSION

**CRITICAL**: This is an active testing orchestrator that:
- **Executes the test suite** and monitors progress in real-time
- **Troubleshoots failures** by analyzing logs and error messages
- **Iterates on test scripts** to fix issues
- **Researches solutions** using available tools and documentation
- **Re-runs tests** until they pass or root causes are identified
- **Documents findings** throughout the process

## 📋 PREREQUISITES CHECK

First, ensure the test suite exists and is ready:

```bash
# Check if test suite exists
if [ ! -d "claude-code-tests" ]; then
    echo "Error: Test suite not found. Creating it now..."
    # You may need to recreate the test suite
fi

# Make scripts executable
chmod +x claude-code-tests/*.sh

# Check Claude is authenticated
claude --version || echo "Claude may need authentication"
```

## 🎯 ARGUMENTS

Parse the following arguments from "$ARGUMENTS":
1. `test_scope` - Which tests to run (default: all)
   Options: all, basic, advanced, specific-test-name, failed-only
   
2. `max_iterations` - Maximum retry attempts per test (default: 3)
   
3. `debug_level` - How much debugging info to collect (default: normal)
   Options: minimal, normal, verbose, extreme
   
4. `fix_mode` - How aggressively to fix issues (default: moderate)
   Options: observe-only, moderate, aggressive

## PHASE 1: INITIAL TEST EXECUTION

1. **Start Test Suite**:
   ```bash
   cd claude-code-tests
   ./master-test-runner.sh 2>&1 | tee test_run_$(date +%Y%m%d_%H%M%S).log
   ```

2. **Monitor Execution**:
   - Watch for test failures in real-time
   - Capture error messages and exit codes
   - Note which tests timeout or hang

3. **Initial Assessment**:
   - Identify passing vs failing tests
   - Categorize failure types:
     - Authentication issues
     - Timeout problems
     - Command syntax errors
     - Output parsing failures
     - Environment issues

## PHASE 2: TROUBLESHOOTING & RESEARCH

For each failed test:

1. **Analyze Failure**:
   ```bash
   # Check specific test log
   tail -100 logs/test-XX-*.log
   
   # Look for error patterns
   grep -E "(ERROR|FAILED|timeout|permission)" logs/*.log
   ```

2. **Research Solutions**:
   - Use Context7 to search Claude Code documentation for error patterns
   - Search web for similar issues and solutions
   - Check if it's a known limitation or bug

3. **Identify Root Cause**:
   - Authentication/API key issues?
   - Environment configuration problems?
   - Test assumptions incorrect?
   - Claude Code behavior changed?
   - System resource constraints?

## PHASE 3: ITERATIVE FIXES

Based on fix_mode setting:

### Observe-Only Mode:
- Document issues without modifying tests
- Create detailed failure report
- Suggest fixes for manual implementation

### Moderate Mode:
- Fix obvious issues (paths, timeouts, syntax)
- Add better error handling
- Improve logging for debugging
- Re-run affected tests

### Aggressive Mode:
- Rewrite failing test logic
- Add retry mechanisms
- Create workarounds for limitations
- Modify test expectations if needed
- Add new tests to validate fixes

## PHASE 4: FIX IMPLEMENTATION

Common fixes to attempt:

1. **Authentication Issues**:
   ```bash
   # Check if Claude needs login
   claude --version
   
   # Add longer timeouts for first-time auth
   sed -i 's/timeout 30/timeout 60/g' test-*.sh
   ```

2. **Timeout Problems**:
   ```bash
   # Increase timeouts for slow operations
   # Add exponential backoff
   # Split large tests into smaller chunks
   ```

3. **Output Parsing**:
   ```bash
   # Add more robust JSON parsing
   # Handle empty outputs gracefully
   # Fix regex patterns for different OS
   ```

4. **Environment Issues**:
   ```bash
   # Check Node version compatibility
   # Verify Claude Code installation
   # Fix path issues
   ```

## PHASE 5: RE-RUN AND VALIDATE

1. **Re-run Failed Tests**:
   ```bash
   # Run specific failed test
   ./test-XX-name.sh 2>&1 | tee rerun_$(date +%s).log
   
   # If passes, run full suite again
   ./master-test-runner.sh
   ```

2. **Verify Fixes**:
   - Ensure previously passing tests still pass
   - Confirm failed tests now succeed
   - Check for new issues introduced

3. **Document Changes**:
   - What was fixed and why
   - Any workarounds implemented
   - Remaining known issues

## PHASE 6: CONTINUOUS ITERATION

Repeat phases 2-5 until either:
- All tests pass successfully
- Maximum iterations reached
- Root causes identified for unfixable issues

## PHASE 7: FINAL REPORTING

Generate comprehensive report including:

1. **Test Results Summary**:
   - Initial pass/fail rates
   - Final pass/fail rates
   - Tests fixed during iteration
   - Remaining failures with root causes

2. **Key Discoveries**:
   - New findings about Claude Code behavior
   - Limitations discovered
   - Best practices identified
   - Performance insights

3. **Implementation Guide**:
   - Fixed test scripts
   - Updated automation patterns
   - Troubleshooting playbook
   - Team recommendations

## 🔧 TROUBLESHOOTING TOOLKIT

Use these approaches for different error types:

```bash
# For hanging tests
timeout --preserve-status 300 ./test-name.sh

# For authentication issues
export ANTHROPIC_API_KEY="..." # If needed
claude logout && claude login

# For parsing errors
./test-name.sh 2>&1 | python3 -m json.tool

# For performance issues
time ./test-name.sh
strace -c ./test-name.sh 2>/dev/null

# For environment issues
env | grep -E "(CLAUDE|ANTHROPIC|NODE)"
which claude && claude --version
```

## 📊 REAL-TIME MONITORING

While tests run, continuously:
- Check system resources (CPU, memory)
- Monitor Claude processes
- Watch for hanging operations
- Track test progress

## 🎯 SUCCESS CRITERIA

The testing is complete when:
1. All critical tests pass consistently
2. Failures have documented root causes
3. Workarounds exist for known limitations
4. Team has clear automation patterns
5. Performance baselines established

Begin by checking the test suite status and starting the initial test run. Monitor actively and be prepared to troubleshoot and iterate as needed.