#!/usr/bin/env node
/**
 * Purpose: Trigger bundled skill installation during npm package postinstall.
 * Why: Makes fresh npm installs immediately usable in supported agent environments.
 * Docs: docs/features/feature/enhanced-cli-config/index.md
 *
 * Postinstall hook for bundled Kaboom skills.
 * Never fails package installation.
 */

const { installBundledSkills } = require('./skills');

async function run() {
  try {
    const verbose =
      String(process.env.KABOOM_SKILL_VERBOSE || '')
        .trim()
        .toLowerCase() === '1' ||
      String(process.env.KABOOM_SKILL_VERBOSE || '')
        .trim()
        .toLowerCase() === 'true';

    const result = await installBundledSkills({ verbose });

    if (result.skipped) {
      if (verbose) {
        console.log(`[kaboom-mcp] skill install skipped: ${result.reason}`);
      }
      return;
    }

    const s = result.summary;
    console.log(
      `[kaboom-mcp] skills installed (${result.agents.join(', ')} / ${result.scope}): ` +
        `source=${result.source} created=${s.created} updated=${s.updated} unchanged=${s.unchanged} ` +
        `skipped=${s.skipped_user_owned} errors=${s.errors}`
    );
    for (const warning of result.warnings || []) {
      console.warn(`[kaboom-mcp] warning: ${warning}`);
    }
  } catch (err) {
    console.warn(`[kaboom-mcp] warning: skill postinstall failed: ${err.message}`);
  }
}

run();
