import { type ResolveHookContext, registerHooks } from 'node:module';

type NextResolve = Parameters<
  Parameters<typeof registerHooks>[0]['resolve'] & {}
>[2];

const SOURCE_SUFFIXES = ['.ts', '.tsx', '/index.ts'];
const RELATIVE_SPECIFIER = /^\.{1,2}\//;
const HAS_FILE_EXTENSION = /\.[cm]?[jt]sx?$|\.json$/;
let installed = false;

function isExtensionlessRelativeSpecifier(specifier: string) {
  return (
    RELATIVE_SPECIFIER.test(specifier) && !HAS_FILE_EXTENSION.test(specifier)
  );
}

function isModuleNotFound(error: unknown) {
  return (error as { code?: string })?.code === 'ERR_MODULE_NOT_FOUND';
}

function resolveWithSourceSuffix(
  specifier: string,
  context: ResolveHookContext,
  nextResolve: NextResolve,
) {
  for (const suffix of SOURCE_SUFFIXES) {
    try {
      return nextResolve(specifier + suffix, context);
    } catch (error) {
      if (!isModuleNotFound(error)) {
        throw error;
      }
    }
  }
  return;
}

/**
 * Source files in `src/app` import each other without file extensions because the app is bundled,
 * but dev fixtures are loaded straight from source by Node, whose resolver needs the extension.
 */
export function resolveExtensionlessSourceImports() {
  if (installed) {
    return;
  }
  installed = true;
  registerHooks({
    resolve(specifier, context, nextResolve) {
      try {
        return nextResolve(specifier, context);
      } catch (error) {
        if (
          !(
            isModuleNotFound(error) &&
            isExtensionlessRelativeSpecifier(specifier)
          )
        ) {
          throw error;
        }
        const resolved = resolveWithSourceSuffix(
          specifier,
          context,
          nextResolve,
        );
        if (resolved) {
          return resolved;
        }
        throw error;
      }
    },
  });
}
