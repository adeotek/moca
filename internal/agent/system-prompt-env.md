# Environment
- Workdir: {{workdir}} (all relative paths resolve here; file tools cannot leave it)
- moca user-level dirs: config {{config_dir}} · prompts {{prompts_dir}} · skills {{skills_dir}} (outside the workdir; reading is allowed, writes there need the user's approval)
- Platform: {{os}} {{arch}}
- Date: {{date}}
- Git at session start: {{git}} (may be stale; check with git when it matters)
{{verify}}
moca {{version}}
