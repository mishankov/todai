import crypto from 'node:crypto';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import process from 'node:process';

const MODELS = csvEnv('TODAI_BENCH_MODELS', [
	'gpt-5.6-luna',
	'gpt-5.6-sol',
	'gpt-5.6-terra'
]);
const EFFORTS = csvEnv('TODAI_BENCH_EFFORTS', [
	'off',
	'minimal',
	'low',
	'medium',
	'high',
	'xhigh',
	'max'
]);
const RUNS_PER_CONFIG = Number(process.env.TODAI_BENCH_RUNS ?? 10);
const WARMUP_RUNS = Number(process.env.TODAI_BENCH_WARMUPS ?? 3);
const BASE_URL = process.env.TODAI_BENCH_URL ?? 'http://127.0.0.1:18080';
const USERNAME = process.env.TODAI_BENCH_USERNAME ?? 'admin';
const PASSWORD = process.env.TODAI_BENCH_PASSWORD;
const OUTPUT_PATH = path.resolve(process.argv[2] ?? 'benchmarks/decomposition-gpt-5.6/results.json');
const RUN_TIMEOUT_MS = Number(process.env.TODAI_BENCH_TIMEOUT_MS ?? 600_000);
const COOLDOWN_MS = Number(process.env.TODAI_BENCH_COOLDOWN_MS ?? 3000);
const SEED = 0x5_600_2026;
const SCENARIO = {
	title: 'Подготовить запуск новой версии мобильного приложения',
	description:
		'Релиз должен пройти для 100% пользователей. Нужно учесть финальную проверку качества, release notes, готовность поддержки, мониторинг ключевых метрик, поэтапный rollout и план отката.'
};

if (!PASSWORD) throw new Error('TODAI_BENCH_PASSWORD is required');

let cookie = '';
let checkpoint;
let stopping = false;

process.on('SIGINT', () => {
	stopping = true;
});
process.on('SIGTERM', () => {
	stopping = true;
});

await main();

