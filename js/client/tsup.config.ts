import type { Options } from 'tsup';

const env = process.env.NODE_ENV;

export const tsup: Options = {
    splitting: false,
    sourcemap: true,
    clean: true, // clean up the dist folder
    dts: true, // generate dts files
    platform: 'browser',
    format: ['cjs', 'esm'], // generate cjs and esm files
    minify: true,
    bundle: true,
    skipNodeModulesBundle: false,
    entry: ['src/index.ts'],
    watch: false,
    target: 'es5',
    outDir: 'dist',
};