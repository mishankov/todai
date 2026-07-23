export type Appearance = 'system' | 'light' | 'dark';
export type EffectiveAppearance = Exclude<Appearance, 'system'>;

export const appearanceCacheKey = 'todai:appearance';
export const appearanceSavedEvent = 'todai:appearance-saved';

export function effectiveAppearance(
	appearance: Appearance,
	systemPrefersDark: boolean
): EffectiveAppearance {
	return appearance === 'system' ? (systemPrefersDark ? 'dark' : 'light') : appearance;
}

export function cachedAppearance(storage: Pick<Storage, 'getItem'>): Appearance | null {
	try {
		const value = storage.getItem(appearanceCacheKey);
		return value === 'system' || value === 'light' || value === 'dark' ? value : null;
	} catch {
		return null;
	}
}

export function cacheAppearance(storage: Pick<Storage, 'setItem'>, appearance: Appearance): void {
	try {
		storage.setItem(appearanceCacheKey, appearance);
	} catch {
		// Persistence is an optimization; the server remains the source of truth.
	}
}

export function applyAppearance(document: Document, appearance: EffectiveAppearance): void {
	document.documentElement.dataset.appearance = appearance;
	document.documentElement.style.colorScheme = appearance;
	const themeColor = document.querySelector<HTMLMetaElement>('meta[name="theme-color"]');
	if (themeColor) themeColor.content = appearance === 'dark' ? '#121512' : '#f5f7f3';
}

export function publishSavedAppearance(appearance: Appearance): void {
	window.dispatchEvent(new CustomEvent<Appearance>(appearanceSavedEvent, { detail: appearance }));
}

export function resetToSystemAppearance(document: Document, systemPrefersDark: boolean): void {
	delete document.documentElement.dataset.appearance;
	document.documentElement.style.colorScheme = '';
	const themeColor = document.querySelector<HTMLMetaElement>('meta[name="theme-color"]');
	if (themeColor) themeColor.content = systemPrefersDark ? '#121512' : '#f5f7f3';
}