async function main() {
	await fs.mkdir(path.dirname(OUTPUT_PATH), { recursive: true });
	checkpoint = await loadCheckpoint();
	await login();
	const initialView = await requestJSON('/api/settings');
	checkpoint.meta.availableModels = initialView.availableAgentModels;
	checkpoint.meta.availableEfforts = initialView.availableAgentThinkingEfforts;
	checkpoint.meta.originalSettings ??= {
		timezone: initialView.settings.timezone,
		agentModel: initialView.settings.agentModel,
		agentThinkingEffort: initialView.settings.agentThinkingEffort
	};
	await saveCheckpoint();

	const unsupportedModels = MODELS.filter((model) => !initialView.availableAgentModels.includes(model));
	const unsupportedEfforts = EFFORTS.filter(
		(effort) => !initialView.availableAgentThinkingEfforts.includes(effort)
	);
	if (unsupportedModels.length || unsupportedEfforts.length) {
		throw new Error(
			`Benchmark matrix is unavailable: models=${unsupportedModels.join(',')} efforts=${unsupportedEfforts.join(',')}`
		);
	}

	const schedule = buildSchedule();
	const completedKeys = new Set(
		checkpoint.runs.map((run) => `${run.model}|${run.effort}|${run.iteration}`)
	);
	const pending = schedule.filter(
		(item) => !completedKeys.has(`${item.model}|${item.effort}|${item.iteration}`)
	);
	const total = MODELS.length * EFFORTS.length * RUNS_PER_CONFIG;
	const startedWall = Date.now();
	if (!checkpoint.runs.length && !checkpoint.meta.warmedUpAt && WARMUP_RUNS > 0) {
		checkpoint.meta.warmups = [];
		for (let index = 0; index < WARMUP_RUNS; index += 1) {
			const model = MODELS[index % MODELS.length];
			const effort = EFFORTS.includes('medium') ? 'medium' : EFFORTS[0];
			console.log(JSON.stringify({ type: 'benchmark.warmup', index: index + 1, model, effort }));
			checkpoint.meta.warmups.push(
				await benchmarkOne({ model, effort, iteration: 0, roundPosition: index + 1 }, `warmup-${index + 1}`)
			);
			await saveCheckpoint();
			if (index + 1 < WARMUP_RUNS) await sleep(COOLDOWN_MS);
		}
		checkpoint.meta.warmedUpAt = new Date().toISOString();
		await saveCheckpoint();
	}

	console.log(
		JSON.stringify({
			type: 'benchmark.started',
			total,
			completed: checkpoint.runs.length,
			pending: pending.length,
			output: OUTPUT_PATH
		})
	);

	try {
		for (const item of pending) {
			if (stopping) break;
			const index = checkpoint.runs.length + 1;
			const result = await benchmarkOne(item, String(index));
			checkpoint.runs.push(result);
			checkpoint.meta.updatedAt = new Date().toISOString();
			await saveCheckpoint();
			const elapsed = Date.now() - startedWall;
			const finishedThisProcess = index - (total - pending.length);
			const average = finishedThisProcess > 0 ? elapsed / finishedThisProcess : 0;
			console.log(
				JSON.stringify({
					type: 'benchmark.progress',
					completed: checkpoint.runs.length,
					total,
					model: item.model,
					effort: item.effort,
					iteration: item.iteration,
					status: result.status,
					totalMs: result.timing.totalMs,
					subtasks: result.output.subtaskCount,
					etaMs: Math.round(average * (total - checkpoint.runs.length))
				})
			);
			if (!stopping) await sleep(COOLDOWN_MS);
		}
	} finally {
		await restoreSettings();
		checkpoint.meta.completedAt =
			checkpoint.runs.length === total ? new Date().toISOString() : checkpoint.meta.completedAt;
		checkpoint.meta.updatedAt = new Date().toISOString();
		await saveCheckpoint();
	}

	console.log(
		JSON.stringify({
			type: stopping ? 'benchmark.stopped' : 'benchmark.completed',
			completed: checkpoint.runs.length,
			total,
			output: OUTPUT_PATH
		})
	);
}

