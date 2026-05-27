#!/usr/bin/env node
const fs = require('node:fs')
const path = require('node:path')

const sourcePath = path.join(process.cwd(), 'src', 'electron', 'main.ts')
const source = fs.readFileSync(sourcePath, 'utf8')

function fail(message) {
  console.error(`Internal extension check failed: ${message}`)
  process.exit(1)
}

if (/const\s+INTERNAL_EXTENSIONS\s*=/.test(source)) {
  fail('internal extensions must be registered from lazy factories, not eager instances')
}

const factoriesMatch = source.match(/const\s+INTERNAL_EXTENSION_FACTORIES[\s\S]*?\]\n/)
if (!factoriesMatch) fail('missing INTERNAL_EXTENSION_FACTORIES')
const factories = factoriesMatch[0]

if (!factories.includes('createAiBuilderExtension')) fail('AI Builder factory is not registered')
if (factories.includes('createAiBuilderExtension()')) fail('AI Builder must not be constructed in the factory list')

if (!/const\s+AI_BUILDER_EXTENSION_ID\s*=\s*['"]nevermind\.ai-builder['"]/.test(source)) {
  fail('missing canonical AI Builder extension id constant')
}

if (!/REQUIRED_INTERNAL_EXTENSIONS[\s\S]*AI_BUILDER_EXTENSION_ID/.test(source)) {
  fail('AI Builder is not required by internal extension assertions')
}

if (!/REQUIRED_INTERNAL_COMMANDS[\s\S]*AI_BUILDER_EXTENSION_ID[\s\S]*commandId:\s*['"]ai-chats['"]/.test(source)) {
  fail('AI Chats command is not required by internal command assertions')
}

if (!/function\s+assertInternalExtensionsRegistered\s*\(/.test(source)) {
  fail('missing internal extension registration assertion')
}

console.log('Internal extension checks passed')
