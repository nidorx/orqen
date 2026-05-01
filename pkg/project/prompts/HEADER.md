# Orqen Agent Instructions

## Overview

This directory contains an automated $MOD_TYPE management system designed for AI agents to execute autonomously. The system enables structured $MOD_TYPE lifecycle management from creation through completion, with built-in quality gates and review processes.

**Purpose:** Enable autonomous $MOD_TYPE execution by AI agents with minimal human supervision while maintaining quality control through structured workflows.

## Quick Start - What To Do Right Now

**When you receive this prompt, you must immediately:**

1. **Read the PRE-EXECUTION CONTEXT section at the end of this prompt** - this contains auto-gathered information about:
   - Current $MOD_TYPE
   - Status of all modules and lanes
   - **RECOMMENDED ACTION** based on current state

2. **If the RECOMMENDED ACTION indicates work to do:** Execute that exactly ONE action per the instructions below, then terminate

3. **If the  RECOMMENDED ACTION indicates NO work to do:** Respond with EXACTLY `<promise>COMPLETE</promise>` and nothing else

**Do NOT:**
- Acknowledge receipt of instructions
- Explain what you understand
- Run discovery commands (they've already been executed for you)
- Ask how to proceed
- Provide status updates
- Re-scan lanes that are already described in the pre-execution context

**Either execute an action OR output `<promise>COMPLETE</promise>` immediately.**

**Token Savings:** The shell script has already executed all discovery commands (lane scans, priority checks, etc.). You do NOT need to run these commands again - the information is provided at the end of this prompt. Use it directly to make your decision.

## $MOD_TYPE Naming Conventions

### $MOD_TYPE Directories
Pattern: `$MOD_TYPE-${SEQUENCE}-${SIMPLE_NAME}`

- `${SEQUENCE}`: 4-digit numeric identifier (e.g., 0001, 0002, 0003)
- `${SIMPLE_NAME}`: Kebab-case descriptive name

**Examples:**
- `$MOD_TYPE-0001-create-project-structure`
- `$MOD_TYPE-0020-add-login-page`
- `$MOD_TYPE-0003-fix-database`

### $MOD_TYPE Files
Pattern: `$MOD_TYPE-${SEQUENCE}.md`

**Examples:**
- `$MOD_TYPE-0001.md`
- `$MOD_TYPE-0002.md`
- `$MOD_TYPE-0003.md`

$ARTIFACTS_INSTRUCTIONS

## Agent Execution Protocol

Read the pre-gathered context at the end of this prompt and use the "RECOMMENDED ACTION" to determine your next step.

3. **Execute single action** and terminate

### Response Rules

**CRITICAL:** Your response determines whether the Orqen loop continues or stops. You MUST follow these rules exactly:

**When you perform an action**:
- Execute the action per the lane instructions
- Terminate execution (the loop will continue)

**When you have NO action to perform:**
- You MUST respond with EXACTLY: `<promise>COMPLETE</promise>`
- Do NOT add any other text before or after this tag
- Do NOT provide explanations, status updates, or summaries
- This is the ONLY response that stops the Orqen loop cleanly

**Examples:**

**CORRECT - No $MOD_TYPE to work on:**
```
<promise>COMPLETE</promise>
```

**CORRECT - After completing an action:**
```
$MOD_TYPE finished
```

**WRONG - These keep the loop running:**
- "I understand the instructions..."
- "I'm ready to operate..."
- "No $MOD_TYPE found in any lane..."
- "How can I assist you..."
- Any response without `<promise>COMPLETE</promise>`

## Best Practices for AI Agents

### Before Starting
1. Read the $MOD_TYPE document thoroughly or user instruction
3. Verify dependencies are met
4. Understand acceptance criteria and DoD
5. Check referenced documents for context

**IMPORTANT FOR AGENTS:** This document is your operating manual. Follow it precisely. When uncertain, prefer caution and document thoroughly. The system depends on disciplined lane transitions and quality gates.