async function benchmarkOne(item, sampleID) {
	let task = null;
	const startedAt = new Date().toISOString();
	const timing = {
		acceptedMs: null,
		firstEventMs: null,
		runStartedMs: null,
		firstToolMs: null,
		firstDeltaMs: null,
		terminalMs: null,
		serverDurationMs: null,
		totalMs: null
	};
	const output = { subtaskCount: null, completedSubtaskCount: null, subtaskTitles: [] };
	const observed = {
		eventCount: 0,
		toolCalls: 0,
		toolErrors: 0,
		eventTypes: {},
		tools: [],
		effectiveModel: null,
		effectiveEffort: null
	};
	let runId = null;
	let status = 'failed';
	let error = null;
	let terminalPayload = null;
	let measurementStarted = null;

	try {
		await setSettings(item.model, item.effort);
		task = await prepareTask(sampleID);
		const started = performance.now();
		measurementStarted = started;
		const run = await requestJSON('/api/agent/runs', {
			method: 'POST',
			body: {
				context: { type: 'task', taskId: task.id, action: 'decompose' }
			}
		});
		runId = run.id;
		timing.acceptedMs = round(performance.now() - started);
		const terminal = await waitForRun(run.id, started, timing, observed);
		timing.terminalMs = round(performance.now() - started);
		timing.serverDurationMs = elapsedISO(run.createdAt, terminal.occurredAt);
		terminalPayload = terminal.payload;
		status = terminal.type === 'agent.run.completed' ? 'completed' : terminal.type.replace('agent.run.', '');
		if (status !== 'completed') error = eventError(terminal) || `Run ended as ${status}`;
		if (
			observed.effectiveModel !== item.model ||
			observed.effectiveEffort !== item.effort
		) {
			status = 'configuration_mismatch';
			error = `Requested ${item.model}/${item.effort}, runtime reported ${observed.effectiveModel}/${observed.effectiveEffort}`;
		}
		const subtasks = await requestJSON(`/api/tasks/${encodeURIComponent(task.id)}/subtasks`);
		output.subtaskCount = subtasks.tasks.length;
		output.completedSubtaskCount = subtasks.tasks.filter(
			(subtask) => subtask.status === 'completed'
		).length;
		output.subtaskTitles = subtasks.tasks.map((subtask) => subtask.title);
		timing.totalMs = round(performance.now() - started);
		if (status === 'completed' && output.subtaskCount === 0) {
			status = 'empty';
			error = 'Run completed without creating subtasks';
		}
	} catch (caught) {
		error = errorText(caught);
		if (caught?.name === 'TimeoutError') status = 'timeout';
	} finally {
		if (measurementStarted !== null && timing.totalMs === null) {
			timing.totalMs = round(performance.now() - measurementStarted);
		}
		if (task) {
			try {
				const current = await requestJSON(`/api/tasks/${encodeURIComponent(task.id)}`);
				await requestJSON(`/api/tasks/${encodeURIComponent(task.id)}`, {
					method: 'DELETE',
					body: { version: current.version },
					expectEmpty: true
				});
			} catch (cleanupError) {
				error = [error, `Cleanup failed: ${errorText(cleanupError)}`].filter(Boolean).join('; ');
			}
		}
	}

	return {
		id: /^\d+$/.test(sampleID) ? `run-${sampleID.padStart(3, '0')}` : sampleID,
		iteration: item.iteration,
		roundPosition: item.roundPosition,
		model: item.model,
		effort: item.effort,
		status,
		startedAt,
		finishedAt: new Date().toISOString(),
		agentRunId: runId,
		timing,
		output,
		observed,
		terminalPayload,
		error
	};
}

async function setSettings(model, effort) {
	const view = await requestJSON('/api/settings');
	const timezone = view.settings.timezone ?? 'Europe/Moscow';
	await requestJSON('/api/settings', {
		method: 'PATCH',
		body: {
			timezone,
			agentModel: model,
			agentThinkingEffort: effort,
			version: view.settings.version
		}
	});
}

async function restoreSettings() {
	if (!checkpoint?.meta?.originalSettings) return;
	try {
		const view = await requestJSON('/api/settings');
		const original = checkpoint.meta.originalSettings;
		await requestJSON('/api/settings', {
			method: 'PATCH',
			body: {
				timezone: original.timezone ?? 'Europe/Moscow',
				agentModel: original.agentModel,
				agentThinkingEffort: original.agentThinkingEffort,
				version: view.settings.version
			}
		});
	} catch (caught) {
		console.error(JSON.stringify({ type: 'benchmark.restore_failed', error: errorText(caught) }));
	}
}

async function prepareTask(sampleID) {
	const created = await requestJSON('/api/tasks', {
		method: 'POST',
		body: { title: `${SCENARIO.title} [benchmark ${sampleID}]` }
	});
	return requestJSON(`/api/tasks/${encodeURIComponent(created.id)}`, {
		method: 'PATCH',
		body: { version: created.version, title: SCENARIO.title, description: SCENARIO.description }
	});
}

