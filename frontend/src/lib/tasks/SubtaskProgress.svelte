<script lang="ts">
	interface Props {
		completed: number;
		total: number;
	}

	let { completed, total }: Props = $props();
</script>

{#if total > 0}
	<span class="subtask-progress" aria-label={`${completed} of ${total} subtasks completed`}>
		<svg aria-hidden="true" viewBox="0 0 24 24">
			<path d="M6.5 12.5 10 16l7.5-8" />
		</svg>
		<span>{completed}/{total}</span>
	</span>
{/if}

<style>
	.subtask-progress {
		display: inline-flex;
		align-items: center;
		gap: 0.2rem;
		color: #708076;
		font-size: 0.72rem;
		font-weight: 700;
		white-space: nowrap;
	}

	svg {
		width: 0.8rem;
		height: 0.8rem;
		fill: none;
		stroke: currentColor;
		stroke-linecap: round;
		stroke-linejoin: round;
		stroke-width: 2;
	}
</style>
