export type ActivityActorType = 'user' | 'built_in_agent' | 'external_agent' | 'system';
export type ActivitySource = 'web' | 'internal_api' | 'system';

export interface ActivityEvent {
	streamOffset: number;
	id: string;
	type: string;
	occurredAt: string;
	actorType: ActivityActorType;
	actorId: string | null;
	source: ActivitySource;
	aggregateType: string | null;
	aggregateId: string | null;
	correlationId: string;
	agentRunId: string | null;
	payload: Record<string, unknown>;
}

export class ActivityRequestError extends Error {
	constructor() {
		super('Could not load activity.');
		this.name = 'ActivityRequestError';
	}
}

export async function getActivity(fetcher: typeof fetch, limit = 100): Promise<ActivityEvent[]> {
	const query = new URLSearchParams({ limit: String(limit) });
	const response = await fetcher(`/api/activity/?${query}`, {
		credentials: 'same-origin',
		headers: { Accept: 'application/json' }
	});
	if (!response.ok) throw new ActivityRequestError();

	const body = (await response.json()) as { events: ActivityEvent[] };
	return body.events;
}
