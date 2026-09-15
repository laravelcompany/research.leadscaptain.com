package ai

import _ "embed"

// PlannerPrompt is the shared instruction block for the agent's planning
// step, kept in prompts/planner.txt so it can be edited without a rebuild of
// the logic that renders the rest of the prompt.
//
//go:embed prompts/planner.txt
var PlannerPrompt string
