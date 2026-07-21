<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';

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
		--accent: #2d6540;
		--accent-soft: #dfeadf;
		--canvas: #f5f7f3;
		--border: #dce4d9;
		position: relative;
		display: grid;
		min-height: 100svh;
		place-items: center;
		padding: 6.5rem 2rem 2rem;
		background:
			radial-gradient(circle at 12% 8%, rgb(87 139 101 / 14%), transparent 32rem),
			radial-gradient(circle at 88% 92%, rgb(172 130 139 / 12%), transparent 34rem), var(--canvas);
		overflow: hidden;
	}
	.brand {
		position: absolute;
		top: 2rem;
		left: 2rem;
		display: inline-flex;
		align-items: center;
		gap: 0.7rem;
		color: #292927;
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
		color: #fff;
		background: var(--accent);
		font-weight: 800;
	}
	.error-card {
		display: grid;
		grid-template-columns: minmax(0, 1fr) minmax(22rem, 0.9fr);
		width: min(72rem, 100%);
		min-height: min(38rem, calc(100svh - 8.5rem));
		border: 1px solid var(--border);
		border-radius: 1.5rem;
		background: rgb(255 255 255 / 88%);
		box-shadow: 0 2rem 6rem rgb(31 54 37 / 11%);
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
		color: #292927;
		font-size: clamp(3rem, 7vw, 5.6rem);
		letter-spacing: -0.075em;
		line-height: 0.94;
	}
	.description {
		max-width: 34rem;
		margin: 1.6rem 0 0;
		color: #6f746e;
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
		color: #fff;
		background: var(--accent);
		box-shadow: 0 0.65rem 1.5rem rgb(45 101 64 / 17%);
	}
	.primary-action:hover {
		background: #245535;
	}
	.secondary-action {
		border: 1px solid var(--border);
		color: #4f544f;
		background: #fff;
	}
	.secondary-action:hover {
		border-color: #b9cbb9;
		background: #f8faf7;
	}
	.primary-action:focus-visible,
	.secondary-action:focus-visible,
	.brand:focus-visible {
		outline: 3px solid rgb(45 101 64 / 18%);
		outline-offset: 3px;
	}
	.hint {
		max-width: 31rem;
		margin: 1.3rem 0 0;
		color: #92968f;
		font-size: 0.78rem;
		line-height: 1.5;
	}
	.error-visual {
		position: relative;
		min-height: 28rem;
		border-left: 1px solid var(--border);
		background:
			linear-gradient(rgb(255 255 255 / 42%), rgb(255 255 255 / 42%)),
			repeating-linear-gradient(0deg, transparent 0 31px, rgb(45 101 64 / 5%) 31px 32px),
			repeating-linear-gradient(90deg, transparent 0 31px, rgb(45 101 64 / 5%) 31px 32px),
			var(--accent-soft);
		overflow: hidden;
	}
	.status-number {
		position: absolute;
		top: 8%;
		right: -0.08em;
		color: rgb(45 101 64 / 9%);
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
		stroke: rgb(45 101 64 / 42%);
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
		border: 1px solid rgb(45 101 64 / 16%);
		border-radius: 1rem;
		background: rgb(255 255 255 / 91%);
		box-shadow: 0 1.4rem 3.5rem rgb(35 69 44 / 17%);
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
		border: 2px solid #7e9a84;
		border-radius: 50%;
	}
	.task-title,
	.task-line {
		display: block;
		height: 0.65rem;
		border-radius: 999px;
		background: #d7e3d6;
	}
	.task-title {
		width: 58%;
		height: 0.85rem;
		background: #9eb8a2;
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
		color: #54715b;
		background: #edf3eb;
		font-size: 0.68rem;
		font-weight: 750;
	}
	.waypoint {
		position: absolute;
		width: 0.85rem;
		height: 0.85rem;
		border: 3px solid rgb(255 255 255 / 90%);
		border-radius: 50%;
		background: var(--accent);
		box-shadow: 0 0 0 1px rgb(45 101 64 / 22%);
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
