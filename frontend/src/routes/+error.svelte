<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import SystemAppearanceController from '$lib/appearance/SystemAppearanceController.svelte';

	let notFound = $derived(page.status === 404);
	let heading = $derived(notFound ? 'This page wandered off.' : 'Something went wrong.');
	let description = $derived(
		notFound
			? 'The page you’re looking for may have moved, or the address might be slightly off.'
			: 'Todai hit an unexpected snag. Your tasks are safe — try loading the page again.'
	);

	function goBack() {
		if (window.history.length > 1) {
			window.history.back();
			return;
		}
		void goto(resolve('/'));
	}
</script>

<svelte:head>
	<title>{page.status} · {notFound ? 'Page not found' : 'Error'} — Todai</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<SystemAppearanceController forceSystem />
<main class="error-shell">
	<a class="brand" href={resolve('/')} aria-label="Todai home">
		<span aria-hidden="true">T</span>
		<strong>Todai</strong>
	</a>

	<section class="error-card" aria-labelledby="error-title">
		<div class="error-copy">
			<p class="eyebrow">{page.status} · {notFound ? 'Page not found' : 'Unexpected error'}</p>
			<h1 id="error-title">{heading}</h1>
			<p class="description">{description}</p>

			<div class="actions">
				{#if notFound}
					<a class="primary-action" href={resolve('/')}>Back to Todai</a>
				{:else}
					<button class="primary-action" type="button" onclick={() => window.location.reload()}>
						Try again
					</button>
				{/if}
				<button class="secondary-action" type="button" onclick={goBack}>Go back</button>
			</div>

			{#if notFound}
				<p class="hint">Check the address, or return to your projects and choose another path.</p>
			{/if}
		</div>

		<div class="error-visual" aria-hidden="true">
			<span class="status-number">{page.status}</span>
			<svg class="route" viewBox="0 0 420 300">
				<path d="M48 244C85 199 104 222 135 190S181 144 218 169 272 182 300 137 335 82 380 68" />
			</svg>
			<div class="task-card">
				<div class="task-heading">
					<span class="checkbox"></span>
					<span class="task-title"></span>
				</div>
				<span class="task-line long"></span>
				<span class="task-line"></span>
				<span class="task-label">No section</span>
			</div>
			<span class="waypoint start"></span>
			<span class="waypoint end"></span>
		</div>
	</section>
</main>

<style>
	.error-shell {
		--accent: var(--theme-accent);
		--accent-solid: var(--theme-accent-solid, var(--theme-accent));
		--accent-soft: var(--theme-accent-soft);
		--canvas: var(--color-canvas);
		--border: var(--color-border);
		position: relative;
		display: grid;
		min-height: 100svh;
		place-items: center;
		padding: 6.5rem 2rem 2rem;
		background:
			radial-gradient(
				circle at 12% 8%,
				color-mix(in srgb, var(--accent) 14%, transparent),
				transparent 32rem
			),
			radial-gradient(
				circle at 88% 92%,
				color-mix(in srgb, var(--color-error) 10%, transparent),
				transparent 34rem
			),
			var(--canvas);
		overflow: hidden;
	}
	.brand {
		position: absolute;
		top: 2rem;
		left: 2rem;
		display: inline-flex;
		align-items: center;
		gap: 0.7rem;
		color: var(--color-text);
		font-size: 1.1rem;
		letter-spacing: -0.03em;
		text-decoration: none;
	}
	.brand span {
		display: grid;
		width: 2.35rem;
		height: 2.35rem;
		place-items: center;
		border-radius: 0.7rem;
		color: var(--color-on-accent);
		background: var(--accent-solid);
		font-weight: 800;
	}
	.error-card {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(22rem, 0.9fr);
		width: min(72rem, 100%);
		min-height: min(38rem, calc(100svh - 8.5rem));
		border: 1px solid var(--border);
		border-radius: 1.5rem;
		background: color-mix(in srgb, var(--color-surface) 88%, transparent);
		box-shadow: var(--shadow-modal);
		overflow: hidden;
		backdrop-filter: blur(18px);
	}
	.error-copy {
		display: flex;
		flex-direction: column;
		justify-content: center;
		padding: clamp(2.25rem, 7vw, 5rem);
	}
	.eyebrow {
		margin: 0 0 1rem;
		color: var(--accent);
		font-size: 0.76rem;
		font-weight: 800;
		letter-spacing: 0.12em;
		text-transform: uppercase;
	}
	h1 {
		max-width: 9ch;
		margin: 0;
		color: var(--color-text);
		font-size: clamp(3rem, 7vw, 5.6rem);
		letter-spacing: -0.075em;
		line-height: 0.94;
	}
	.description {
		max-width: 34rem;
		margin: 1.6rem 0 0;
		color: var(--color-text-secondary);
		font-size: clamp(1rem, 2vw, 1.18rem);
		line-height: 1.65;
	}
	.actions {
		display: flex;
		flex-wrap: wrap;
		gap: 0.7rem;
		margin-top: 2.2rem;
	}
	.primary-action,
	.secondary-action {
		display: inline-flex;
		min-height: 3rem;
		align-items: center;
		justify-content: center;
		padding: 0.75rem 1.15rem;
		border-radius: 0.7rem;
		font: inherit;
		font-size: 0.9rem;
		font-weight: 750;
		text-decoration: none;
		cursor: pointer;
	}
	.primary-action {
		border: 1px solid var(--accent);
		color: var(--color-on-accent);
		background: var(--accent-solid);
		box-shadow: var(--shadow-small);
	}
	.primary-action:hover {
		background: color-mix(in srgb, var(--accent-solid) 86%, black);
	}
	.secondary-action {
		border: 1px solid var(--border);
		color: var(--color-text-secondary);
		background: var(--color-surface);
	}
	.secondary-action:hover {
		border-color: var(--color-border-strong);
		background: var(--color-control);
	}
	.primary-action:focus-visible,
	.secondary-action:focus-visible,
	.brand:focus-visible {
		outline: 3px solid var(--theme-focus);
		outline-offset: 3px;
	}
	.hint {
		max-width: 31rem;
		margin: 1.3rem 0 0;
		color: var(--color-text-muted);
		font-size: 0.78rem;
		line-height: 1.5;
	}
	.error-visual {
		position: relative;
		min-height: 28rem;
		border-left: 1px solid var(--border);
		background:
			linear-gradient(
				color-mix(in srgb, var(--color-surface) 42%, transparent),
				color-mix(in srgb, var(--color-surface) 42%, transparent)
			),
			repeating-linear-gradient(
				0deg,
				transparent 0 31px,
				color-mix(in srgb, var(--accent) 8%, transparent) 31px 32px
			),
			repeating-linear-gradient(
				90deg,
				transparent 0 31px,
				color-mix(in srgb, var(--accent) 8%, transparent) 31px 32px
			),
			var(--accent-soft);
		overflow: hidden;
	}
	.status-number {
		position: absolute;
		top: 8%;
		right: -0.08em;
		color: color-mix(in srgb, var(--accent) 12%, transparent);
		font-size: clamp(9rem, 23vw, 18rem);
		font-weight: 900;
		letter-spacing: -0.12em;
		line-height: 1;
	}
	.route {
		position: absolute;
		inset: 20% 2% auto;
		width: 96%;
		overflow: visible;
	}
	.route path {
		fill: none;
		stroke: color-mix(in srgb, var(--accent) 55%, transparent);
		stroke-dasharray: 7 10;
		stroke-linecap: round;
		stroke-width: 2;
	}
	.task-card {
		position: absolute;
		bottom: 16%;
		left: 50%;
		width: min(18rem, 72%);
		padding: 1.25rem;
		border: 1px solid color-mix(in srgb, var(--accent) 32%, var(--border));
		border-radius: 1rem;
		background: color-mix(in srgb, var(--color-surface) 91%, transparent);
		box-shadow: var(--shadow-elevated);
		transform: translateX(-50%) rotate(-3deg);
	}
	.task-heading {
		display: flex;
		align-items: center;
		gap: 0.75rem;
	}
	.checkbox {
		width: 1.3rem;
		height: 1.3rem;
		flex: none;
		border: 2px solid var(--color-border-strong);
		border-radius: 50%;
	}
	.task-title,
	.task-line {
		display: block;
		height: 0.65rem;
		border-radius: 999px;
		background: var(--color-border);
	}
	.task-title {
		width: 58%;
		height: 0.85rem;
		background: color-mix(in srgb, var(--accent) 50%, var(--color-border));
	}
	.task-line {
		width: 56%;
		margin-top: 0.72rem;
	}
	.task-line.long {
		width: 88%;
		margin-top: 1.1rem;
	}
	.task-label {
		display: inline-flex;
		margin-top: 1.15rem;
		padding: 0.38rem 0.55rem;
		border-radius: 999px;
		color: var(--accent);
		background: var(--color-hover);
		font-size: 0.68rem;
		font-weight: 750;
	}
	.waypoint {
		position: absolute;
		width: 0.85rem;
		height: 0.85rem;
		border: 3px solid color-mix(in srgb, var(--color-surface) 90%, transparent);
		border-radius: 50%;
		background: var(--accent-solid);
		box-shadow: 0 0 0 1px color-mix(in srgb, var(--accent) 35%, transparent);
	}
	.waypoint.start {
		bottom: 24%;
		left: 9%;
	}
	.waypoint.end {
		top: 19%;
		right: 8%;
	}
	@media (max-width: 52rem) {
		.error-shell {
			padding: 5.5rem 1rem 1rem;
			place-items: start center;
		}
		.brand {
			top: 1.25rem;
			left: 1.25rem;
		}
		.error-card {
			grid-template-columns: 1fr;
			min-height: 0;
		}
		.error-copy {
			padding: 2.5rem 1.5rem;
		}
		h1 {
			font-size: clamp(3rem, 15vw, 4.6rem);
		}
		.error-visual {
			min-height: 18rem;
			border-top: 1px solid var(--border);
			border-left: 0;
		}
		.status-number {
			font-size: clamp(8rem, 42vw, 13rem);
		}
		.route {
			inset: 4% 4% auto;
			width: 92%;
		}
		.task-card {
			bottom: 11%;
			width: min(17rem, 78%);
		}
	}
	@media (max-width: 30rem) {
		.error-copy {
			padding: 2rem 1.15rem;
		}
		.actions {
			display: grid;
		}
		.primary-action,
		.secondary-action {
			width: 100%;
		}
	}
	@media (prefers-reduced-motion: no-preference) {
		.task-card {
			animation: drift 5s ease-in-out infinite;
		}
		@keyframes drift {
			0%,
			100% {
				transform: translateX(-50%) rotate(-3deg) translateY(0);
			}
			50% {
				transform: translateX(-50%) rotate(-1deg) translateY(-0.45rem);
			}
		}
	}
</style>
