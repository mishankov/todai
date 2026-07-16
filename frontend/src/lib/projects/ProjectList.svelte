<script lang="ts">
	import { resolve } from '$app/paths';
	import { invalidate } from '$app/navigation';
	import { ProjectConflictError, type Project, createProject, updateProject } from './client';

	interface Props {
		initialProjects: Project[];
	}

	let { initialProjects }: Props = $props();
	let projects = $derived([...initialProjects]);
	let activeProjects = $derived(projects.filter((project) => project.archivedAt === null));
	let archivedProjects = $derived(projects.filter((project) => project.archivedAt !== null));
	let name = $state('');
	let creating = $state(false);
	let editingId = $state<string | null>(null);
	let editingName = $state('');
	let busyId = $state<string | null>(null);
	let errorMessage = $state('');

	async function create() {
		if (!name.trim()) return;
		creating = true;
		errorMessage = '';
		try {
			const created = await createProject(fetch, name);
			projects = [...projects, created];
			name = '';
			await refreshProjectLoads();
		} catch {
			errorMessage = 'The project could not be created. Please try again.';
		} finally {
			creating = false;
		}
	}

	function beginRename(project: Project) {
		editingId = project.id;
		editingName = project.name;
		errorMessage = '';
	}

	async function rename(project: Project) {
		if (!editingName.trim()) return;
		busyId = project.id;
		errorMessage = '';
		try {
			const updated = await updateProject(fetch, project.id, {
				version: project.version,
				name: editingName
			});
			projects = projects.map((candidate) => (candidate.id === updated.id ? updated : candidate));
			editingId = null;
			await refreshProjectLoads();
		} catch (error) {
			errorMessage =
				error instanceof ProjectConflictError
					? 'This project changed elsewhere. Reload before saving.'
					: 'The project could not be renamed. Please try again.';
		} finally {
			busyId = null;
		}
	}

	async function archive(project: Project) {
		busyId = project.id;
		errorMessage = '';
		try {
			const updated = await updateProject(fetch, project.id, {
				version: project.version,
				archived: true
			});
			projects = projects.map((candidate) => (candidate.id === updated.id ? updated : candidate));
			await refreshProjectLoads();
		} catch {
			errorMessage = 'The project could not be archived. Please try again.';
		} finally {
			busyId = null;
		}
	}

	async function restore(project: Project) {
		busyId = project.id;
		errorMessage = '';
		try {
			const updated = await updateProject(fetch, project.id, {
				version: project.version,
				archived: false
			});
			projects = projects.map((candidate) => (candidate.id === updated.id ? updated : candidate));
			await refreshProjectLoads();
		} catch {
			errorMessage = 'The project could not be restored. Please try again.';
		} finally {
			busyId = null;
		}
	}

	function refreshProjectLoads(): Promise<void> {
		return invalidate((url) => url.pathname === '/api/projects');
	}
</script>

