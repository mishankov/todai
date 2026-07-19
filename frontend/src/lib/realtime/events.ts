import type { ActivityEvent } from '$lib/activity/client';

const activityEventName = 'todai:activity';
const activityEvents = new EventTarget();

export function publishActivityEvent(event: ActivityEvent): void {
	activityEvents.dispatchEvent(
		new CustomEvent<ActivityEvent>(activityEventName, { detail: event })
	);
}

export function subscribeActivityEvents(listener: (event: ActivityEvent) => void): () => void {
	const handle = (event: Event) => listener((event as CustomEvent<ActivityEvent>).detail);
	activityEvents.addEventListener(activityEventName, handle);
	return () => activityEvents.removeEventListener(activityEventName, handle);
}
