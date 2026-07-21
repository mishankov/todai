import fs from 'node:fs/promises';
import path from 'node:path';
import process from 'node:process';

const inputPath = path.resolve(process.argv[2] ?? 'benchmarks/decomposition-gpt-5.6/results.json');
const outputPath = path.resolve(process.argv[3] ?? 'benchmarks/decomposition-gpt-5.6/report.html');
const templatePath = new URL('./report-template.html', import.meta.url);

const [data, template] = await Promise.all([
	fs.readFile(inputPath, 'utf8').then(JSON.parse),
	fs.readFile(templatePath, 'utf8')
]);
const embedded = JSON.stringify(data).replaceAll('<', '\\u003c');
await fs.writeFile(outputPath, template.replace('__BENCHMARK_DATA__', embedded));
console.log(outputPath);
