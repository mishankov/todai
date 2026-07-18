import type { ActivityEvent } from '$lib/activity/client';

export interface ActivityChanges {
	cursor: number;
	events: ActivityEvent[];
}

export class RealtimeRequestError extends Error {
	constructor(
		message: string,
		readonly status: number
	) {
		super(message);
		this.name = 'RealtimeRequestError';
	}
}

export async function pollActivityChanges(
	fetcher: typeof fetch,
	after: number | null,
	signal: AbortSignal
): Promise<ActivityChanges> {
	const query = after === null ? '' : `?${new URLSearchParams({ after: String(after) })}`;
	const response = await fetcher(`/api/activity/changes${query}`, {
		credentials: 'same-origin',
		headers: { Accept: 'application/json' },
		signal
	});
	if (!response.ok) {
		throw new RealtimeRequestError('Could not connect to live updates.', response.status);
	}
	return (await response.json()) as ActivityChanges;
}
