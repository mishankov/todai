<script lang="ts">
	import type { ActivityActorType, ActivityEvent, ActivitySource } from './client';

	interface Props {
		initialEvents: ActivityEvent[];
	}

	interface ActivityGroup {
		key: string;
		label: string;
		events: ActivityEvent[];
	}

	let { initialEvents: events }: Props = $props();
	let groups = $derived(groupEvents(events));

	const eventLabels: Record<string, string> = {
		'task.created': 'Created task',
		'task.updated': 'Updated task',
		'task.moved': 'Moved task',
		'task.completed': 'Completed task',
		'task.reopened': 'Reopened task',
		'task.reordered': 'Reordered task',
		'task.deleted': 'Deleted task',
		'task.subtask.created': 'Created subtask',
		'task.comment.created': 'Added comment',
		'task.comment.updated': 'Updated comment',
		'task.comment.deleted': 'Deleted comment',
		'project.created': 'Created project',
		'project.updated': 'Updated project',
		'project.archived': 'Archived project',
		'section.created': 'Created section',
		'section.updated': 'Updated section',
		'section.reordered': 'Reordered section',
		'section.deleted': 'Deleted section',
		'user_settings.updated': 'Updated settings'
	};

	function groupEvents(items: ActivityEvent[]): ActivityGroup[] {
		const grouped: ActivityGroup[] = [];
		for (const event of items) {
			const date = new Date(event.occurredAt);
			const key = Number.isNaN(date.getTime()) ? 'unknown' : localDateKey(date);
			const existing = grouped.find((group) => group.key === key);
			if (existing) {
				existing.events.push(event);
				continue;
			}
			grouped.push({ key, label: formatDay(date), events: [event] });
		}
		return grouped;
	}

	function localDateKey(date: Date): string {
		return `${date.getFullYear()}-${date.getMonth()}-${date.getDate()}`;
	}

	function formatDay(date: Date): string {
		if (Number.isNaN(date.getTime())) return 'Unknown date';
		const today = new Date();
		const yesterday = new Date(today.getTime() - 24 * 60 * 60 * 1000);
		if (localDateKey(date) === localDateKey(today)) return 'Today';
		if (localDateKey(date) === localDateKey(yesterday)) return 'Yesterday';
		return new Intl.DateTimeFormat(undefined, {
			weekday: 'long',
			month: 'long',
			day: 'numeric',
			year: date.getFullYear() === today.getFullYear() ? undefined : 'numeric'
		}).format(date);
	}

	function formatTime(value: string): string {
		const date = new Date(value);
		if (Number.isNaN(date.getTime())) return 'Unknown time';
		return new Intl.DateTimeFormat(undefined, { hour: '2-digit', minute: '2-digit' }).format(date);
	}

	function eventLabel(type: string): string {
		return eventLabels[type] ?? humanizeType(type);
	}

	function humanizeType(type: string): string {
		const value = type.split('.').at(-1)?.replaceAll('_', ' ') ?? type;
		return value.charAt(0).toUpperCase() + value.slice(1);
	}

	function entityName(event: ActivityEvent): string | null {
		const payload = event.payload;
		for (const key of ['task', 'after', 'before']) {
			const nested = asRecord(payload[key]);
			if (typeof nested?.title === 'string' && nested.title.trim() !== '') return nested.title;
		}
		if (typeof payload.name === 'string' && payload.name.trim() !== '') return payload.name;
		if (typeof payload.title === 'string' && payload.title.trim() !== '') return payload.title;
		return null;
	}

	function asRecord(value: unknown): Record<string, unknown> | null {
		if (typeof value !== 'object' || value === null || Array.isArray(value)) return null;
		return value as Record<string, unknown>;
	}

	function actorLabel(actorType: ActivityActorType): string {
		switch (actorType) {
			case 'user':
				return 'You';
			case 'built_in_agent':
				return 'Agent';
			case 'external_agent':
				return 'External agent';
			case 'system':
				return 'System';
		}
	}

	function sourceLabel(source: ActivitySource): string {
		switch (source) {
			case 'web':
				return 'Web';
			case 'internal_api':
				return 'Internal API';
			case 'system':
				return 'System';
		}
	}

	function eventKind(event: ActivityEvent): string {
		return event.aggregateType === 'user_settings' || event.type.startsWith('user_settings.')
			? 'settings'
			: event.aggregateType === 'project' || event.type.startsWith('project.')
				? 'project'
				: event.aggregateType === 'section' || event.type.startsWith('section.')
					? 'section'
					: event.actorType === 'built_in_agent'
						? 'agent'
						: 'task';
	}
</script>

