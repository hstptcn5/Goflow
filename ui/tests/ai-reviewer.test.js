import { describe, expect, it } from 'vitest';
import { isReviewerCredential, reviewerCredentialProvider } from '../src/utils/aiReviewer';

describe('AI workflow reviewer credential selection', () => {
  it('accepts canonical OpenAI and DeepSeek API key credentials', () => {
    const openai = { type: 'API_KEY', kind: 'API_KEY', provider: 'openai' };
    const deepseek = { type: 'API_KEY', kind: 'API_KEY', provider: 'deepseek' };

    expect(isReviewerCredential(openai)).toBe(true);
    expect(isReviewerCredential(deepseek)).toBe(true);
    expect(reviewerCredentialProvider(openai)).toBe('openai');
    expect(reviewerCredentialProvider(deepseek)).toBe('deepseek');
  });

  it('keeps explicit legacy OpenAI and DeepSeek credentials compatible', () => {
    expect(isReviewerCredential({ type: 'OpenAI' })).toBe(true);
    expect(isReviewerCredential({ type: 'DeepSeek' })).toBe(true);
    expect(reviewerCredentialProvider({ type: 'openai_api_key' })).toBe('openai');
    expect(reviewerCredentialProvider({ type: 'deepseek_api_key' })).toBe('deepseek');
  });

  it('rejects generic/custom API keys and non-API-key credentials', () => {
    expect(isReviewerCredential({ type: 'API_KEY', kind: 'API_KEY', provider: 'custom' })).toBe(false);
    expect(isReviewerCredential({ type: 'API_KEY', kind: 'BEARER_TOKEN', provider: 'openai' })).toBe(false);
    expect(isReviewerCredential({ type: 'TELEGRAM_BOT', kind: 'API_KEY', provider: 'telegram' })).toBe(false);
  });
});
