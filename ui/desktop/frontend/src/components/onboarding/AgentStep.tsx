import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { getApiClient } from '../../lib/api'
import { slugify, isValidSlug } from '../../lib/slug'
import { agentService } from '../../services/agent-service'

import { SummoningModal } from './SummoningModal'
import type { ProviderData } from '../../types/provider'

// Preset keys — labels & prompts from agents.json (web) or desktop.json
// agentKey: stable English slug used as agent_key (never translated)
const PRESET_KEYS = [
  { key: 'foxSpirit', emoji: '🦊', ns: 'agents', agentKey: 'little-fox' },
  { key: 'artisan', emoji: '🎨', ns: 'agents', agentKey: 'artisan' },
  { key: 'astrologer', emoji: '🔮', ns: 'agents', agentKey: 'mimi' },
  { key: 'researcher', emoji: '🔬', ns: 'desktop', agentKey: 'scholar' },
  { key: 'writer', emoji: '✍️', ns: 'desktop', agentKey: 'quill' },
  { key: 'coder', emoji: '👨‍💻', ns: 'desktop', agentKey: 'dev' },
]

interface AgentStepProps {
  provider: ProviderData
  model: string | null
  onBack: () => void
  onComplete: () => void
}

export function AgentStep({ provider, model, onBack, onComplete }: AgentStepProps) {
  const { t } = useTranslation(['desktop', 'agents', 'common'])
  const [selectedPresetIdx, setSelectedPresetIdx] = useState<number>(0)
  const [description, setDescription] = useState('')
  // Identity is editable state, only *prefilled* from the selected preset. It must
  // survive description edits — deselecting a preset never rewrites the name/key.
  const [displayName, setDisplayName] = useState('')
  const [agentKey, setAgentKey] = useState('')
  const [displayNameEdited, setDisplayNameEdited] = useState(false)
  const [agentKeyEdited, setAgentKeyEdited] = useState(false)
  const [emoji, setEmoji] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [createdAgent, setCreatedAgent] = useState<{ id: string; name: string } | null>(null)

  // Get prompt text from locale for the selected preset
  function getPresetPrompt(idx: number): string {
    const preset = PRESET_KEYS[idx]
    if (!preset) return ''
    return t(`${preset.ns}:presets.${preset.key}.prompt`)
  }

  // Label format: "🦊 Fox Spirit" — the display name drops the emoji prefix
  function getPresetName(idx: number): string {
    const preset = PRESET_KEYS[idx]
    if (!preset) return ''
    const label = t(`${preset.ns}:presets.${preset.key}.label`) as string
    return label.replace(/^\S+\s+/, '').trim() || label.trim()
  }

  // Init key + emoji from the first preset; name and prompt are filled by the
  // locale-sync effects below.
  useEffect(() => {
    setAgentKey(PRESET_KEYS[0].agentKey)
    setEmoji(PRESET_KEYS[0].emoji)
  }, [])

  const presetPrompt = selectedPresetIdx >= 0 ? getPresetPrompt(selectedPresetIdx) : undefined
  const presetName = selectedPresetIdx >= 0 ? getPresetName(selectedPresetIdx) : undefined

  // Language-aware re-sync: while a preset is active and the field is untouched,
  // follow the translated preset text. Deps are plain strings, so this only fires
  // when the translation actually changes (locale switch), not on every render.
  useEffect(() => {
    if (presetPrompt === undefined) return
    setDescription(presetPrompt)
  }, [presetPrompt])

  useEffect(() => {
    if (presetName === undefined || displayNameEdited) return
    setDisplayName(presetName)
  }, [presetName, displayNameEdited])

  const trimmedName = displayName.trim()
  const keyValid = isValidSlug(agentKey)
  const canSubmit = !!trimmedName && keyValid && !!description.trim()

  const handleSelectPreset = (idx: number) => {
    const preset = PRESET_KEYS[idx]
    if (!preset) return
    setSelectedPresetIdx(idx)
    setDescription(getPresetPrompt(idx))
    setDisplayName(getPresetName(idx))
    setDisplayNameEdited(false)
    setEmoji(preset.emoji)
    setAgentKey(preset.agentKey)
    setAgentKeyEdited(false)
  }

  const handleDescriptionChange = (value: string) => {
    setDescription(value)
    // Editing the prompt only clears the preset highlight — identity stays as typed.
    if (selectedPresetIdx >= 0 && value !== presetPrompt) setSelectedPresetIdx(-1)
  }

  const handleDisplayNameChange = (value: string) => {
    setDisplayNameEdited(true)
    setDisplayName(value)
    if (!agentKeyEdited) setAgentKey(slugify(value))
  }

  const handleAgentKeyChange = (value: string) => {
    setAgentKeyEdited(true)
    setAgentKey(value)
  }

  const handleSubmit = async () => {
    if (!canSubmit) return
    setLoading(true)
    setError('')
    try {
      const result = await getApiClient().post<{ id: string }>('/v1/agents', {
        agent_key: agentKey,
        display_name: trimmedName,
        provider: provider.name,
        model: model || '',
        agent_type: 'predefined',
        is_default: true,
        // Promoted fields at top level — agent_description triggers summoning on backend
        agent_description: description.trim() || null,
        emoji: emoji.trim() || null,
      })
      setCreatedAgent({ id: result.id, name: trimmedName || agentKey })
    } catch (err) {
      setError(err instanceof Error ? err.message : t('common:failedToCreateAgent'))
    } finally {
      setLoading(false)
    }
  }

  const providerLabel = provider.display_name || provider.name

  if (createdAgent) {
    return (
      <SummoningModal
        agentId={createdAgent.id}
        agentName={createdAgent.name}
        onContinue={onComplete}
        onCancel={(id) => agentService.cancelSummon(id)}
      />
    )
  }

  return (
    <div className="bg-surface-secondary border border-border rounded-xl p-6 space-y-4">
      <div>
        <h2 className="text-lg font-semibold text-text-primary">{t('onboarding.agentStep')}</h2>
        <p className="text-sm text-text-muted">{t('onboarding.agentStepDesc')}</p>
      </div>

      {/* Provider + model info */}
      <div className="flex flex-wrap items-center gap-x-4 gap-y-1">
        <div className="flex items-center gap-2">
          <span className="text-sm text-text-muted">{t('common:provider')}</span>
          <span className="text-xs font-medium px-2 py-0.5 rounded-md bg-surface-tertiary border border-border text-text-secondary">
            {providerLabel}
          </span>
        </div>
        {model && (
          <div className="flex items-center gap-2">
            <span className="text-sm text-text-muted">{t('common:model')}</span>
            <span className="text-xs font-mono px-2 py-0.5 rounded-md border border-border text-text-secondary">
              {model}
            </span>
          </div>
        )}
      </div>

      {/* Agent identity — editable name + key, prefilled from the preset */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
        <div className="space-y-1.5">
          <label htmlFor="onboard-agent-name" className="block text-sm font-medium text-text-secondary">
            {t('agents:create.displayName')}
          </label>
          <div className="flex gap-2">
            <input
              id="onboard-agent-emoji"
              value={emoji}
              onChange={(e) => setEmoji(e.target.value)}
              maxLength={2}
              placeholder="🤖"
              title={t('agents:create.emojiHint')}
              className="w-14 shrink-0 text-center w-full bg-surface-tertiary border border-border rounded-lg px-3 py-2.5 text-base md:text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:ring-1 focus:ring-accent"
            />
            <input
              id="onboard-agent-name"
              value={displayName}
              onChange={(e) => handleDisplayNameChange(e.target.value)}
              placeholder={t('agents:create.displayNamePlaceholder')}
              className="w-full bg-surface-tertiary border border-border rounded-lg px-3 py-2.5 text-base md:text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:ring-1 focus:ring-accent"
            />
          </div>
        </div>

        <div className="space-y-1.5">
          <label htmlFor="onboard-agent-key" className="block text-sm font-medium text-text-secondary">
            {t('agents:create.agentKey')}
          </label>
          <input
            id="onboard-agent-key"
            value={agentKey}
            onChange={(e) => handleAgentKeyChange(e.target.value)}
            onBlur={(e) => setAgentKey(slugify(e.target.value))}
            placeholder={t('agents:create.agentKeyPlaceholder')}
            className="w-full bg-surface-tertiary border border-border rounded-lg px-3 py-2.5 text-base md:text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:ring-1 focus:ring-accent"
          />
          <p className={agentKey && !keyValid ? 'text-xs text-error' : 'text-xs text-text-muted'}>
            {t('agents:create.agentKeyHint')}
          </p>
        </div>
      </div>

      {/* Preset personality buttons */}
      <div className="space-y-2">
        <label className="block text-sm font-medium text-text-secondary">{t('agents:detail.personality')}</label>
        <div className="flex flex-wrap gap-1.5">
          {PRESET_KEYS.map((preset, idx) => (
            <button
              key={preset.key}
              type="button"
              onClick={() => handleSelectPreset(idx)}
              className={[
                'cursor-pointer rounded-full border px-3 py-1 text-xs transition-colors',
                selectedPresetIdx === idx
                  ? 'border-accent bg-accent/10 text-accent font-medium'
                  : 'border-border text-text-secondary hover:bg-surface-tertiary',
              ].join(' ')}
            >
              {t(`${preset.ns}:presets.${preset.key}.label`)}
            </button>
          ))}
        </div>
      </div>

      {/* Description textarea */}
      <div className="space-y-1.5">
        <label className="block text-sm font-medium text-text-secondary">{t('common:description')}</label>
        <textarea
          value={description}
          onChange={(e) => handleDescriptionChange(e.target.value)}
          placeholder={t('onboarding.descPlaceholder')}
          rows={4}
          className="w-full bg-surface-tertiary border border-border rounded-lg px-3 py-2.5 text-base md:text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:ring-1 focus:ring-accent resize-none"
        />
      </div>

      {error && <p className="text-sm text-error">{error}</p>}

      <div className="flex justify-between gap-2">
        <button
          onClick={onBack}
          className="px-4 py-2.5 border border-border rounded-lg text-sm font-medium text-text-secondary hover:bg-surface-tertiary transition-colors"
        >
          &larr; {t('common:back')}
        </button>
        <button
          onClick={handleSubmit}
          disabled={loading || !canSubmit}
          className="px-6 py-2.5 bg-accent text-white rounded-lg font-medium hover:bg-accent-hover transition-colors disabled:opacity-40 disabled:cursor-not-allowed flex items-center gap-2"
        >
          {loading && <div className="w-3.5 h-3.5 border-2 border-white border-t-transparent rounded-full animate-spin" />}
          {t('desktop:agent.summon')}
        </button>
      </div>
    </div>
  )
}
