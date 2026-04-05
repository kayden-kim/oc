/**
 * oc-skills plugin for OpenCode
 *
 * Walks up the directory tree from the current working directory,
 * discovering skill folders (.agents/skills, .opencode/skills) at each level,
 * and registers their paths so OpenCode can load them.
 */

import path from "path";
import fs from "fs";
import type { Plugin } from "@opencode-ai/plugin";

const SKILL_DIRS = [".agents/skills", ".opencode/skills"] as const;

/**
 * Walk from `startDir` up to the filesystem root, collecting every
 * existing skills directory along the way.
 *
 * Returns paths ordered from deepest (closest to cwd) to shallowest (root).
 */
function discoverSkillPaths(startDir: string): string[] {
  const found: string[] = [];
  let current = path.resolve(startDir);

  // eslint-disable-next-line no-constant-condition
  while (true) {
    for (const rel of SKILL_DIRS) {
      const candidate = path.join(current, rel);
      if (fs.existsSync(candidate) && fs.statSync(candidate).isDirectory()) {
        found.push(candidate);
      }
    }

    const parent = path.dirname(current);
    if (parent === current) break; // reached filesystem root
    current = parent;
  }

  return found;
}

export const OcSkillsPlugin: Plugin = async ({ directory }) => {
  const skillPaths = discoverSkillPaths(directory);

  return {
    config: async (config) => {
      config.skills = config.skills || {};
      config.skills.paths = config.skills.paths || [];

      for (const p of skillPaths) {
        if (!config.skills.paths.includes(p)) {
          config.skills.paths.push(p);
        }
      }
    },
  };
};

export default { server: OcSkillsPlugin } satisfies {
  server: Plugin;
};
