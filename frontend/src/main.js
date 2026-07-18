import { mount } from 'svelte';
import App from './App.svelte';

// Svelte 5 마운팅 API (mount) 적용
const app = mount(App, {
  target: document.getElementById('app'),
});

export default app;
