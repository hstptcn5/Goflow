import { createApp } from 'vue';
import { createPinia } from 'pinia';
import App from './App.vue';
import { router } from './router';
import { installWorkflowArrowMarkers } from './utils/workflowArrowMarkers';
import { installVietnameseOperatorUI } from './utils/operatorVi';
import './assets/main.css';
import './assets/stitch-theme.css';
import './assets/workflow-arrows.css';

installWorkflowArrowMarkers();

const app = createApp(App);

app.use(createPinia());
app.use(router);
app.mount('#app');
installVietnameseOperatorUI();