async function waitForRun(runId, started, timing, observed) {
	const controller = new AbortController();
	const timeout = setTimeout(() => controller.abort(new DOMException('Run timed out', 'TimeoutError')), RUN_TIMEOUT_MS);
	let terminal = null;
	try {
		const response = await fetch(`${BASE_URL}/api/agent/runs/${encodeURIComponent(runId)}/events`, {
			headers: { Accept: 'text/event-stream', Cookie: cookie },
			signal: controller.signal
		});
		if (!response.ok || !response.body) throw new Error(`SSE HTTP ${response.status}`);
		const reader = response.body.getReader();
		const decoder = new TextDecoder();
		let buffer = '';
		while (!terminal) {
			const { done, value } = await reader.read();
			buffer += decoder.decode(value, { stream: !done });
			const parsed = extractRecords(buffer);
			buffer = parsed.remainder;
			for (const event of parsed.events) {
				const elapsed = round(performance.now() - started);
				observeEvent(event, elapsed, timing, observed);
				if (['agent.run.completed', 'agent.run.failed', 'agent.run.aborted'].includes(event.type)) {
					terminal = event;
					break;
				}
			}
			if (done && !terminal) throw new Error('Agent event stream ended before a terminal event');
		}
		return terminal;
	} catch (caught) {
		if (controller.signal.aborted) {
			const timeoutError = new Error('Run timed out');
			timeoutError.name = 'TimeoutError';
			throw timeoutError;
		}
		throw caught;
	} finally {
		clearTimeout(timeout);
		controller.abort();
	}
}

function observeEvent(event, elapsed, timing, observed) {
	observed.eventCount += 1;
	observed.eventTypes[event.type] = (observed.eventTypes[event.type] ?? 0) + 1;
	timing.firstEventMs ??= elapsed;
	if (event.type === 'agent.run.started') {
		timing.runStartedMs ??= elapsed;
		observed.effectiveModel ??= event.payload?.model ?? null;
		observed.effectiveEffort ??= event.payload?.thinkingEffort ?? null;
	}
	if (event.type === 'agent.tool.started') {
		timing.firstToolMs ??= elapsed;
		observed.toolCalls += 1;
		observed.tools.push({
			id: event.payload?.toolCallId ?? null,
			name: event.payload?.name ?? event.payload?.toolName ?? 'unknown',
			startedMs: elapsed,
			completedMs: null,
			isError: null
		});
	}
	if (event.type === 'agent.tool.completed') {
		if (event.payload?.isError) observed.toolErrors += 1;
		const callID = event.payload?.toolCallId ?? null;
		const tool = [...observed.tools]
			.reverse()
			.find((item) => item.completedMs === null && (!callID || item.id === callID));
		if (tool) {
			tool.completedMs = elapsed;
			tool.isError = Boolean(event.payload?.isError);
		}
	}
	if (event.type === 'agent.message.delta') timing.firstDeltaMs ??= elapsed;
}

function extractRecords(input) {
	const normalized = input.replaceAll('\r\n', '\n');
	const parts = normalized.split('\n\n');
	const remainder = parts.pop() ?? '';
	const events = [];
	for (const part of parts) {
		const data = part
			.split('\n')
			.filter((line) => line.startsWith('data:'))
			.map((line) => line.slice(5).trimStart());
		if (data.length) events.push(JSON.parse(data.join('\n')));
	}
	return { events, remainder };
}

async function login() {
	const response = await fetch(`${BASE_URL}/api/auth/login`, {
		method: 'POST',
		headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
		body: JSON.stringify({ login: USERNAME, password: PASSWORD })
	});
	if (!response.ok) throw new Error(`Login failed with HTTP ${response.status}`);
	const setCookie = response.headers.get('set-cookie');
	if (!setCookie) throw new Error('Login response did not include a session cookie');
	cookie = setCookie.split(';', 1)[0];
}

async function requestJSON(relativePath, options = {}) {
	const headers = { Accept: 'application/json', Cookie: cookie };
	let body;
	if (options.body !== undefined) {
		headers['Content-Type'] = 'application/json';
		body = JSON.stringify(options.body);
	}
	const response = await fetch(`${BASE_URL}${relativePath}`, {
		method: options.method ?? 'GET',
		headers,
		body
	});
	if (!response.ok) {
		const text = await response.text();
		throw new Error(`${options.method ?? 'GET'} ${relativePath}: HTTP ${response.status}: ${text.trim()}`);
	}
	if (options.expectEmpty || response.status === 204) return null;
	return response.json();
}

