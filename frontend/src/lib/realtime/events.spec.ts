import { describe, expect, it, vi } from 'vitest';
import type { ActivityEvent } from '$lib/activity/client';
import { publishActivityEvent, subscribeActivityEvents } from './events';

describe('activity event notifications', () => {
	it('delivers events to subscribers until they unsubscribe', () => {
		const listener = vi.fn();
		const unsubscribe = subscribeActivityEvents(listener);
		const event = testEvent();

		publishActivityEvent(event);
		unsubscribe();
		publishActivityEvent({ ...event, id: 'ignored-event' });

		expect(listener).toHaveBeenCalledOnce();
		expect(listener).toHaveBeenCalledWith(event);
	});
});

function testEvent(): ActivityEvent {
	return {
		streamOffset: 1,
		id: 'event-id',
		type: 'task.comment.created',
		occurredAt: '2026-07-19T10:00:00Z',
		actorType: 'user',
		actorId: 'user-id',
		source: 'web',
		aggregateType: 'task_comment',
		aggregateId: 'comment-id',
		correlationId: 'correlation-id',
		agentRunId: null,
		payload: { taskId: 'task-id' }
	};
}
