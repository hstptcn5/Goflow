import { beforeEach, describe, expect, it } from 'vitest';
import {
  AI_WELCOME_MESSAGE,
  clearAIChatHistory,
  loadAIChatHistory,
  saveAIChatHistory,
} from '../src/utils/aiChatHistory';

beforeEach(() => {
  window.localStorage.clear();
});

describe('AI chat history', () => {
  it('persists messages per workflow and restores the welcome message', () => {
    saveAIChatHistory('wf-a', [
      AI_WELCOME_MESSAGE,
      { id: 'u1', sender: 'user', type: 'text', text: 'Tạo workflow báo cáo' },
      { id: 'a1', sender: 'ai', type: 'text', text: 'Đã hiểu.' },
    ]);

    const restored = loadAIChatHistory('wf-a');
    expect(restored[0]).toMatchObject({ id: 'welcome' });
    expect(restored.map((item) => item.id)).toEqual(['welcome', 'u1', 'a1']);
    expect(loadAIChatHistory('wf-b')).toEqual([AI_WELCOME_MESSAGE]);
  });

  it('clears only the selected workflow history', () => {
    saveAIChatHistory('wf-a', [{ id: 'a', sender: 'user', type: 'text', text: 'A' }]);
    saveAIChatHistory('wf-b', [{ id: 'b', sender: 'user', type: 'text', text: 'B' }]);

    clearAIChatHistory('wf-a');

    expect(loadAIChatHistory('wf-a')).toEqual([AI_WELCOME_MESSAGE]);
    expect(loadAIChatHistory('wf-b').map((item) => item.id)).toEqual(['welcome', 'b']);
  });
});
