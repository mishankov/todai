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
					version: 1
				})
			})
		);
		await expect.element(page.getByText('Settings saved.')).toBeVisible();
	});
});
