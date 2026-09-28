import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

const app = JSON.parse(await readFile(new URL('../apps/mobile/package.json', import.meta.url)));
const bundled = JSON.parse(await readFile(new URL('../apps/mobile/node_modules/expo/bundledNativeModules.json', import.meta.url)));
assert.deepEqual(app.expo?.install?.exclude, ['react-native', '@react-navigation/native', '@react-navigation/native-stack', 'expo-updates'], 'only documented exact-version metadata exceptions are allowed');
for (const [name, version] of [['@react-navigation/native', '7.3.8'], ['@react-navigation/native-stack', '7.17.10'], ['expo-updates', '55.0.31']]) {
  const installed = JSON.parse(await readFile(new URL(`../apps/mobile/node_modules/${name}/package.json`, import.meta.url)));
  assert.equal(app.dependencies[name], version, `${name} must retain the reviewed native dependency version`);
  assert.equal(installed.version, version, `${name} must resolve to the reviewed version`);
}
assert.equal(app.dependencies['react-native'], bundled['react-native'], 'React Native must match Expo bundledNativeModules.json exactly');
console.log(`React Native ${app.dependencies['react-native']} matches the installed Expo native manifest`);

assert.equal(bundled['expo-updates'], '~55.0.30', 'revisit the Updates compatibility exception when upgrading Expo');
