import { redirectProviderSystemPath } from '../src/provider-auth-intent';

export function redirectSystemPath({ path }: { initial: boolean; path: string }): string | null {
  return redirectProviderSystemPath(path);
}
