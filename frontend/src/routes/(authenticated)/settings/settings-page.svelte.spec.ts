import { page } from 'vitest/browser';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import SettingsPage from './+page.svelte';

describe('Settings page', () => {
	afterEach(() => vi.unstubAllGlobals());

	it('saves the selected timezone and agent model', async () => {
		const fetcher = vi.fn(
			async () =>
				new Response(
					JSON.stringify({
						settings: {
							timezone: 'Europe/London',
							agentModel: 'gpt-fast',
							version: 2,
							createdAt: '2026-07-18T10:00:00Z',
							updatedAt: '2026-07-18T10:01:00Z',
							lastModifiedBy: 'user-id'
						},
						availableAgentModels: ['gpt-default', 'gpt-fast']
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
						version: 1,
						createdAt: null,
						updatedAt: null,
						lastModifiedBy: ''
					},
					availableAgentModels: ['gpt-default', 'gpt-fast']
				}
			}
		} as never);

		await page.getByLabelText('Time zone').selectOptions('Europe/London');
		await page.getByLabelText('Model').selectOptions('gpt-fast');
		await page.getByRole('button', { name: 'Save changes' }).click();

		expect(fetcher).toHaveBeenCalledWith(
			'/api/settings',
			expect.objectContaining({
				body: JSON.stringify({
					timezone: 'Europe/London',
					agentModel: 'gpt-fast',
					version: 1
				})
			})
		);
		await expect.element(page.getByText('Settings saved.')).toBeVisible();
	});
});