<section class="activity-page" aria-labelledby="activity-heading">
	<header class="page-header">
		<div>
			<p class="eyebrow">History</p>
			<h1 id="activity-heading">Activity</h1>
			<p class="intro">A record of changes made by you, your agents, and the system.</p>
		</div>
		<span class="event-count">{events.length} {events.length === 1 ? 'event' : 'events'}</span>
	</header>

	{#if groups.length === 0}
		<div class="empty-state">
			<div class="empty-icon" aria-hidden="true">
				<svg viewBox="0 0 24 24">
					<path d="M12 7v5l3 2M21 12a9 9 0 1 1-3-6.7" />
					<path d="M21 4v5h-5" />
				</svg>
			</div>
			<h2>No activity yet</h2>
			<p>Changes to tasks, projects, and sections will appear here.</p>
		</div>
	{:else}
		<div class="timeline">
			{#each groups as group (group.key)}
				<section class="day-group" aria-labelledby={`activity-${group.key}`}>
					<h2 id={`activity-${group.key}`}>{group.label}</h2>
					<ol>
						{#each group.events as event (event.id)}
							<li>
								<span class={`event-icon ${eventKind(event)}`} aria-hidden="true">
									{#if eventKind(event) === 'settings'}
										<svg viewBox="0 0 24 24"
											><circle cx="12" cy="12" r="3" /><path
												d="M12 3v3M12 18v3M3 12h3M18 12h3M5.6 5.6l2.1 2.1M16.3 16.3l2.1 2.1M18.4 5.6l-2.1 2.1M7.7 16.3l-2.1 2.1"
											/></svg
										>
									{:else if eventKind(event) === 'project'}
										<svg viewBox="0 0 24 24"><path d="M4 6h6l2 2h8v10H4z" /></svg>
									{:else if eventKind(event) === 'section'}
										<svg viewBox="0 0 24 24"><path d="M6 5h12M6 12h12M6 19h12" /></svg>
									{:else if eventKind(event) === 'agent'}
										<svg viewBox="0 0 24 24"
											><path
												d="M12 3v3M5.6 5.6l2.1 2.1M18.4 5.6l-2.1 2.1M4 13h16v7H4zM8 16h.01M16 16h.01"
											/></svg
										>
									{:else}
										<svg viewBox="0 0 24 24"><path d="m5 12 4 4L19 6" /></svg>
									{/if}
								</span>
								<div class="event-content">
									<div class="event-summary">
										<p>
											<span class="actor">{actorLabel(event.actorType)}</span>
											<span>{eventLabel(event.type).toLowerCase()}</span>
											{#if entityName(event)}
												<strong>“{entityName(event)}”</strong>
											{/if}
										</p>
										<time datetime={event.occurredAt}>{formatTime(event.occurredAt)}</time>
									</div>
									<div class="event-meta">
										<span>{sourceLabel(event.source)}</span>
										<span>{event.type}</span>
										<details>
											<summary>Details</summary>
											<dl>
												<dt>Correlation</dt>
												<dd>{event.correlationId}</dd>
												{#if event.agentRunId}
													<dt>Agent run</dt>
													<dd>{event.agentRunId}</dd>
												{/if}
											</dl>
										</details>
									</div>
								</div>
							</li>
						{/each}
					</ol>
				</section>
			{/each}
		</div>
	{/if}
</section>

<style>
	.activity-page {
		width: min(100%, 56rem);
		margin: 0 auto;
	}

	.page-header {
		display: flex;
		align-items: flex-end;
		justify-content: space-between;
		gap: 2rem;
		margin-bottom: 3.25rem;
	}

	.eyebrow {
		margin: 0 0 0.55rem;
		color: var(--theme-accent);
		font-size: 0.72rem;
		font-weight: 800;
		letter-spacing: 0.13em;
		text-transform: uppercase;
	}

	h1 {
		margin: 0;
		color: var(--color-text);
		font-size: clamp(2.3rem, 7vw, 4rem);
		letter-spacing: -0.055em;
		line-height: 0.98;
	}

	.intro {
		max-width: 34rem;
		margin: 1rem 0 0;
		color: var(--color-text-secondary);
		font-size: 0.94rem;
		line-height: 1.55;
	}

	.event-count {
		flex: none;
		padding-bottom: 0.35rem;
		color: var(--color-text-muted);
		font-size: 0.82rem;
		font-weight: 650;
	}

	.timeline {
		display: grid;
		gap: 2.7rem;
	}

	.day-group > h2 {
		margin: 0 0 0.9rem;
		color: var(--color-text-muted);
		font-size: 0.76rem;
		font-weight: 760;
		letter-spacing: 0.04em;
		text-transform: uppercase;
	}

	ol {
		margin: 0;
		padding: 0;
		list-style: none;
	}

	li {
		position: relative;
		display: grid;
		grid-template-columns: 2.25rem minmax(0, 1fr);
		gap: 1rem;
		padding: 0.8rem 0 1.35rem;
	}

	li:not(:last-child)::before {
		position: absolute;
		top: 2.8rem;
		bottom: -0.2rem;
		left: 1.1rem;
		width: 1px;
		background: var(--theme-border);
		content: '';
	}

	.event-icon {
		position: relative;
		z-index: 1;
		display: grid;
		width: 2.25rem;
		height: 2.25rem;
		place-items: center;
		border: 1px solid var(--theme-border);
		border-radius: 50%;
		color: var(--theme-accent);
		background: var(--theme-accent-soft);
	}

	.event-icon.project {
		color: var(--color-text-secondary);
		background: var(--color-warning-soft);
	}

	.event-icon.section {
		color: var(--color-info);
		background: var(--color-info-soft);
	}

	.event-icon.agent {
		color: var(--color-agent);
		background: var(--color-agent-soft);
	}

	.event-icon svg {
		width: 1rem;
		height: 1rem;
		fill: none;
		stroke: currentColor;
		stroke-linecap: round;
		stroke-linejoin: round;
		stroke-width: 1.7;
	}

	.event-content {
		min-width: 0;
		padding-top: 0.25rem;
	}

	.event-summary {
		display: flex;
		align-items: baseline;
		justify-content: space-between;
		gap: 1.5rem;
	}

	.event-summary p {
		min-width: 0;
		margin: 0;
		color: var(--color-text-secondary);
		font-size: 0.94rem;
		line-height: 1.45;
	}

	.actor,
	.event-summary strong {
		color: var(--color-text);
		font-weight: 720;
	}

	.event-summary strong {
		word-break: break-word;
	}

	.event-summary p > * + * {
		margin-left: 0.28rem;
	}

	time {
		flex: none;
		color: var(--color-text-muted);
		font-size: 0.75rem;
		font-variant-numeric: tabular-nums;
	}

	.event-meta {
		display: flex;
		align-items: center;
		flex-wrap: wrap;
		gap: 0.45rem 0.8rem;
		margin-top: 0.45rem;
		color: var(--color-text-muted);
		font-size: 0.7rem;
	}

	.event-meta > span:first-child {
		padding: 0.15rem 0.4rem;
		border-radius: 0.25rem;
		color: var(--theme-accent);
		background: var(--theme-accent-soft);
		font-weight: 680;
	}

	details {
		position: relative;
	}

	summary {
		color: var(--color-text-muted);
		cursor: pointer;
	}

	dl {
		display: grid;
		grid-template-columns: auto minmax(0, 1fr);
		gap: 0.35rem 0.8rem;
		margin: 0.65rem 0 0;
		padding: 0.65rem 0.75rem;
		border: 1px solid var(--theme-border);
		border-radius: 0.45rem;
		background: var(--color-surface);
	}

	dt {
		color: var(--color-text-muted);
		font-weight: 700;
	}

	dd {
		min-width: 0;
		margin: 0;
		overflow-wrap: anywhere;
		font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
	}

	.empty-state {
		display: grid;
		justify-items: center;
		padding: 5rem 1rem;
		border: 1px dashed var(--theme-border);
		border-radius: 0.8rem;
		text-align: center;
	}

	.empty-icon {
		display: grid;
		width: 3rem;
		height: 3rem;
		place-items: center;
		border-radius: 50%;
		color: var(--theme-accent);
		background: var(--theme-accent-soft);
	}

	.empty-icon svg {
		width: 1.4rem;
		fill: none;
		stroke: currentColor;
		stroke-linecap: round;
		stroke-linejoin: round;
		stroke-width: 1.6;
	}

	.empty-state h2 {
		margin: 1rem 0 0.35rem;
		color: var(--color-text);
		font-size: 1.05rem;
	}

	.empty-state p {
		margin: 0;
		color: var(--color-text-muted);
		font-size: 0.85rem;
	}

	@media (max-width: 40rem) {
		.page-header {
			align-items: flex-start;
			flex-direction: column;
			gap: 0.8rem;
			margin-bottom: 2.4rem;
		}

		.event-count {
			padding: 0;
		}

		.event-summary {
			align-items: flex-start;
			flex-direction: column;
			gap: 0.25rem;
		}

		li {
			grid-template-columns: 2rem minmax(0, 1fr);
			gap: 0.75rem;
		}

		.event-icon {
			width: 2rem;
			height: 2rem;
		}

		li:not(:last-child)::before {
			top: 2.55rem;
			left: 0.98rem;
		}
	}
</style>
