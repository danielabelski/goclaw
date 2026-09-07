import { useTranslation } from "react-i18next";

export interface AgentPreset {
  /** Chip label as translated, may carry a leading emoji (e.g. "🦊 Fox Spirit"). */
  label: string;
  /** Translated display name with the leading emoji stripped. */
  name: string;
  prompt: string;
  emoji: string;
  /** Stable English slug used as agent_key — never translated. */
  agentKey: string;
}

/** Labels are stored as "<emoji> <name>"; the emoji lives in its own field. */
function stripLeadingEmoji(label: string): string {
  return label.replace(/^\S+\s+/, "").trim() || label.trim();
}

export function useAgentPresets(): AgentPreset[] {
  const { t } = useTranslation("agents");

  const build = (key: string, emoji: string, agentKey: string): AgentPreset => {
    const label = t(`presets.${key}.label`);
    return {
      label,
      name: stripLeadingEmoji(label),
      prompt: t(`presets.${key}.prompt`),
      emoji,
      agentKey,
    };
  };

  return [
    build("foxSpirit", "🦊", "little-fox"),
    build("coder", "💻", "dev"),
    build("support", "🎧", "support"),
    build("writer", "✍️", "quill"),
    build("translator", "🌐", "translator"),
    build("artisan", "🎨", "artisan"),
    build("astrologer", "🔮", "mimi"),
  ];
}
