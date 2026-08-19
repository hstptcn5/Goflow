import { afterEach, describe, expect, it, vi } from 'vitest';
import PropertiesPanel from '../src/components/PropertiesPanel.vue';
import { api } from '../src/services/api';
import { useWorkflowStore } from '../src/stores/workflowStore';
import { credentialsForParam } from '../src/utils/inspector';
import { mountWithApp, nextFrame } from './mount';

const aiExtractDef = {
  type: 'aiExtract',
  name: 'AI Extract',
  category: 'AI & LLM',
  description: 'Extract structured data',
  params: [
    { name: 'provider', label: 'AI Provider', type: 'select', required: true, default: 'openai', options: ['openai', 'deepseek'] },
    {
      name: 'credential_id',
      label: 'AI Provider Credential',
      type: 'credential',
      required: true,
      default: '',
      credential_kinds: ['API_KEY'],
      credential_providers: ['openai', 'deepseek'],
    },
  ],
};

const credentials = [
  { id: 'openai', type: 'OpenAI', kind: 'API_KEY', provider: 'openai', name: 'OpenAI' },
  { id: 'deepseek', type: 'DeepSeek', kind: 'API_KEY', provider: 'deepseek', name: 'DeepSeek' },
  { id: 'custom-api-key', type: 'API_KEY', kind: 'API_KEY', provider: 'custom', name: 'Other service' },
];

describe('AI credential hardening', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('filters AI Extract credentials to the selected provider', () => {
    const credentialParam = aiExtractDef.params[1];
    expect(credentialsForParam(credentials, credentialParam, 'aiExtract', { provider: 'openai' }).map((cred) => cred.id)).toEqual(['openai']);
    expect(credentialsForParam(credentials, credentialParam, 'aiExtract', { provider: 'deepseek' }).map((cred) => cred.id)).toEqual(['deepseek']);
  });

  it('clears an incompatible AI Extract credential when provider changes', async () => {
    const selectedNode = {
      id: 'extract_1',
      type: 'aiExtract',
      name: 'AI Extract',
      params: { provider: 'openai', credential_id: 'openai' },
    };
    let updated = selectedNode.params;
    const { root } = await mountWithApp(PropertiesPanel, {
      props: {
        selectedNode,
        onUpdateNodeParams: (_id, params) => {
          updated = params;
          selectedNode.params = params;
        },
      },
    });
    const store = useWorkflowStore();
    store.nodeDefinitions = [aiExtractDef];
    store.credentials = credentials;
    await nextFrame();

    const credentialSelect = root.querySelector('[aria-label="AI Provider Credential"]');
    const optionValues = Array.from(credentialSelect.options).map((option) => option.value);
    expect(optionValues).toEqual(['', 'openai']);

    const providerSelect = root.querySelector('[aria-label="AI Provider"]');
    providerSelect.value = 'deepseek';
    providerSelect.dispatchEvent(new Event('change'));
    await nextFrame();

    expect(updated.provider).toBe('deepseek');
    expect(updated.credential_id).toBe('');
  });

  it('does not let AI Quick Config silently consume a generic custom API key', async () => {
    const selectedNode = { id: 'extract_1', type: 'aiExtract', name: 'AI Extract', params: { provider: 'openai', credential_id: '' } };
    const configureSpy = vi.spyOn(api, 'configureNodeParams').mockResolvedValue({});
    const { root } = await mountWithApp(PropertiesPanel, { props: { selectedNode } });
    const store = useWorkflowStore();
    store.nodeDefinitions = [aiExtractDef];
    store.credentials = [credentials[2]];
    await nextFrame();

    const prompt = root.querySelector('.ai-configurer-input');
    prompt.value = 'configure this node';
    prompt.dispatchEvent(new Event('input'));
    await nextFrame();
    root.querySelector('.ai-configurer-btn').click();
    await nextFrame();

    expect(configureSpy).not.toHaveBeenCalled();
    expect(root.textContent).toContain('No provider-tagged OpenAI or DeepSeek credential found.');
  });
});
