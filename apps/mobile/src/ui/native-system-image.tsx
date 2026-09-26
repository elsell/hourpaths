import { PlatformSymbol } from './platform-symbol';
export function NativeSystemImage({ systemName }: { systemName: string }) {
  return <PlatformSymbol systemName={systemName} size={32} />;
}
