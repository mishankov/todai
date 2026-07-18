import { describe, expect, it, vi } from 'vitest';
import { pollActivityChanges, RealtimeRequestError } from './client';

describe('realtime client', () => {
	it('starts at the current server cursor', async () => {
		const fetcher = changeFetcher({ cursor: 42, events: [] });
		const signal = new AbortController().signal;

		await expect(pollActivityChanges(fetcher, null, signal)).resolves.toEqual({
			cursor: 42,
			events: []
		});
		expect(fetcher).toHaveBeenCalledWith('/api/activity/changes', {
			credentials: 'same-origin',
			headers: { Accept: 'application/json' },
			signal
		});
	});

	it('requests changes after the last durable cursor', async () => {
		const fetcher = changeFetcher({ cursor: 8, events: [] });
		const signal = new AbortController().signal;

		await pollActivityChanges(fetcher, 7, signal);

		expect(fetcher).toHaveBeenCalledWith('/api/activity/changes?after=7', {
			credentials: 'same-origin',
			headers: { Accept: 'application/json' },
			signal
		});
	});

	it('reports a rejected change request', async () => {
		const fetcher = vi.fn(
			async () => new Response(null, { status: 401 })
		) as unknown as typeof fetch;

		await expect(pollActivityChanges(fetcher, null, new AbortController().signal)).rejects.toEqual(
			expect.objectContaining<Partial<RealtimeRequestError>>({ status: 401 })
		);
	});
});

function changeFetcher(body: object): typeof fetch {
	return vi.fn(
		async () =>
			new Response(JSON.stringify(body), {
				status: 200,
				headers: { 'Content-Type': 'application/json' }
			})
	) as unknown as typeof fetch;
}
