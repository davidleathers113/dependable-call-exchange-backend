**INFINITE AGENTIC LOOP COMMAND - ORIGINAL PATTERN TEST**

Think deeply about this infinite generation task. You are about to embark on a sophisticated iterative creation process.

**Variables:**
spec_file: $ARGUMENTS
output_dir: $ARGUMENTS
count: $ARGUMENTS

**ARGUMENTS PARSING:**
Parse the following arguments from "$ARGUMENTS":
1. `spec_file` - Path to the markdown specification file
2. `output_dir` - Directory where iterations will be saved  
3. `count` - Number of iterations (1-N or "infinite")

**PHASE 1: SPECIFICATION ANALYSIS**
Read and deeply understand the specification file at `spec_file`. This file defines:
- What type of content to generate
- The format and structure requirements
- Any specific parameters or constraints
- The intended evolution pattern between iterations

**PHASE 2: OUTPUT DIRECTORY RECONNAISSANCE** 
Thoroughly analyze the `output_dir` to understand the current state:
- List all existing files and their naming patterns
- Identify the highest iteration number currently present
- Determine what gaps or opportunities exist for new iterations

**PHASE 3: ITERATION STRATEGY**
Based on the spec analysis and existing iterations:
- Determine the starting iteration number (highest existing + 1)
- Plan how each new iteration will be unique and evolutionary
- If count is "infinite", prepare for continuous generation until context limits

**PHASE 4: PARALLEL AGENT COORDINATION**
Deploy multiple Sub Agents to generate iterations in parallel for maximum efficiency and creative diversity:

**Sub-Agent Distribution Strategy:**
- For count 1-5: Launch all agents simultaneously using Task tool
- For count 6-20: Launch in batches of 5 agents to manage coordination
- For "infinite": Launch waves of 3-5 agents, monitoring context and spawning new waves

**Agent Assignment Protocol:**
Each Sub Agent receives:
1. **Spec Context**: Complete specification file analysis
2. **Directory Snapshot**: Current state of output_dir at launch time
3. **Iteration Assignment**: Specific iteration number (starting_number + agent_index)
4. **Uniqueness Directive**: Explicit instruction to avoid duplicating concepts from existing iterations
5. **Quality Standards**: Detailed requirements from the specification

**CRITICAL: Use the Task tool to spawn each Sub Agent with this exact prompt structure:**

For each agent, create a Task with description like "Sub Agent 1 - Dashboard Iteration 1" and prompt:
```
You are Sub Agent [X] generating iteration [NUMBER]. 

TASK: Generate dashboard_iteration_[NUMBER].html in test-outputs/original-pattern/

SPECIFICATION: [Include full spec content]

UNIQUE DIRECTION: [Assign specific creative direction]
- Sub Agent 1: Minimalist card-based design
- Sub Agent 2: Gauge and dial visualizations  
- Sub Agent 3: Terminal/matrix style dark theme
- Sub Agent 4: Colorful chart-heavy analytics
- Sub Agent 5: Futuristic holographic interface

REQUIREMENTS:
1. Create a complete, self-contained HTML file
2. Implement your assigned design direction
3. Include simulated real-time updates
4. Ensure mobile responsiveness
5. Make it visually distinct from other iterations

Generate the complete HTML file with embedded CSS and JavaScript.
```

**PHASE 5: INFINITE MODE ORCHESTRATION**
For infinite generation mode, orchestrate continuous parallel waves:
- Launch waves of 3-5 agents at a time
- Monitor context capacity between waves
- Continue until context limits approached

Begin execution with parallel Sub Agent deployment!