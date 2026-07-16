<script lang="ts">
	import { resolve } from '$app/paths';
	import type { Project } from '$lib/projects/client';
	import type { Snippet } from 'svelte';

	interface Props {
		username: string;
		projects?: Project[];
		onLogout: () => Promise<void>;
		currentPath?: string;
		children?: Snippet;
	}

	let { username, projects = [], onLogout, currentPath = '/', children }: Props = $props();
	let signingOut = $state(false);
	let errorMessage = $state('');
	let sidebarOpen = $state(false);

	async function signOut() {
		signingOut = true;
		errorMessage = '';

		try {
			await onLogout();
		} catch {
			errorMessage = 'Sign out failed. Please try again.';
			signingOut = false;
		}
	}

	function projectIsActive(projectId: string): boolean {
		return currentPath === `/projects/${projectId}`;
	}

	function closeSidebar() {
		sidebarOpen = false;
	}
</script>

<main class="shell">
	<button
		class="sidebar-backdrop"
		class:visible={sidebarOpen}
		type="button"
		aria-label="Close navigation"
		tabindex={sidebarOpen ? 0 : -1}
		onclick={closeSidebar}
	></button>

	<aside class:open={sidebarOpen} aria-label="Application sidebar">
		<div class="sidebar-heading">
			<a class="brand" href={resolve('/')} onclick={closeSidebar}>
				<span class="mark" aria-hidden="true">T</span>
				<span>Todai</span>
			</a>
			<button class="close-sidebar" type="button" aria-label="Close sidebar" onclick={closeSidebar}>
				<svg viewBox="0 0 24 24" aria-hidden="true">
					<path d="m7 7 10 10M17 7 7 17" />
				</svg>
			</button>
		</div>

		<div class="profile">
			<span class="username">{username}</span>
		</div>

		<nav class="primary-navigation" aria-label="Primary navigation">
			<a
				href={resolve('/')}
				class:active={currentPath === '/'}
				aria-current={currentPath === '/' ? 'page' : undefined}
				onclick={closeSidebar}
			>
				<svg viewBox="0 0 24 24" aria-hidden="true">
					<path d="M4 7.5h16v11H4zM7 4h10l3 3.5H4zM8 13h8" />
				</svg>
				<span>Inbox</span>
			</a>
			<a
				href={resolve('/today')}
				class:active={currentPath === '/today'}
				aria-current={currentPath === '/today' ? 'page' : undefined}
				onclick={closeSidebar}
			>
				<svg viewBox="0 0 24 24" aria-hidden="true">
					<rect x="4" y="5" width="16" height="15" rx="2" />
					<path d="M8 3v4M16 3v4M4 9h16" />
					<path d="M9 13h.01M12 13h.01M15 13h.01M9 16h.01M12 16h.01" />
				</svg>
				<span>Today</span>
			</a>
		</nav>

		<section class="project-navigation" aria-labelledby="sidebar-projects">
			<div class="section-title">
				<h2 id="sidebar-projects">
					<a href={resolve('/projects')} onclick={closeSidebar}>Projects</a>
				</h2>
			</div>
			<nav aria-label="Projects">
				{#each projects as project (project.id)}
					<a
						href={resolve(`/projects/${project.id}`)}
						class:active={projectIsActive(project.id)}
						aria-current={projectIsActive(project.id) ? 'page' : undefined}
						onclick={closeSidebar}
					>
						<span>{project.name}</span>
					</a>
				{/each}
				{#if projects.length === 0}
					<a class="empty-projects" href={resolve('/projects')} onclick={closeSidebar}>
						Create your first project
					</a>
				{/if}
			</nav>
		</section>

		<div class="session">
			<button type="button" disabled={signingOut} onclick={() => void signOut()}>
				<svg viewBox="0 0 24 24" aria-hidden="true">
					<path d="M10 5H5v14h5M14 8l4 4-4 4M8 12h10" />
				</svg>
				{signingOut ? 'Signing out…' : 'Log out'}
			</button>
			{#if errorMessage}
				<p class="error" role="alert">{errorMessage}</p>
			{/if}
		</div>
	</aside>

	<section class="content">
		<header class="mobile-bar">
			<button type="button" aria-label="Open navigation" onclick={() => (sidebarOpen = true)}>
				<svg viewBox="0 0 24 24" aria-hidden="true">
					<path d="M4 7h16M4 12h16M4 17h16" />
				</svg>
			</button>
			<a class="brand" href={resolve('/')}>
				<span class="mark" aria-hidden="true">T</span>
				<span>Todai</span>
			</a>
		</header>

		<section class="workspace">
			{#if children}
				{@render children()}
			{/if}
		</section>
	</section>
</main>

<style>
	.shell {
		display: grid;
		grid-template-columns: 17rem minmax(0, 1fr);
		min-height: 100vh;
		background: #fbfcfa;
	}

	aside {
		position: sticky;
		top: 0;
		display: flex;
		flex-direction: column;
		height: 100vh;
		padding: 1rem 0.75rem;
		border-right: 1px solid #dfe5dc;
		background: #f1f5ef;
		overflow-y: auto;
	}

	.sidebar-heading,
	.mobile-bar {
		display: flex;
		align-items: center;
		justify-content: space-between;
	}

	.brand {
		display: flex;
		align-items: center;
		gap: 0.65rem;
		padding: 0.25rem 0.4rem;
		color: #292927;
		font-weight: 760;
		letter-spacing: -0.025em;
		text-decoration: none;
	}

	.mark {
		display: grid;
		place-items: center;
		color: #fff;
		background: #2d6540;
	}

	.mark {
		width: 1.75rem;
		height: 1.75rem;
		border-radius: 0.5rem;
	}

	.close-sidebar {
		display: none;
	}

	.profile {
		display: flex;
		align-items: center;
		gap: 0.65rem;
		margin: 1.4rem 0 0.75rem;
		padding: 0.5rem 0.55rem;
	}

	.username {
		overflow: hidden;
		color: #353532;
		font-size: 0.86rem;
		font-weight: 700;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.primary-navigation,
	.project-navigation nav {
		display: grid;
		gap: 0.15rem;
	}

	.primary-navigation a,
	.project-navigation nav a {
		display: flex;
		align-items: center;
		gap: 0.7rem;
		min-width: 0;
		padding: 0.48rem 0.6rem;
		border-radius: 0.38rem;
		color: #4b4b48;
		font-size: 0.86rem;
		font-weight: 560;
		text-decoration: none;
	}

	.primary-navigation a:hover,
	.project-navigation nav a:hover {
		background: #e6ece4;
	}

	.primary-navigation a.active,
	.project-navigation nav a.active {
		color: #2d6540;
		background: #dfeadf;
		font-weight: 700;
	}

	.primary-navigation svg,
	.session svg,
	.mobile-bar svg,
	.close-sidebar svg {
		width: 1.2rem;
		height: 1.2rem;
		fill: none;
		stroke: currentColor;
		stroke-linecap: round;
		stroke-linejoin: round;
		stroke-width: 1.6;
	}

	.project-navigation {
		margin-top: 1.7rem;
	}

	.section-title {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 0 0.55rem 0.45rem;
	}

	.section-title h2 {
		margin: 0;
		color: #6d6d68;
		font-size: 0.72rem;
		font-weight: 760;
		letter-spacing: 0.02em;
		text-transform: uppercase;
	}

	.section-title h2 a {
		color: inherit;
		text-decoration: none;
	}

	.section-title h2 a:hover {
		color: #2d6540;
	}

	.project-navigation nav a span:last-child {
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.project-navigation nav .empty-projects {
		color: #85857f;
		font-size: 0.78rem;
		font-weight: 500;
	}

	.session {
		margin-top: auto;
		padding-top: 1.5rem;
	}

	.session button {
		display: flex;
		align-items: center;
		gap: 0.65rem;
		width: 100%;
		padding: 0.55rem 0.6rem;
		border: 0;
		border-radius: 0.38rem;
		color: #656560;
		background: transparent;
		font-size: 0.8rem;
		font-weight: 650;
		cursor: pointer;
	}

	.session button:hover:not(:disabled) {
		color: #2d6540;
		background: #e6ece4;
	}

	.session button:disabled {
		cursor: wait;
		opacity: 0.55;
	}

	.content {
		min-width: 0;
	}

	.workspace {
		padding: clamp(3rem, 7vw, 5.5rem) clamp(1.5rem, 6vw, 6rem);
	}

	.mobile-bar {
		display: none;
	}

	.sidebar-backdrop {
		display: none;
	}

	.error {
		margin: 0.6rem 0.5rem 0;
		color: #b83f34;
		font-size: 0.76rem;
	}

	@media (max-width: 48rem) {
		.shell {
			display: block;
		}

		aside {
			position: fixed;
			z-index: 20;
			left: 0;
			width: min(19rem, 86vw);
			transform: translateX(-102%);
			box-shadow: 1rem 0 3rem rgb(30 29 27 / 14%);
			transition: transform 160ms ease;
		}

		aside.open {
			transform: translateX(0);
		}

		.close-sidebar {
			display: grid;
			width: 2rem;
			height: 2rem;
			place-items: center;
			padding: 0;
			border: 0;
			border-radius: 0.4rem;
			color: #66625f;
			background: transparent;
		}

		.sidebar-backdrop {
			position: fixed;
			z-index: 15;
			inset: 0;
			display: block;
			border: 0;
			background: rgb(23 22 20 / 30%);
			opacity: 0;
			pointer-events: none;
			transition: opacity 160ms ease;
		}

		.sidebar-backdrop.visible {
			opacity: 1;
			pointer-events: auto;
		}

		.mobile-bar {
			position: sticky;
			z-index: 10;
			top: 0;
			display: flex;
			height: 3.5rem;
			padding: 0 1rem;
			border-bottom: 1px solid #dfe5dc;
			background: rgb(251 252 250 / 92%);
			backdrop-filter: blur(12px);
		}

		.mobile-bar > button {
			display: grid;
			width: 2.2rem;
			height: 2.2rem;
			place-items: center;
			padding: 0;
			border: 0;
			border-radius: 0.45rem;
			color: #4e4d49;
			background: transparent;
		}

		.mobile-bar .brand {
			margin-right: auto;
			margin-left: 0.4rem;
		}

		.mobile-bar .mark {
			width: 1.5rem;
			height: 1.5rem;
			font-size: 0.8rem;
		}

		.workspace {
			padding: 2.5rem 1.1rem 4rem;
		}
	}
</style>
