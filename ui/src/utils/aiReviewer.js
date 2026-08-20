export function reviewerCredentialProvider(credential) {
  const provider = String(credential?.provider || '').trim().toLowerCase();
  if (provider === 'openai' || provider === 'deepseek') return provider;

  const type = String(credential?.type || '').trim().toLowerCase();
  if (type === 'openai' || type === 'openai_api_key') return 'openai';
  if (type === 'deepseek' || type === 'deepseek_api_key') return 'deepseek';
  return '';
}

export function isReviewerCredential(credential) {
  const provider = reviewerCredentialProvider(credential);
  if (!provider) return false;

  const kind = String(credential?.kind || '').trim().toUpperCase();
  return !kind || kind === 'API_KEY';
}