function buildSchedule() {
	const random = seededRandom(SEED);
	const combinations = MODELS.flatMap((model) => EFFORTS.map((effort) => ({ model, effort })));
	const schedule = [];
	for (let iteration = 1; iteration <= RUNS_PER_CONFIG; iteration += 1) {
		const round = combinations.map((item) => ({ ...item }));
		shuffle(round, random);
		round.forEach((item, roundPosition) => schedule.push({ ...item, iteration, roundPosition: roundPosition + 1 }));
	}
	return schedule;
}

function seededRandom(seed) {
	let state = seed >>> 0;
	return () => {
		state ^= state << 13;
		state ^= state >>> 17;
		state ^= state << 5;
		return (state >>> 0) / 0x1_0000_0000;
	};
}

function shuffle(values, random) {
	for (let index = values.length - 1; index > 0; index -= 1) {
		const target = Math.floor(random() * (index + 1));
		[values[index], values[target]] = [values[target], values[index]];
	}
}

async function loadCheckpoint() {
	try {
		return JSON.parse(await fs.readFile(OUTPUT_PATH, 'utf8'));
	} catch (caught) {
		if (caught?.code !== 'ENOENT') throw caught;
		const git = await gitMetadata();
		return {
			meta: {
				title: 'GPT-5.6 task decomposition latency benchmark',
				generatedAt: new Date().toISOString(),
				updatedAt: new Date().toISOString(),
				completedAt: null,
				provider: 'openai-codex',
				baseUrl: BASE_URL,
				models: MODELS,
				efforts: EFFORTS,
				runsPerConfig: RUNS_PER_CONFIG,
				concurrency: 1,
				order: `${RUNS_PER_CONFIG} seeded shuffled rounds, one sample per configuration per round`,
				seed: SEED,
				timeoutMs: RUN_TIMEOUT_MS,
				cooldownMs: COOLDOWN_MS,
				warmupRuns: WARMUP_RUNS,
				scenario: SCENARIO,
				scenarioSha256: crypto.createHash('sha256').update(JSON.stringify(SCENARIO)).digest('hex'),
				machine: {
					platform: `${os.platform()} ${os.release()} ${os.arch()}`,
					cpu: os.cpus()[0]?.model ?? 'unknown',
					logicalCpus: os.cpus().length,
					memoryBytes: os.totalmem(),
					node: process.version
				},
				git
			},
			runs: []
		};
	}
}

async function gitMetadata() {
	const { execFile } = await import('node:child_process');
	const exec = (args) =>
		new Promise((resolve) =>
			execFile('git', args, { cwd: process.cwd() }, (error, stdout) =>
				resolve(error ? null : stdout.trim())
			)
		);
	return {
		commit: await exec(['rev-parse', 'HEAD']),
		dirty: Boolean(await exec(['status', '--porcelain']))
	};
}

async function saveCheckpoint() {
	const temporary = `${OUTPUT_PATH}.tmp`;
	await fs.writeFile(temporary, `${JSON.stringify(checkpoint, null, 2)}\n`);
	await fs.rename(temporary, OUTPUT_PATH);
}

function eventError(event) {
	const value = event?.payload?.error;
	if (typeof value === 'string') return value;
	if (value && typeof value.message === 'string') return value.message;
	return '';
}

function errorText(error) {
	return error instanceof Error ? `${error.name}: ${error.message}` : String(error);
}

function round(value) {
	return Math.round(value * 10) / 10;
}

function elapsedISO(start, end) {
	const startTime = Date.parse(start ?? '');
	const endTime = Date.parse(end ?? '');
	return Number.isFinite(startTime) && Number.isFinite(endTime) ? endTime - startTime : null;
}

function csvEnv(name, fallback) {
	const value = process.env[name];
	return value ? value.split(',').map((item) => item.trim()).filter(Boolean) : fallback;
}

function sleep(milliseconds) {
	return new Promise((resolve) => setTimeout(resolve, milliseconds));
}
