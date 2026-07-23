<script setup>
import { onMounted, computed } from 'vue';
import { useWorkflowStore } from '@/stores/workflowStore';

const workflowStore = useWorkflowStore();

onMounted(() => {
  workflowStore.fetchNodeDefinitions();
});

function onDragStart(event, nodeDef) {
  event.dataTransfer.setData('application/goflow-node', JSON.stringify(nodeDef));
  event.dataTransfer.effectAllowed = 'move';
}

const groupedNodes = computed(() => {
  const groups = {};
  workflowStore.nodeDefinitions.forEach((def) => {
    const cat = def.category || 'OTHER';
    if (!groups[cat]) {
      groups[cat] = [];
    }
    groups[cat].push(def);
  });
  return groups;
});

function getGroupTitle(category) {
  const titles = {
    'TRIGGER': 'Triggers',
    'ACTION': 'Actions',
    'LOGIC': 'Logic & Flow',
    'LOGIC & UTILITY': 'Logic & Utilities',
    'DATABASE': 'Databases',
    'AI & LLM': 'AI & Language Models',
    'COMMUNICATION': 'SaaS & Communication',
    'DEVELOPER': 'Developer Tools',
  };
  return titles[category] || category;
}

function getGroupIcon(category) {
  const icons = {
    'TRIGGER': '⚡',
    'ACTION': '⚙️',
    'LOGIC': '🔀',
    'LOGIC & UTILITY': '🧩',
    'DATABASE': '🗄️',
    'AI & LLM': '🧠',
    'COMMUNICATION': '💬',
    'DEVELOPER': '💻',
  };
  return icons[category] || '⚙️';
}

function getPaletteItemClass(category) {
  const classes = {
    'TRIGGER': 'item-trigger',
    'DATABASE': 'item-db',
    'COMMUNICATION': 'item-saas',
    'AI & LLM': 'item-ai',
    'DEVELOPER': 'item-dev',
  };
  return classes[category] || 'item-logic';
}

function getNodeIcon(type) {
  const icons = {
    webhookTrigger: '🔗',
    cronTrigger: '⏰',
    manualTrigger: '⚡',
    httpRequest: '🌐',
    telegramBot: '📢',
    jsonTransform: '🔄',
    conditionIf: '🌿',
    emailSMTP: '📧',
    delaySleep: '⏳',
    openAIGPT: '🤖',
    deepseekAI: '🧠',
    discordBot: '💬',
    slackBot: '🗣️',
    jsCodeRunner: '⚙️',
    subWorkflow: '📁',
    postgresQuery: '🐘',
    redisCommand: '🔑',
    googleSheets: '📊',
    mysqlQuery: '🐬',
    mongodbCommand: '🍃',
    googleDrive: '💾',
    gmailREST: '✉️',
    notionPage: '📓',
    sshRunner: '💻',
    gitCommand: '🚀',
    githubWebhook: '🐙',
    goflowPlugin: '🔌',
  };
  return icons[type] || '⚙️';
}
</script>

<template>
  <aside class="node-palette glass-panel">
    <div class="palette-header">
      <span class="icon">🧩</span>
      <span class="title">Node Library</span>
    </div>

    <div class="palette-scroll">
      <div v-for="(defs, category) in groupedNodes" :key="category" class="category-group">
        <div class="group-title">
          <span class="group-icon">{{ getGroupIcon(category) }}</span>
          {{ getGroupTitle(category) }}
        </div>
        <div class="nodes-grid">
          <div
            v-for="def in defs"
            :key="def.type"
            class="palette-item"
            :class="getPaletteItemClass(category)"
            draggable="true"
            @dragstart="onDragStart($event, def)"
          >
            <div class="item-icon">{{ getNodeIcon(def.type) }}</div>
            <div class="item-info">
              <span class="item-name">{{ def.name }}</span>
              <span class="item-desc">{{ def.description }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </aside>
</template>

<style scoped>
.node-palette {
  width: 250px;
  height: calc(100vh - 60px);
  display: flex;
  flex-direction: column;
  border-radius: 0;
  background: #ffffff;
  border-right: 1px solid var(--border-color);
  user-select: none;
}

.palette-header {
  padding: 14px 16px;
  display: flex;
  align-items: center;
  gap: 8px;
  font-weight: 700;
  font-size: 0.95rem;
  color: #0f172a;
  border-bottom: 1px solid var(--border-color);
  background: #f8fafc;
}

.palette-scroll {
  padding: 14px;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.group-title {
  font-size: 0.725rem;
  font-weight: 800;
  color: #64748b;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}

.nodes-grid {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.palette-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  background: #ffffff;
  border: 1px solid #cbd5e1;
  border-radius: 8px;
  cursor: grab;
  transition: all 0.15s ease;
  box-shadow: 0 1px 3px rgba(0,0,0,0.05);
}

.palette-item:hover {
  transform: translateY(-2px);
  border-color: #2563eb;
  box-shadow: 0 4px 12px rgba(37, 99, 235, 0.15);
}

.palette-item.item-trigger:hover { border-color: #f97316; box-shadow: 0 4px 12px rgba(249, 115, 22, 0.15); }
.palette-item.item-logic:hover { border-color: #3b82f6; box-shadow: 0 4px 12px rgba(59, 130, 246, 0.15); }
.palette-item.item-saas:hover { border-color: #10b981; box-shadow: 0 4px 12px rgba(16, 185, 129, 0.15); }
.palette-item.item-db:hover { border-color: #6366f1; box-shadow: 0 4px 12px rgba(99, 102, 241, 0.15); }
.palette-item.item-ai:hover { border-color: #a855f7; box-shadow: 0 4px 12px rgba(168, 85, 247, 0.15); }
.palette-item.item-dev:hover { border-color: #64748b; box-shadow: 0 4px 12px rgba(100, 116, 139, 0.15); }

.palette-item:active {
  cursor: grabbing;
}

.item-icon {
  font-size: 1.1rem;
}

.item-info {
  display: flex;
  flex-direction: column;
}

.item-name {
  font-size: 0.85rem;
  font-weight: 700;
  color: #0f172a;
}

.item-desc {
  font-size: 0.725rem;
  color: #64748b;
  line-height: 1.25;
}
</style>
