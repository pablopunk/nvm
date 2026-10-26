import { resolve } from 'node:path';
import { Config } from '@remotion/cli/config';

const rootReact = resolve(process.cwd(), '../../node_modules/react');
const rootReactDom = resolve(process.cwd(), '../../node_modules/react-dom');

Config.overrideWebpackConfig((config) => ({
  ...config,
  resolve: {
    ...config.resolve,
    alias: {
      ...config.resolve?.alias,
      react: rootReact,
      'react-dom': rootReactDom,
    },
  },
}));
