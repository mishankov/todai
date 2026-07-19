export interface UserSettings {
	timezone: string | null;
	agentModel: string;
	agentThinkingEffort: AgentThinkingEffort;
	version: number;
	createdAt: string | null;
	updatedAt: string | null;
	lastModifiedBy: string;
}

export interface SettingsView {
	settings: UserSettings;
	availableAgentModels: string[];
	availableAgentThinkingEfforts: AgentThinkingEffort[];
}

export type AgentThinkingEffort = 'off' | 'minimal' | 'low' | 'medium' | 'high' | 'xhigh' | 'max';

export interface SettingsUpdate {
	timezone: string;
	agentModel: string;
	agentThinkingEffort: AgentThinkingEffort;
	version: number;
}

export class SettingsRequestError extends Error {
	constructor(message: string) {
		super(message);
		this.name = 'SettingsRequestError';
	}
}

export class SettingsConflictError extends SettingsRequestError {
	constructor() {
		super('Settings changed after this page was opened. Reload and try again.');
		this.name = 'SettingsConflictError';
	}
}

export async function getSettings(fetcher: typeof fetch): Promise<SettingsView> {
	const response = await fetcher('/api/settings', {
		credentials: 'same-origin',
		headers: { Accept: 'application/json' }
	});
	if (!response.ok) throw new SettingsRequestError('Could not load settings.');
	return (await response.json()) as SettingsView;
}

export async function updateSettings(
	fetcher: typeof fetch,
	update: SettingsUpdate
): Promise<SettingsView> {
	const response = await fetcher('/api/settings', {
		method: 'PATCH',
		credentials: 'same-origin',
		headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
		body: JSON.stringify(update)
	});
	if (response.status === 409) throw new SettingsConflictError();
	if (!response.ok) throw new SettingsRequestError('Could not save settings.');
	return (await response.json()) as SettingsView;
}
