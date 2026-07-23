<script lang="ts">
	import { afterNavigate } from '$app/navigation';
	import { onMount } from 'svelte';
	import { resetToSystemAppearance } from './theme';

	interface Props {
		forceSystem?: boolean;
	}

	let { forceSystem = false }: Props = $props();
	let systemPrefersDark = false;

	function sync() {
		if (forceSystem || !document.documentElement.dataset.appearance) {
			resetToSystemAppearance(document, systemPrefersDark);
		}
	}

	afterNavigate(({ to }) => {
		if (forceSystem || (to && !authenticatedAppearancePath(to.url.pathname))) {
			delete document.documentElement.dataset.appearance;
		}
		sync();
	});

	onMount(() => {
		const media = window.matchMedia('(prefers-color-scheme: dark)');
		const update = () => {
			systemPrefersDark = media.matches;
			sync();
		};
		update();
		media.addEventListener('change', update);
		return () => media.removeEventListener('change', update);
	});

	function authenticatedAppearancePath(pathname: string): boolean {
		return /^(\/(projects(?:\/|$)|today(?:\/|$)|activity(?:\/|$)|settings(?:\/|$))|\/$)/.test(
			pathname
		);
	}
</script>
