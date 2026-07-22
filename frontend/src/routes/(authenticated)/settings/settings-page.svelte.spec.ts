import { page } from 'vitest/browser';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import SettingsPage from './+page.svelte';

describe('Settings page', () => {
	afterEach(() => vi.unstubAllGlobals());

	it('saves the selected timezone without exposing account-level agent controls', async () => {
		const fetcher = vi.fn(
			async () =>
				new Response(
					JSON.stringify({
						settings: {
							timezone: 'Europe/London',
							agentModel: 'gpt-fast',
							agentThinkingEffort: 'high',
							appearance: 'dark',
							version: 2,
							createdAt: '2026-07-18T10:00:00Z',
							updatedAt: '2026-07-18T10:01:00Z',
							lastModifiedBy: 'user-id'
						},
						availableAgentModels: ['gpt-default', 'gpt-fast'],
						availableAgentThinkingEfforts: ['off', 'low', 'medium', 'high']
					}),
					{ status: 200, headers: { 'Content-Type': 'application/json' } }
				)
		);
		vi.stubGlobal('fetch', fetcher);
		render(SettingsPage, {
			data: {
				settings: {
					settings: {
						timezone: 'UTC',
						agentModel: 'gpt-default',
						agentThinkingEffort: 'medium',
						appearance: 'system',
						version: 1,
						createdAt: null,
						updatedAt: null,
						lastModifiedBy: ''
					},
					availableAgentModels: ['gpt-default', 'gpt-fast'],
					availableAgentThinkingEfforts: ['off', 'low', 'medium', 'high']
				}
			}
		} as never);

		await page.getByLabelText('Time zone').selectOptions('Europe/London');
		await page.getByRole('radio', { name: /Dark/ }).click();
		await expect.element(page.getByLabelText('Model')).not.toBeInTheDocument();
		await expect.element(page.getByLabelText('Thinking effort')).not.toBeInTheDocument();
		await page.getByRole('button', { name: 'Save changes' }).click();

		expect(fetcher).toHaveBeenCalledWith(
			'/api/settings',
			expect.objectContaining({
				body: JSON.stringify({
					timezone: 'Europe/London',
					agentModel: 'gpt-default',
					agentThinkingEffort: 'medium',
					appearance: 'dark',
					version: 1
				})
			})
		);
		await expect.element(page.getByText('Settings saved.')).toBeVisible();
	});

	it('keeps an unsaved appearance draft when the server reports a conflict', async () => {
		const fetcher = vi.fn(async () => new Response(null, { status: 409 }));
		vi.stubGlobal('fetch', fetcher);
		document.documentElement.dataset.appearance = 'light';
		render(SettingsPage, {
			data: {
				settings: {
					settings: {
						timezone: 'UTC',
						agentModel: 'gpt-default',
						agentThinkingEffort: 'medium',
						appearance: 'light',
						version: 1,
						createdAt: null,
						updatedAt: null,
						lastModifiedBy: ''
					},
					availableAgentModels: ['gpt-default'],
					availableAgentThinkingEfforts: ['medium']
				}
			}
		} as never);

		const dark = page.getByRole('radio', { name: /Dark/ });
		await dark.click();
		await page.getByRole('button', { name: 'Save changes' }).click();

		await expect.element(page.getByRole('alert')).toHaveTextContent('Settings changed');
		await expect.element(dark).toBeChecked();
		expect(document.documentElement.dataset.appearance).toBe('light');
	});
});
