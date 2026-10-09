import adapter from '@sveltejs/adapter-node';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';
export default {
  preprocess: vitePreprocess(),
  kit: {
    adapter: adapter(),
    // The same public shell restores every nested Studio URL offline.
    paths: { relative: false },
    csp: {
      mode: 'nonce',
      directives: {
        'default-src': ['self'],
        'base-uri': ['self'],
        'connect-src': ['self'],
        // Reviewed avatars use anonymous HTTPS; crop previews are normalized JPEG data.
        'img-src': ['self', 'https:', 'data:'],
        'form-action': ['self'],
        'frame-ancestors': ['none'],
        'object-src': ['none']
      }
    }
  }
};
