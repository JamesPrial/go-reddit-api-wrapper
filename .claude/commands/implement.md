# Create the command file
mkdir -p .claude/commands
cat > .claude/commands/implement.md << 'EOF'
---
description: Implement a feature using coordinated subagents workflow - plan, write, review, test, and commit
---

# Implementation Workflow

Implement: $ARGUMENTS

## Required Workflow Process

You MUST follow this exact workflow pattern:

### Phase 1: Planning
1. First, create a detailed implementation plan that includes:
   - Files that need to be created or modified
   - Key functions/classes/modules to implement
   - Dependencies and integration points
   - Testing strategy
   - Expected timeline/complexity

### Phase 2: Parallel Code Writing
2. Use multiple code-writer subagents IN PARALLEL to write the implementation:
   - Delegate different files/modules to separate code-writer subagents simultaneously
   - Each code-writer should focus on their specific component
   - Wait for all writers to complete before proceeding

### Phase 3: Code Review
3. Use code-reviewer subagents to review the written code:
   - Run code-reviewer subagent on all modified files
   - Document all issues found (critical, warnings, suggestions)
   - Create a prioritized list of fixes needed

### Phase 4: Iteration Loop
4. IF code review found issues:
   - Use code-writer subagents to fix the identified issues
   - Run code-reviewer subagents again on the fixes
   - REPEAT this cycle until code-reviewer finds NO critical issues

### Phase 5: Testing
5. Once code passes review:
   - Use the test-runner subagent to run all relevant tests
   - IF tests fail:
     - Analyze failures
     - Use code-writer to fix failing tests
     - Re-run tests until all pass

### Phase 6: Git Operations
6. ONLY after all tests pass:
   - Use git-ops subagent to:
     - Stage all changes
     - Create a descriptive commit message
     - Commit the changes
     - Push to remote

## Critical Requirements

- **Parallel execution**: Code-writers MUST work in parallel, not sequentially
- **No skipping steps**: Every phase must complete before moving to the next
- **Iteration**: MUST iterate on review feedback until resolved
- **Testing gate**: MUST NOT commit until all tests pass
- **Clear communication**: Report progress after each phase

Begin implementation now following this workflow.