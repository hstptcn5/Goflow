import { createApp } from 'vue';
import { createPinia, setActivePinia } from 'pinia';
import { router } from '../src/router';

export async function mountWithApp(component, options = {}) {
  const root = document.createElement('div');
  document.body.appendChild(root);
  const pinia = createPinia();
  setActivePinia(pinia);
  const app = createApp(component, options.props || {});
  app.use(pinia);
  app.use(router);
  await router.push(options.route || '/workflows');
  await router.isReady();
  app.mount(root);
  await nextFrame();
  return {
    root,
    app,
    pinia,
    unmount: () => app.unmount(),
  };
}

export function nextFrame() {
  return new Promise((resolve) => requestAnimationFrame(resolve));
}
