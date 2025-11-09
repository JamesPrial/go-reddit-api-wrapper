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
   - **Complexity assessment** and reviewer allocation strategy:
     - **Low complexity** (1-3 files, single package, <200 LOC): 1 reviewer
     - **Medium complexity** (4-8 files, 2-3 packages, 200-500 LOC): 2 reviewers
     - **High complexity** (9+ files, 4+ packages, >500 LOC, architectural changes): 2-3 reviewers
   - **Logical split strategy** for reviewers (if multiple reviewers):
     - By package (e.g., handlers, middleware, config)
     - By layer (e.g., API, business logic, storage)
     - By feature boundary (e.g., auth vs data vs UI)

### Phase 2: Parallel Code Writing
2. Use multiple code-writer subagents IN PARALLEL to write the implementation:
   - Delegate different files/modules to separate code-writer subagents simultaneously
   - Each code-writer should focus on their specific component
   - Wait for all writers to complete before proceeding

### Phase 3: Consolidated Code Review
3. Use CONSOLIDATED code-reviewer subagents based on complexity assessment:

   **Reviewer Allocation Strategy:**
   - **CRITICAL**: Use FEWER reviewers than writers to give each reviewer MORE context
   - Each reviewer should see related files together for coherent cross-file feedback
   - Number of reviewers determined in Phase 1 based on complexity

   **Low Complexity (1 reviewer):**
   - Single reviewer reviews ALL modified files together
   - Gets complete context of the entire change

   **Medium Complexity (2 reviewers):**
   - Split by logical boundaries (package, layer, or feature)
   - Example: Reviewer 1 = handlers + middleware, Reviewer 2 = config + utils
   - Each reviewer sees their entire logical section

   **High Complexity (2-3 reviewers):**
   - Split by architectural concerns or major subsystems
   - Example: Reviewer 1 = API layer, Reviewer 2 = business logic, Reviewer 3 = storage
   - Each reviewer gets deep context in their area

   **Review Output:**
   - Each reviewer documents ALL issues found (critical, warnings, suggestions)
   - Consolidate findings into a comprehensive, prioritized list of fixes needed

### Phase 4: MANDATORY Review-Fix Iteration Loop

**THIS IS CRITICAL - YOU MUST ITERATE UNTIL CLEAN:**

4a. **IF ANY issues were found in the review:**
   - Use code-writer subagents to fix ALL identified issues
   - WAIT for all fixes to be written

4b. **THEN run the SAME consolidated code-reviewer subagents AGAIN:**
   - Each reviewer re-reviews their assigned logical section
   - Check if the fixes resolved the issues
   - Check if new issues were introduced
   - Maintain same reviewer allocation as Phase 3 for consistency

4c. **REPEAT steps 4a-4b until:**
   - Code-reviewer finds ZERO critical issues
   - Code-reviewer finds ZERO warnings
   - Code-reviewer confirms all suggestions have been addressed or explicitly deferred
   - **DO NOT PROCEED to testing until this condition is met**

**Important Notes for Phase 4:**
- This is NOT a one-time review-fix-done process
- You MUST iterate: write → review → fix → review → fix → review... until clean
- Each iteration should show measurable improvement
- Track iteration count and remaining issues after each cycle
- If stuck in a loop (3+ iterations with same issues), escalate to user

### Phase 5: Testing
5. Once code passes ALL review iterations (ZERO issues remaining):
   - Use the test-runner subagent to run all relevant tests
   - IF tests fail:
     - Analyze failures thoroughly
     - Use code-writer subagents to fix failing tests
     - Re-run code-reviewer subagents on the test fixes
     - Re-run tests until all pass

### Phase 6: Git Operations
6. ONLY after all tests pass AND code review is clean:
   - Use git-ops subagent to:
     - Stage all changes
     - Create a descriptive commit message
     - Commit the changes
     - Push to remote

## Critical Requirements

- **Parallel execution**: Code-writers MUST work in parallel when possible, not sequentially
- **Consolidated review**: Use FEWER reviewers than writers to maximize context per reviewer
- **Writers parallel, reviewers consolidate**: Writers work independently, reviewers see the big picture
- **No skipping steps**: Every phase must complete before moving to the next
- **MANDATORY iteration**: MUST iterate on review feedback until COMPLETELY resolved
- **Zero-tolerance gate**: Code review must find ZERO issues before proceeding
- **Testing gate**: MUST NOT commit until all tests pass
- **Clear communication**: Report progress after each phase and iteration

## Example Iteration Report Format

After each review-fix cycle, report status like this:

```
Iteration 1: Found 12 issues (5 critical, 4 warnings, 3 suggestions)
→ Using code-writer to fix all 12 issues...

Iteration 2: Found 3 issues (0 critical, 2 warnings, 1 suggestion)
→ Using code-writer to fix remaining 3 issues...

Iteration 3: Found 0 issues - CODE REVIEW CLEAN ✓
→ Proceeding to testing phase...
```

Begin implementation now following this workflow.