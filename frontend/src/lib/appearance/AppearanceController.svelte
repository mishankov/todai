<script lang="ts">
	import { browser } from '$app/environment';
	import { onMount } from 'svelte';
	import {
		applyAppearance,
		appearanceSavedEvent,
		cacheAppearance,
		effectiveAppearance,
		resetToSystemAppearance,
		type Appearance
	} from './theme';

	interface Props {
		appearance: Appearance;
	}

	let { appearance }: Props = $props();
	let savedAppearance = $derived(appearance);
	let systemPrefersDark = $state(
		browser && window.matchMedia('(prefers-color-scheme: dark)').matches
	);

	onMount(() => {
		const media = window.matchMedia('(prefers-color-scheme: dark)');
		const update = () => (systemPrefersDark = media.matches);
		const saved = (event: Event) => {
			savedAppearance = (event as CustomEvent<Appearance>).detail;
		};
		update();
		media.addEventListener('change', update);
		window.addEventListener(appearanceSavedEvent, saved);
		return () => {
			media.removeEventListener('change', update);
			window.removeEventListener(appearanceSavedEvent, saved);
			resetToSystemAppearance(document, media.matches);
		};
	});

	$effect(() => {
		cacheAppearance(window.localStorage, savedAppearance);
		applyAppearance(document, effectiveAppearance(savedAppearance, systemPrefersDark));
	});
</script>
