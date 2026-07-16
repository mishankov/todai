<main class="shell">
	<header class="brand">
		<span class="mark" aria-hidden="true">T</span>
		<span>Todai</span>
	</header>

	<section class="workspace" aria-labelledby="workspace-heading">
		<p class="eyebrow">Development skeleton</p>
		<h1 id="workspace-heading">Your task space is taking shape.</h1>
		<p class="summary">
			The application shell is ready. Projects, Inbox and Today arrive in the next vertical slice.
		</p>
		<div class="status" role="status">
			<span class="status-dot" aria-hidden="true"></span>
			Stage 0 · Backend and frontend foundation
		</div>
	</section>
</main>

<style>
	.shell {
		display: grid;
		grid-template-rows: auto 1fr;
		min-height: 100vh;
		padding: 1.5rem clamp(1.25rem, 4vw, 4rem);
	}

	.brand {
		display: flex;
		align-items: center;
		gap: 0.7rem;
		font-weight: 700;
		letter-spacing: -0.02em;
	}

	.mark {
		display: grid;
		width: 2rem;
		height: 2rem;
		place-items: center;
		border-radius: 0.65rem;
		color: #fff;
		background: #2d6540;
	}

	.workspace {
		align-self: center;
		width: min(42rem, 100%);
		margin: 4rem auto;
		padding: clamp(2rem, 6vw, 5rem);
		border: 1px solid #dce3da;
		border-radius: 1.5rem;
		background: rgb(255 255 255 / 82%);
		box-shadow: 0 1.5rem 5rem rgb(24 56 34 / 8%);
	}

	.eyebrow {
		margin: 0 0 1rem;
		color: #477153;
		font-size: 0.78rem;
		font-weight: 700;
		letter-spacing: 0.12em;
		text-transform: uppercase;
	}

	h1 {
		margin: 0;
		font-size: clamp(2.25rem, 7vw, 4.75rem);
		line-height: 0.98;
		letter-spacing: -0.06em;
	}

	.summary {
		max-width: 34rem;
		margin: 1.5rem 0 2rem;
		color: #5c675e;
		font-size: 1.05rem;
		line-height: 1.65;
	}

	.status {
		display: inline-flex;
		align-items: center;
		gap: 0.65rem;
		padding: 0.65rem 0.85rem;
		border-radius: 999px;
		color: #31523a;
		background: #edf5ed;
		font-size: 0.82rem;
		font-weight: 650;
	}

	.status-dot {
		width: 0.5rem;
		height: 0.5rem;
		border-radius: 50%;
		background: #47a363;
		box-shadow: 0 0 0 0.25rem rgb(71 163 99 / 14%);
	}
</style>
