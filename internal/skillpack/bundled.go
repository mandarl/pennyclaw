package skillpack

import (
	"os"
	"path/filepath"
)

// BundledSkill represents a skill that ships with PennyClaw.
type BundledSkill struct {
	DirName string
	Content string // SKILL.md content
}

// bundledSkills contains the curated set of skills that ship with PennyClaw.
var bundledSkills = []BundledSkill{
	{
		DirName: "morning-briefing",
		Content: `---
name: morning-briefing
description: Generate a personalized morning briefing with weather, news, and tasks
version: "1.0.0"
author: PennyClaw
---

# Morning Briefing Skill

When the user asks for a morning briefing, daily summary, or when triggered by a cron job:

1. **Weather**: Use web_search to find current weather for the user's location (check USER.md for location)
2. **Top News**: Use web_search to find top 3-5 headlines relevant to the user's interests
3. **Pending Tasks**: Check if there are any pending reminders or scheduled tasks
4. **Calendar**: If the user has mentioned upcoming events, remind them

Format the briefing in a clean, scannable format with emoji headers:
- ☀️ Weather
- 📰 Headlines
- ✅ Tasks & Reminders
- 📅 Today's Schedule

Keep it concise — aim for a 30-second read.
`,
	},
	{
		DirName: "research-assistant",
		Content: `---
name: research-assistant
description: Deep research on any topic with source citations
version: "1.0.0"
author: PennyClaw
---

# Research Assistant Skill

When the user asks you to research a topic:

1. **Clarify scope**: Ask what specific aspects they want to understand
2. **Multi-source search**: Use web_search with 3-5 different query variations
3. **Synthesize**: Combine findings into a coherent summary
4. **Cite sources**: Always include URLs for key claims
5. **Follow up**: Offer to dive deeper into specific sub-topics

Research output format:
- **Summary** (2-3 sentences)
- **Key Findings** (numbered list with citations)
- **Sources** (numbered URL list)
- **Further Reading** (optional)

For technical topics, include code examples when relevant.
For current events, note the date of information.
`,
	},
	{
		DirName: "task-manager",
		Content: `---
name: task-manager
description: Natural language task and todo management with reminders
version: "1.0.0"
author: PennyClaw
---

# Task Manager Skill

Manage tasks using the workspace file system. Store tasks in a workspace file called TASKS.md.

## Commands (natural language)
- "Add task: ..." or "Remind me to ..." → Add to TASKS.md
- "Show my tasks" or "What's on my list?" → Read and display TASKS.md
- "Done: ..." or "Complete: ..." → Mark task as done in TASKS.md
- "Clear completed" → Remove done tasks

## Task Format in TASKS.md
` + "```" + `markdown
# Tasks

## Active
- [ ] Task description (added: 2024-01-15)
- [ ] Another task (added: 2024-01-15, due: 2024-01-20)

## Completed
- [x] Finished task (completed: 2024-01-16)
` + "```" + `

## Reminders
When a task has a due date or time, use cron_add to create a reminder.
Parse natural language dates: "tomorrow", "next Monday", "in 2 hours", "Jan 15".

Use workspace_read to read TASKS.md and workspace_write to update it.
`,
	},
	{
		DirName: "email-drafter",
		Content: `---
name: email-drafter
description: Draft professional emails based on context and intent
version: "1.0.0"
author: PennyClaw
---

# Email Drafter Skill

When the user asks to draft an email:

1. **Gather context**: Who is the recipient? What's the purpose? What tone?
2. **Check USER.md**: Use the user's name and any relevant context
3. **Draft**: Write a complete email with subject line
4. **Iterate**: Offer to adjust tone, length, or content

## Tone Guide
- **Professional**: Formal greeting, clear structure, professional sign-off
- **Friendly**: Casual but respectful, conversational
- **Brief**: Minimal words, direct to the point
- **Persuasive**: Clear value proposition, call to action

## Output Format
` + "```" + `
Subject: [subject line]

[email body]

[sign-off]
[user's name]
` + "```" + `

Always ask before sending if an email integration is available.
`,
	},
	{
		DirName: "code-reviewer",
		Content: `---
name: code-reviewer
description: Review code for bugs, security issues, and best practices
version: "1.0.0"
author: PennyClaw
---

# Code Reviewer Skill

When the user shares code or asks for a code review:

1. **Read the code**: Use read_file if it's a file path, or analyze inline code
2. **Check for issues** in this priority order:
   - 🔴 **Security**: SQL injection, XSS, path traversal, hardcoded secrets, SSRF
   - 🟠 **Bugs**: Logic errors, off-by-one, null/nil handling, race conditions
   - 🟡 **Performance**: N+1 queries, unnecessary allocations, missing indexes
   - 🔵 **Style**: Naming, structure, idiomatic patterns, documentation
3. **Provide fixes**: Show corrected code for each issue found
4. **Summary**: Rate the code (needs work / acceptable / good / excellent)

## Output Format
For each issue found:
- **Severity**: 🔴/🟠/🟡/🔵
- **Line(s)**: Where the issue is
- **Problem**: What's wrong
- **Fix**: Corrected code

End with a summary and overall assessment.
`,
	},
	{
		DirName: "summarizer",
		Content: `---
name: summarizer
description: Summarize articles, documents, and web pages
version: "1.0.0"
author: PennyClaw
---

# Summarizer Skill

When the user asks to summarize content:

1. **Fetch content**: Use http_request for URLs, read_file for local files
2. **Identify type**: Article, documentation, research paper, email thread, etc.
3. **Generate summary** at the requested depth:
   - **TL;DR**: 1-2 sentences
   - **Brief**: 1 paragraph (default)
   - **Detailed**: Multiple paragraphs with key points
   - **Bullet points**: Structured list of main ideas

## Guidelines
- Preserve the author's key arguments and conclusions
- Note any data, statistics, or specific claims
- Flag if the content seems biased or one-sided
- For technical content, preserve important details and terminology
- For long content, offer to summarize in sections
`,
	},
}

// seedBundledSkills writes the bundled skills to the skills directory.
func (l *Loader) seedBundledSkills() {
	for _, bs := range bundledSkills {
		dir := filepath.Join(l.skillsDir, bs.DirName)
		if err := os.MkdirAll(dir, 0755); err != nil {
			continue
		}

		skillPath := filepath.Join(dir, "SKILL.md")
		// Don't overwrite if already exists
		if _, err := os.Stat(skillPath); err == nil {
			continue
		}

		os.WriteFile(skillPath, []byte(bs.Content), 0644)
	}
}