<section class="projects" aria-labelledby="projects-heading">
	<header>
		<div>
			<p>My workspace</p>
			<h1 id="projects-heading">Projects</h1>
		</div>
		<span>{activeProjects.length} active</span>
	</header>

	<form
		class="create"
		onsubmit={(event) => {
			event.preventDefault();
			void create();
		}}
	>
		<label class="sr-only" for="project-name">Project name</label>
		<input id="project-name" bind:value={name} maxlength="200" placeholder="New project…" />
		<button type="submit" disabled={creating || !name.trim()}
			>{creating ? 'Creating…' : 'Create'}</button
		>
	</form>

	{#if errorMessage}<p class="error" role="alert">{errorMessage}</p>{/if}

	{#if activeProjects.length > 0}
		<ul aria-label="Projects">
			{#each activeProjects as project (project.id)}
				<li>
					{#if editingId === project.id}
						<form
							class="rename"
							onsubmit={(event) => {
								event.preventDefault();
								void rename(project);
							}}
						>
							<input bind:value={editingName} maxlength="200" aria-label="Project name" />
							<button type="button" onclick={() => (editingId = null)}>Cancel</button>
							<button type="submit" disabled={busyId === project.id || !editingName.trim()}
								>Save</button
							>
						</form>
					{:else}
						<a href={resolve(`/projects/${project.id}`)}>{project.name}</a>
						<div class="actions">
							<button type="button" onclick={() => beginRename(project)}>Rename</button>
							<button
								type="button"
								disabled={busyId === project.id}
								onclick={() => void archive(project)}
							>
								Archive
							</button>
						</div>
					{/if}
				</li>
			{/each}
		</ul>
	{:else}
		<div class="empty">
			<strong>No projects yet.</strong><span>Create one to organize related tasks.</span>
		</div>
	{/if}

	{#if archivedProjects.length > 0}
		<section class="archived" aria-labelledby="archived-heading">
			<h2 id="archived-heading">Archived</h2>
			<ul aria-label="Archived projects">
				{#each archivedProjects as project (project.id)}
					<li>
						<span>{project.name}</span>
						<button
							type="button"
							disabled={busyId === project.id}
							onclick={() => void restore(project)}>Restore</button
						>
					</li>
				{/each}
			</ul>
		</section>
	{/if}
</section>

<style>
	.projects {
		width: min(46rem, 100%);
		margin: 0 auto;
	}
	header {
		display: flex;
		align-items: end;
		justify-content: space-between;
		margin-bottom: 2rem;
	}
	header p {
		margin: 0 0 0.4rem;
		color: #52705a;
		font-size: 0.72rem;
		font-weight: 750;
		letter-spacing: 0.12em;
		text-transform: uppercase;
	}
	h1 {
		margin: 0;
		color: #17211a;
		font-size: clamp(2rem, 6vw, 3.4rem);
		letter-spacing: -0.055em;
	}
	header span {
		color: #6c756f;
		font-size: 0.8rem;
		font-weight: 650;
	}
	.create,
	.rename {
		display: flex;
		gap: 0.65rem;
	}
	.create {
		margin-bottom: 1.5rem;
	}
	input {
		min-width: 0;
		flex: 1;
		padding: 0.8rem 0.9rem;
		border: 1px solid #ccd6ca;
		border-radius: 0.7rem;
		background: #fff;
	}
	button {
		padding: 0.65rem 0.8rem;
		border: 1px solid #c8d3c7;
		border-radius: 0.65rem;
		color: #31523a;
		background: #fff;
		font-weight: 700;
		cursor: pointer;
	}
	.create button,
	.rename button[type='submit'] {
		border-color: #2d6540;
		color: #fff;
		background: #2d6540;
	}
	button:disabled {
		cursor: wait;
		opacity: 0.5;
	}
	ul {
		display: grid;
		gap: 0.65rem;
		padding: 0;
		list-style: none;
	}
	li {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 1rem;
		padding: 1rem 1.1rem;
		border: 1px solid #dbe3d9;
		border-radius: 0.85rem;
		background: rgb(255 255 255 / 72%);
	}
	li > a {
		flex: 1;
		color: #203126;
		font-weight: 750;
		text-decoration: none;
	}
	li > a:hover {
		color: #2d6540;
	}
	.actions {
		display: flex;
		gap: 0.45rem;
	}
	.rename {
		width: 100%;
	}
	.empty {
		display: grid;
		gap: 0.3rem;
		padding: 4rem 1rem;
		color: #6b756e;
		text-align: center;
	}
	.empty strong {
		color: #33443a;
	}
	.archived {
		margin-top: 2.5rem;
	}
	.archived h2 {
		color: #526058;
		font-size: 0.8rem;
		letter-spacing: 0.08em;
		text-transform: uppercase;
	}
	.archived li > span {
		color: #657169;
	}
	.error {
		color: #8c2828;
		font-size: 0.84rem;
	}
	.sr-only {
		position: absolute;
		width: 1px;
		height: 1px;
		overflow: hidden;
		clip: rect(0, 0, 0, 0);
	}
	@media (max-width: 34rem) {
		li {
			align-items: stretch;
			flex-direction: column;
		}
		.actions {
			justify-content: flex-end;
		}
	}
</style>
