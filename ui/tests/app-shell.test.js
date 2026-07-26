import { describe, expect, it } from 'vitest';
import AppShell from '../src/components/AppShell.vue';
import { mountWithApp } from './mount';

describe('AppShell navigation', () => {
  it('renders primary navigation and marks the active route', async () => {
    const { root } = await mountWithApp(AppShell, { route: '/credentials' });

    const links = Array.from(root.querySelectorAll('.rail-link')).map((item) => item.textContent.trim());
    expect(links).toEqual(['Workflows', 'Executions', 'Credentials', 'Templates', 'Nodes', 'Settings', 'Help']);

    const active = root.querySelector('.rail-link[aria-current="page"]');
    expect(active?.textContent.trim()).toBe('Credentials');
  });
});
