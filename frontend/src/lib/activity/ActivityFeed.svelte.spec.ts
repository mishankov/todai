import { page } from 'vitest/browser';
import { describe, expect, it } from 'vitest';
import { render } from 'vitest-browser-svelte';
import ActivityFeed from './ActivityFeed.svelte';
import type { ActivityEvent } from './client';

describe('ActivityFeed', () => {
	it('shows friendly task activity with actor and source', async () => {
		render(ActivityFeed, {
			initialEvents: [
				testEvent({
					type: 'task.updated',
					actorType: 'built_in_agent',
					source: 'internal_api',
					agentRunId: 'run-id',
					payload: { schemaVersion: 1, after: { title: 'Plan the day' } }
				})
			]
		});

		await expect.element(page.getByRole('heading', { name: 'Activity' })).toBeVisible();
		await expect.element(page.getByText('Agent', { exact: true })).toBeVisible();
		await expect.element(page.getByText('“Plan the day”', { exact: true })).toBeVisible();
		await expect.element(page.getByText('Internal API', { exact: true })).toBeVisible();
		await page.getByText('Details', { exact: true }).click();
		await expect.element(page.getByText('correlation-id', { exact: true })).toBeVisible();
		await expect.element(page.getByText('run-id', { exact: true })).toBeVisible();
	});

	it('keeps events useful when their payload has no entity name', async () => {
		render(ActivityFeed, {
			initialEvents: [
				testEvent({ type: 'task.completed', payload: { schemaVersion: 1, version: 2 } })
			]
		});

		await expect.element(page.getByText('completed task', { exact: true })).toBeVisible();
		await expect.element(page.getByText('Web', { exact: true })).toBeVisible();
	});

	it('shows the task name for completion events', async () => {
		render(ActivityFeed, {
			initialEvents: [
				testEvent({
					type: 'task.completed',
					payload: { schemaVersion: 1, task: { title: 'Plan the day' }, version: 2 }
				})
			]
		});

		await expect.element(page.getByText('completed task', { exact: true })).toBeVisible();
		await expect.element(page.getByText('“Plan the day”', { exact: true })).toBeVisible();
	});

	it('shows an empty state', async () => {
		render(ActivityFeed, { initialEvents: [] });

		await expect.element(page.getByRole('heading', { name: 'No activity yet' })).toBeVisible();
		await expect.element(page.getByText('0 events', { exact: true })).toBeVisible();
	});
});

function testEvent(overrides: Partial<ActivityEvent> = {}): ActivityEvent {
	return {
		streamOffset: 1,
		id: 'event-id',
		type: 'task.created',
		occurredAt: new Date().toISOString(),
		actorType: 'user',
		actorId: 'user-id',
		source: 'web',
		aggregateType: 'task',
		aggregateId: 'task-id',
		correlationId: 'correlation-id',
		agentRunId: null,
		payload: { schemaVersion: 1, task: { title: 'Plan the day' } },
		...overrides
	};
}
