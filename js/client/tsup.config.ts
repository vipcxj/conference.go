import type { Options } from 'tsup';
import { umdWrapper } from 'esbuild-plugin-umd-wrapper';
import { dependencies } from './package.json'

const env = process.env.NODE_ENV;

const externalDependencies = Object.keys(dependencies);

export const tsup: Options = {
    splitting: false,
    sourcemap: true,
    clean: true, // clean up the dist folder
    dts: true, // generate dts files
    platform: 'browser',
    esbuildPlugins: [umdWrapper({
        libraryName: 'cg',
        external: 'inherit',
    })],
    format: ['umd'], // generate cjs and esm files
    noExternal: externalDependencies,
    minify: true,
    bundle: true,
    skipNodeModulesBundle: false,
    entry: ['src/index.ts'],
    watch: false,
    target: 'es5',
    outDir: 'dist',
};