export interface CurrentUser {
	username: string;
}

export interface LoginCredentials {
	login: string;
	password: string;
}

export class AuthenticationRequiredError extends Error {
	constructor() {
		super('Authentication is required.');
		this.name = 'AuthenticationRequiredError';
	}
}

export class InvalidCredentialsError extends Error {
	constructor() {
		super('Invalid login or password.');
		this.name = 'InvalidCredentialsError';
	}
}

export class AuthenticationRequestError extends Error {
	constructor(message: string) {
		super(message);
		this.name = 'AuthenticationRequestError';
	}
}

export async function getCurrentUser(fetcher: typeof fetch): Promise<CurrentUser> {
	const response = await fetcher('/api/auth/me', {
		credentials: 'same-origin',
		headers: { Accept: 'application/json' }
	});
	if (response.status === 401) {
		throw new AuthenticationRequiredError();
	}
	if (!response.ok) {
		throw new AuthenticationRequestError('Could not check the current session.');
	}

	return (await response.json()) as CurrentUser;
}

export async function login(fetcher: typeof fetch, credentials: LoginCredentials): Promise<void> {
	const response = await fetcher('/api/auth/login', {
		method: 'POST',
		credentials: 'same-origin',
		headers: {
			Accept: 'application/json',
			'Content-Type': 'application/json'
		},
		body: JSON.stringify(credentials)
	});
	if (response.status === 401) {
		throw new InvalidCredentialsError();
	}
	if (!response.ok) {
		throw new AuthenticationRequestError('Could not sign in.');
	}
}

export async function logout(fetcher: typeof fetch): Promise<void> {
	const response = await fetcher('/api/auth/logout', {
		method: 'POST',
		credentials: 'same-origin',
		headers: { Accept: 'application/json' }
	});
	if (!response.ok) {
		throw new AuthenticationRequestError('Could not sign out.');
	}
}
